package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const composeURL = "https://x.com/compose/post"

func main() {
	var cdpAddr string
	var targetURL string
	var text string
	var publish bool
	var timeout time.Duration
	var screenshot string

	flag.StringVar(&cdpAddr, "cdp", envDefault("CDP_ADDR", "http://127.0.0.1:9222"), "Chromium CDP HTTP address")
	flag.StringVar(&targetURL, "url", envDefault("XPOST_URL", composeURL), "page URL to open before filling the composer")
	flag.StringVar(&text, "text", "", "post text; if empty, remaining args are joined")
	flag.BoolVar(&publish, "publish", false, "click the Post button after filling the composer")
	flag.DurationVar(&timeout, "timeout", 45*time.Second, "maximum time to wait for page controls")
	flag.StringVar(&screenshot, "screenshot", "", "optional path for a PNG screenshot after the flow")
	flag.Parse()

	if text == "" {
		text = strings.TrimSpace(strings.Join(flag.Args(), " "))
	}
	if text == "" {
		fatalf("missing post text; use --text or pass text as arguments")
	}

	ctxDeadline := time.Now().Add(timeout)
	pageWS, err := createPage(cdpAddr, targetURL)
	if err != nil {
		fatalf("open compose page: %v", err)
	}

	client, err := dialCDP(pageWS)
	if err != nil {
		fatalf("connect to page websocket: %v", err)
	}
	defer client.Close()

	must(client.Call("Page.enable", nil, nil))
	must(client.Call("Runtime.enable", nil, nil))

	if err := waitReady(client, ctxDeadline); err != nil {
		fatalf("page did not become ready: %v", err)
	}

	if err := waitAndFillComposer(client, text, ctxDeadline); err != nil {
		fatalf("fill composer: %v", err)
	}

	if publish {
		if err := waitAndClickPost(client, ctxDeadline); err != nil {
			fatalf("click Post: %v", err)
		}
		fmt.Println("post submitted")
	} else {
		fmt.Println("composer filled; pass --publish to click Post")
	}

	if screenshot != "" {
		if err := saveScreenshot(client, screenshot); err != nil {
			fatalf("save screenshot: %v", err)
		}
		fmt.Printf("screenshot saved to %s\n", screenshot)
	}
}

func createPage(cdpAddr, targetURL string) (string, error) {
	base := strings.TrimRight(cdpAddr, "/")
	u := base + "/json/new?" + url.QueryEscape(targetURL)

	req, err := http.NewRequest(http.MethodPut, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var target struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return "", err
	}
	if target.WebSocketDebuggerURL == "" {
		return "", errors.New("CDP target response did not include webSocketDebuggerUrl")
	}
	return target.WebSocketDebuggerURL, nil
}

func waitReady(c *cdpClient, deadline time.Time) error {
	return poll(deadline, 500*time.Millisecond, func() (bool, error) {
		value, err := evalString(c, `document.readyState`)
		if err != nil {
			return false, nil
		}
		return value == "interactive" || value == "complete", nil
	})
}

func waitAndFillComposer(c *cdpClient, text string, deadline time.Time) error {
	script := fmt.Sprintf(`(() => {
const text = %s;
const selectors = [
  'div[data-testid="tweetTextarea_0"]',
  'div[data-testid^="tweetTextarea_"]',
  'div[role="textbox"][contenteditable="true"]'
];
const el = selectors.map((s) => document.querySelector(s)).find(Boolean);
if (!el) return {ok:false, reason:'composer not found'};
el.scrollIntoView({block:'center'});
el.focus();
const paste = () => {
  try {
    const dt = new DataTransfer();
    dt.setData('text/plain', text);
    el.dispatchEvent(new ClipboardEvent('paste', {clipboardData: dt, bubbles: true, cancelable: true}));
    return true;
  } catch (_) {
    return false;
  }
};
paste();
if (!el.innerText || !el.innerText.includes(text)) {
  document.execCommand('selectAll', false, null);
  document.execCommand('insertText', false, text);
}
el.dispatchEvent(new InputEvent('input', {bubbles:true, inputType:'insertText', data:text}));
return {ok: (el.innerText || el.textContent || '').includes(text), reason: 'filled'};
})()`, jsString(text))

	return poll(deadline, 750*time.Millisecond, func() (bool, error) {
		var out struct {
			Result runtimeRemoteObject `json:"result"`
		}
		if err := c.Call("Runtime.evaluate", map[string]any{
			"expression":                  script,
			"awaitPromise":                true,
			"returnByValue":               true,
			"userGesture":                 true,
			"allowUnsafeEvalBlockedByCSP": true,
		}, &out); err != nil {
			return false, nil
		}
		res, _ := out.Result.Value.(map[string]any)
		return res["ok"] == true, nil
	})
}

func waitAndClickPost(c *cdpClient, deadline time.Time) error {
	script := `(() => {
const selectors = [
  '[data-testid="tweetButton"]',
  '[data-testid="tweetButtonInline"]',
  'button[data-testid="tweetButton"]',
  'button[data-testid="tweetButtonInline"]'
];
const btn = selectors.map((s) => document.querySelector(s)).find((el) => el && !el.disabled && el.getAttribute('aria-disabled') !== 'true');
if (!btn) return {ok:false, reason:'post button not ready'};
btn.scrollIntoView({block:'center'});
btn.click();
return {ok:true};
})()`

	return poll(deadline, 750*time.Millisecond, func() (bool, error) {
		var out struct {
			Result runtimeRemoteObject `json:"result"`
		}
		if err := c.Call("Runtime.evaluate", map[string]any{
			"expression":    script,
			"awaitPromise":  true,
			"returnByValue": true,
			"userGesture":   true,
		}, &out); err != nil {
			return false, nil
		}
		res, _ := out.Result.Value.(map[string]any)
		return res["ok"] == true, nil
	})
}

func saveScreenshot(c *cdpClient, path string) error {
	var out struct {
		Data string `json:"data"`
	}
	if err := c.Call("Page.captureScreenshot", map[string]any{"format": "png", "fromSurface": true}, &out); err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func evalString(c *cdpClient, expression string) (string, error) {
	var out struct {
		Result runtimeRemoteObject `json:"result"`
	}
	if err := c.Call("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	}, &out); err != nil {
		return "", err
	}
	if out.Result.Value == nil {
		return "", nil
	}
	return fmt.Sprint(out.Result.Value), nil
}

func poll(deadline time.Time, every time.Duration, fn func() (bool, error)) error {
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := fn()
		if err != nil {
			lastErr = err
		}
		if ok {
			return nil
		}
		time.Sleep(every + time.Duration(mrand.Intn(250))*time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("timed out")
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func envDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func must(err error) {
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "xpost: "+format+"\n", args...)
	os.Exit(1)
}

type runtimeRemoteObject struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type cdpClient struct {
	conn   net.Conn
	reader *bufio.Reader
	nextID int
}

func dialCDP(rawurl string) (*cdpClient, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("unsupported websocket scheme %q; only ws is supported", u.Scheme)
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, err
	}
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, u.Host, key)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(status, " 101 ") {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade failed: %s", strings.TrimSpace(status))
	}
	headers := make(http.Header)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			headers.Add(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
	want := websocketAccept(key)
	if got := headers.Get("Sec-WebSocket-Accept"); got != want {
		conn.Close()
		return nil, errors.New("websocket accept header mismatch")
	}
	return &cdpClient{conn: conn, reader: br}, nil
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (c *cdpClient) Close() error {
	return c.conn.Close()
}

func (c *cdpClient) Call(method string, params map[string]any, result any) error {
	c.nextID++
	id := c.nextID
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := c.writeText(payload); err != nil {
		return err
	}

	for {
		frame, err := c.readFrame()
		if err != nil {
			return err
		}
		var envelope struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(frame, &envelope); err != nil {
			continue
		}
		if envelope.ID != id {
			continue
		}
		if envelope.Error != nil {
			return fmt.Errorf("%s: %s", method, envelope.Error.Message)
		}
		if result != nil && len(envelope.Result) > 0 {
			return json.Unmarshal(envelope.Result, result)
		}
		return nil
	}
}

func (c *cdpClient) writeText(payload []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(0x81)
	n := len(payload)
	switch {
	case n < 126:
		buf.WriteByte(byte(0x80 | n))
	case n <= math.MaxUint16:
		buf.WriteByte(0x80 | 126)
		_ = binary.Write(&buf, binary.BigEndian, uint16(n))
	default:
		buf.WriteByte(0x80 | 127)
		_ = binary.Write(&buf, binary.BigEndian, uint64(n))
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	buf.Write(mask)
	for i, b := range payload {
		buf.WriteByte(b ^ mask[i%4])
	}
	_, err := c.conn.Write(buf.Bytes())
	return err
}

func (c *cdpClient) readFrame() ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return nil, err
	}
	opcode := header[0] & 0x0f
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		var x uint16
		if err := binary.Read(c.reader, binary.BigEndian, &x); err != nil {
			return nil, err
		}
		length = uint64(x)
	case 127:
		if err := binary.Read(c.reader, binary.BigEndian, &length); err != nil {
			return nil, err
		}
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.reader, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	switch opcode {
	case 0x1:
		return payload, nil
	case 0x8:
		return nil, io.EOF
	case 0x9:
		_ = c.writePong(payload)
		return c.readFrame()
	default:
		return c.readFrame()
	}
}

func (c *cdpClient) writePong(payload []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(0x8a)
	buf.WriteByte(byte(0x80 | len(payload)))
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	buf.Write(mask)
	for i, b := range payload {
		buf.WriteByte(b ^ mask[i%4])
	}
	_, err := c.conn.Write(buf.Bytes())
	return err
}
