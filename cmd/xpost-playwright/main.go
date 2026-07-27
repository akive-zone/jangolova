package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const composeURL = "https://x.com/compose/post"

func main() {
	var targetURL string
	var text string
	var publish bool
	var timeout time.Duration
	var screenshot string
	var profileDir string
	var executablePath string

	flag.StringVar(&targetURL, "url", envDefault("XPOST_URL", composeURL), "page URL to open before filling the composer")
	flag.StringVar(&text, "text", "", "post text; if empty, remaining args are joined")
	flag.BoolVar(&publish, "publish", false, "click the Post button after filling the composer")
	flag.DurationVar(&timeout, "timeout", envDuration("XPOST_TIMEOUT", 45*time.Second), "maximum time to wait for page controls")
	flag.StringVar(&screenshot, "screenshot", "", "optional path for a PNG screenshot after the flow")
	flag.StringVar(&profileDir, "profile", envDefault("PROFILE_DIR", defaultProfileDir()), "persistent Chromium profile directory")
	flag.StringVar(&executablePath, "executable", envDefault("PLAYWRIGHT_BROWSER_PATH", ""), "Chromium/Chrome executable path")
	flag.Parse()

	if text == "" {
		text = strings.TrimSpace(strings.Join(flag.Args(), " "))
	}
	if text == "" {
		fatalf("missing post text; use --text or pass text as arguments")
	}
	if executablePath == "" {
		executablePath = findChromium()
	}
	if executablePath == "" {
		fatalf("could not find chromium, chromium-browser, google-chrome, or google-chrome-stable")
	}

	if envFlag("CHROMIUM_CLEAR_STALE_LOCKS") {
		clearChromiumLocks(profileDir)
	}

	pw, err := playwright.Run()
	if err != nil {
		fatalf("start Playwright: %v (install its driver with: go run ./cmd/playwright-install)", err)
	}
	defer func() {
		if err := pw.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "xpost-playwright: stop Playwright: %v\n", err)
		}
	}()

	launchArgs := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-dev-shm-usage",
		"--password-store=basic",
		"--start-maximized",
	}
	if os.Geteuid() == 0 {
		launchArgs = append(launchArgs, "--no-sandbox")
	}

	context, err := pw.Chromium.LaunchPersistentContext(profileDir, playwright.BrowserTypeLaunchPersistentContextOptions{
		Args:           launchArgs,
		ExecutablePath: playwright.String(executablePath),
		Headless:       playwright.Bool(envFlag("PLAYWRIGHT_HEADLESS")),
		NoViewport:     playwright.Bool(true),
		Timeout:        playwright.Float(float64(timeout.Milliseconds())),
	})
	if err != nil {
		fatalf("launch Chromium: %v", err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "xpost-playwright: close browser context: %v\n", err)
		}
	}()

	var page playwright.Page
	pages := context.Pages()
	if len(pages) > 0 {
		page = pages[0]
	} else {
		page, err = context.NewPage()
		if err != nil {
			fatalf("open browser page: %v", err)
		}
	}
	page.SetDefaultTimeout(float64(timeout.Milliseconds()))

	if _, err := page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		fatalf("open compose page: %v", err)
	}

	composer := page.Locator(strings.Join([]string{
		`div[data-testid="tweetTextarea_0"]`,
		`div[data-testid^="tweetTextarea_"]`,
		`div[role="textbox"][contenteditable="true"]`,
	}, ", ")).First()
	if err := composer.WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	}); err != nil {
		fatalf("find composer: %v", err)
	}
	if err := composer.Fill(text); err != nil {
		fatalf("fill composer: %v", err)
	}

	if publish {
		postButton := page.Locator(strings.Join([]string{
			`[data-testid="tweetButton"]`,
			`[data-testid="tweetButtonInline"]`,
			`button[data-testid="tweetButton"]`,
			`button[data-testid="tweetButtonInline"]`,
		}, ", ")).First()
		if err := postButton.Click(); err != nil {
			fatalf("click Post: %v", err)
		}
		fmt.Println("post submitted")
	} else {
		fmt.Println("composer filled; pass --publish to click Post")
	}

	if screenshot != "" {
		if err := os.MkdirAll(filepath.Dir(screenshot), 0o755); err != nil {
			fatalf("create screenshot directory: %v", err)
		}
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			FullPage: playwright.Bool(true),
			Path:     playwright.String(screenshot),
		}); err != nil {
			fatalf("save screenshot: %v", err)
		}
		fmt.Printf("screenshot saved to %s\n", screenshot)
	}
}

func findChromium() string {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	for _, path := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func clearChromiumLocks(profileDir string) {
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		_ = os.Remove(filepath.Join(profileDir, name))
	}
}

func defaultProfileDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".chromium-xpost-profile"
	}
	return filepath.Join(home, ".local", "share", "chromium-xpost-profile")
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		fatalf("invalid %s duration %q: %v", name, value, err)
	}
	return duration
}

func envFlag(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "xpost-playwright: "+format+"\n", args...)
	os.Exit(1)
}
