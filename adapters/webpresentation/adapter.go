// Package webpresentation attaches Jangolova's semantic presentation bridge to
// a caller-owned Chromium-compatible CDP browser. It never launches or stops
// the browser; Xallet or the native host owns that runtime.
package webpresentation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jangolova/internal/bridge"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

const defaultWorkerPath = "scripts/presentation-worker.mjs"

type Adapter struct{}

type options struct {
	NodePath   string `json:"nodePath,omitempty"`
	WorkerPath string `json:"workerPath,omitempty"`
}

type rpcRequest struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}
type rpcResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type instance struct {
	command       *exec.Cmd
	stdin         io.WriteCloser
	responses     chan rpcResponse
	done          chan error
	events        chan orchestrator.EngineEvent
	stderr        *lockedBuffer
	capabilities  []string
	callMu        sync.Mutex
	nextID        atomic.Uint64
	disconnecting atomic.Bool
	closed        atomic.Bool
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(value)
}
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.b.String())
}

var _ orchestrator.EngineAdapter = Adapter{}
var _ orchestrator.EngineInspector = Adapter{}
var _ orchestrator.EngineInstance = (*instance)(nil)
var _ orchestrator.EngineHealthProvider = (*instance)(nil)
var _ orchestrator.EngineCapabilityProvider = (*instance)(nil)
var _ orchestrator.EngineEventSource = (*instance)(nil)
var _ bridge.Caller = (*instance)(nil)

func (Adapter) InspectEngine(context.Context) orchestrator.EngineInspection {
	capabilities := capabilityNames()
	if _, err := exec.LookPath("node"); err != nil {
		return orchestrator.EngineInspection{Capabilities: capabilities, Message: "Node.js is required: " + err.Error()}
	}
	if _, err := resolveWorker(""); err != nil {
		return orchestrator.EngineInspection{Capabilities: capabilities, Message: err.Error()}
	}
	return orchestrator.EngineInspection{Available: true, Capabilities: capabilities}
}

func (Adapter) Connect(ctx context.Context, spec manifest.EngineSpec, target orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	if target.Kind != "browser" {
		return nil, errors.New("web-presentation requires target.kind browser")
	}
	endpoint, ok := target.Endpoint("cdp")
	if !ok {
		return nil, errors.New("web-presentation requires a caller-owned CDP endpoint")
	}
	if err := validateEndpoint(endpoint.URL); err != nil {
		return nil, err
	}
	config, err := decodeOptions(spec.Options)
	if err != nil {
		return nil, err
	}
	nodePath := strings.TrimSpace(config.NodePath)
	if nodePath == "" {
		nodePath, err = exec.LookPath("node")
		if err != nil {
			return nil, fmt.Errorf("find Node.js for web-presentation: %w", err)
		}
	}
	workerPath, err := resolveWorker(config.WorkerPath)
	if err != nil {
		return nil, err
	}
	command := exec.Command(nodePath, workerPath)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open presentation worker input: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open presentation worker output: %w", err)
	}
	stderr := &lockedBuffer{}
	command.Stderr = io.MultiWriter(os.Stderr, stderr)
	running := &instance{command: command, stdin: stdin, responses: make(chan rpcResponse, 1), done: make(chan error, 1), events: make(chan orchestrator.EngineEvent, 1), stderr: stderr}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start web-presentation worker: %w", err)
	}
	go running.readResponses(stdout)
	go running.wait()
	params, _ := json.Marshal(map[string]string{"endpoint": endpoint.URL, "source": strings.TrimSpace(spec.Source)})
	result, err := running.request(ctx, "connect", params)
	if err != nil {
		running.terminate()
		return nil, fmt.Errorf("connect web-presentation to target: %w%s", err, running.stderrSuffix())
	}
	var connected struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &connected); err != nil {
		running.terminate()
		return nil, fmt.Errorf("decode web-presentation worker handshake: %w", err)
	}
	running.capabilities = stableStrings(append(capabilityNames(), connected.Capabilities...))
	running.events <- orchestrator.EngineEvent{Type: "presentation.connected", Status: "connected", OccurredAt: time.Now().UTC()}
	return running, nil
}

func (i *instance) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case bridge.MethodHello, bridge.MethodCapabilities, bridge.MethodDescribe, bridge.MethodAct, bridge.MethodEvents:
	default:
		return nil, fmt.Errorf("unsupported interaction method %q", method)
	}
	return i.request(ctx, method, params)
}

func (i *instance) request(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	i.callMu.Lock()
	defer i.callMu.Unlock()
	if i.closed.Load() {
		return nil, errors.New("presentation worker is disconnected")
	}
	id := i.nextID.Add(1)
	request, _ := json.Marshal(rpcRequest{ID: id, Method: method, Params: params})
	if _, err := i.stdin.Write(append(request, '\n')); err != nil {
		return nil, fmt.Errorf("write presentation worker request: %w", err)
	}
	select {
	case response, open := <-i.responses:
		if !open {
			return nil, errors.New("presentation worker exited")
		}
		if response.ID != id {
			return nil, fmt.Errorf("worker response id %d does not match request %d", response.ID, id)
		}
		if response.Error != "" {
			return nil, errors.New(response.Error)
		}
		if !json.Valid(response.Result) {
			return nil, errors.New("worker returned invalid JSON")
		}
		return response.Result, nil
	case <-ctx.Done():
		i.terminate()
		return nil, ctx.Err()
	}
}

func (i *instance) Disconnect(ctx context.Context) error {
	if !i.disconnecting.CompareAndSwap(false, true) {
		select {
		case <-i.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_, requestErr := i.request(ctx, "disconnect", json.RawMessage(`{}`))
	_ = i.stdin.Close()
	select {
	case waitErr := <-i.done:
		if requestErr != nil {
			return requestErr
		}
		return waitErr
	case <-ctx.Done():
		i.terminate()
		return ctx.Err()
	}
}

func (i *instance) EngineHealth(ctx context.Context) orchestrator.EngineHealth {
	health := orchestrator.EngineHealth{ObservedAt: time.Now().UTC()}
	result, err := i.request(ctx, "health", json.RawMessage(`{}`))
	if err != nil {
		health.Status = orchestrator.EngineHealthUnhealthy
		health.Message = err.Error()
		return health
	}
	var value struct {
		Connected bool `json:"connected"`
	}
	if err := json.Unmarshal(result, &value); err != nil || !value.Connected {
		health.Status = orchestrator.EngineHealthUnhealthy
		health.Message = "presentation target is disconnected"
		return health
	}
	health.Status = orchestrator.EngineHealthHealthy
	return health
}
func (i *instance) EngineCapabilities() []string                  { return append([]string(nil), i.capabilities...) }
func (i *instance) EngineEvents() <-chan orchestrator.EngineEvent { return i.events }

func (i *instance) readResponses(stdout io.Reader) {
	defer close(i.responses)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var response rpcResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			response.Error = "decode worker response: " + err.Error()
		}
		i.responses <- response
	}
}
func (i *instance) wait() {
	err := i.command.Wait()
	i.closed.Store(true)
	i.done <- err
	close(i.done)
	event := orchestrator.EngineEvent{Type: "presentation.disconnected", Status: "disconnected", OccurredAt: time.Now().UTC()}
	if err != nil && !i.disconnecting.Load() {
		event.Type = "presentation.failed"
		event.Status = "failed"
		event.Message = err.Error() + i.stderrSuffix()
	}
	select {
	case i.events <- event:
	default:
	}
	close(i.events)
}
func (i *instance) terminate() {
	if i.command.Process != nil {
		_ = i.command.Process.Kill()
	}
}
func (i *instance) stderrSuffix() string {
	if value := i.stderr.String(); value != "" {
		return ": " + value
	}
	return ""
}

func decodeOptions(raw json.RawMessage) (options, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return options{}, nil
	}
	var value options
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return options{}, fmt.Errorf("decode web-presentation options: %w", err)
	}
	return value, nil
}
func validateEndpoint(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse CDP target endpoint: %w", err)
	}
	switch parsed.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("CDP target endpoint has unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("CDP target endpoint must include a host")
	}
	return nil
}
func resolveWorker(configured string) (string, error) {
	candidates := []string{strings.TrimSpace(configured), strings.TrimSpace(os.Getenv("JANGOLOVA_PRESENTATION_WORKER")), defaultWorkerPath, "/usr/local/lib/jangolova/presentation-worker.mjs"}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "..", "lib", "jangolova", "presentation-worker.mjs"))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err == nil {
			candidate = absolute
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("web presentation worker not found; set JANGOLOVA_PRESENTATION_WORKER")
}
func capabilityNames() []string {
	return []string{"act", "capabilities", "describe", "events", "presentation.activate", "presentation.capture", "presentation.create", "presentation.describe", "presentation.execute", "presentation.patch", "presentation.replace", "presentation.write", "target.cdp"}
}
func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
