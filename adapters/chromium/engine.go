// Package chromium implements a browser engine adapter that launches or
// attaches to a Chromium-compatible CDP endpoint.
package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"jangolova/internal/engineprovider"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

const defaultStartupTimeout = 30 * time.Second

type Adapter struct{}

type options struct {
	Address              string   `json:"address,omitempty"`
	Attach               bool     `json:"attach,omitempty"`
	Executable           string   `json:"executable,omitempty"`
	ProfileDir           string   `json:"profileDir,omitempty"`
	Headless             *bool    `json:"headless,omitempty"`
	NoSandbox            bool     `json:"noSandbox,omitempty"`
	AllowRemoteDebugging bool     `json:"allowRemoteDebugging,omitempty"`
	StartupTimeout       string   `json:"startupTimeout,omitempty"`
	Args                 []string `json:"args,omitempty"`
}

type instance struct {
	mu               sync.Mutex
	address          string
	command          *exec.Cmd
	done             chan error
	events           chan orchestrator.EngineEvent
	temporaryProfile string
}

var _ orchestrator.EngineAdapter = Adapter{}
var _ orchestrator.EngineInstance = (*instance)(nil)
var _ orchestrator.EngineEventSource = (*instance)(nil)
var _ orchestrator.EngineHealthProvider = (*instance)(nil)
var _ orchestrator.EngineInspector = Adapter{}
var _ engineprovider.EndpointProvider = (*instance)(nil)

func (Adapter) InspectEngine(context.Context) orchestrator.EngineInspection {
	capabilities := []string{
		"attach",
		"endpoint.cdp",
		"events",
		"health",
		"runtime.environment",
		"stop",
	}
	inspection := orchestrator.EngineInspection{
		Available:    true,
		Capabilities: capabilities,
	}
	if _, err := findExecutable(""); err != nil {
		inspection.Message = "Chromium launch unavailable; attach remains supported: " + err.Error()
		return inspection
	}
	inspection.Capabilities = append(inspection.Capabilities, "launch")
	return inspection
}

func (Adapter) Start(
	ctx context.Context,
	spec manifest.EngineSpec,
	launch orchestrator.EngineRuntime,
) (orchestrator.EngineInstance, error) {
	config, err := decodeOptions(spec.Options)
	if err != nil {
		return nil, err
	}
	if config.Address == "" {
		config.Address = "http://127.0.0.1:9222"
	}
	parsedAddress, err := validateAddress(config.Address)
	if err != nil {
		return nil, err
	}
	timeout, err := parseTimeout(config.StartupTimeout)
	if err != nil {
		return nil, err
	}
	startupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if config.Attach {
		if err := waitReady(startupCtx, config.Address, nil); err != nil {
			return nil, fmt.Errorf("attach to Chromium: %w", err)
		}
		events := make(chan orchestrator.EngineEvent)
		close(events)
		return &instance{address: config.Address, events: events}, nil
	}
	if err := validateLaunchHost(parsedAddress.Hostname(), config.AllowRemoteDebugging); err != nil {
		return nil, err
	}

	executable, err := findExecutable(config.Executable)
	if err != nil {
		return nil, err
	}
	profileDir := strings.TrimSpace(config.ProfileDir)
	temporaryProfile := ""
	if profileDir == "" {
		profileDir, err = os.MkdirTemp("", "jangolova-chromium-profile-*")
		if err != nil {
			return nil, fmt.Errorf("create Chromium profile: %w", err)
		}
		temporaryProfile = profileDir
	} else if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, fmt.Errorf("create Chromium profile directory: %w", err)
	}

	headless := !hostDisplayAvailable(launch.Environment)
	if config.Headless != nil {
		headless = *config.Headless
	}
	args := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-dev-shm-usage",
		"--password-store=basic",
	}
	if headless {
		args = append(args, "--headless=new", "--disable-gpu")
	}
	if config.NoSandbox || os.Geteuid() == 0 {
		args = append(args, "--no-sandbox")
	}
	args = append(args, config.Args...)
	args = append(args,
		"--remote-debugging-address="+parsedAddress.Hostname(),
		"--remote-debugging-port="+parsedAddress.Port(),
		"--user-data-dir="+profileDir,
	)
	if source := strings.TrimSpace(spec.Source); source != "" {
		args = append(args, source)
	} else {
		args = append(args, "about:blank")
	}

	command := exec.Command(executable, args...)
	command.Env = engineEnvironment(launch.Environment)
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		if temporaryProfile != "" {
			os.RemoveAll(temporaryProfile)
		}
		return nil, fmt.Errorf("start Chromium: %w", err)
	}
	running := &instance{
		address:          config.Address,
		command:          command,
		done:             make(chan error, 1),
		events:           make(chan orchestrator.EngineEvent, 1),
		temporaryProfile: temporaryProfile,
	}
	go func() {
		waitErr := command.Wait()
		running.done <- waitErr
		close(running.done)
		event := orchestrator.EngineEvent{
			Type:       "engine.exited",
			Status:     "exited",
			OccurredAt: time.Now().UTC(),
		}
		if waitErr != nil {
			event.Type = "engine.failed"
			event.Status = "failed"
			event.Message = waitErr.Error()
		}
		running.events <- event
		close(running.events)
	}()
	if err := waitReady(startupCtx, config.Address, running.done); err != nil {
		_ = running.Stop(context.Background())
		return nil, fmt.Errorf("start Chromium: %w", err)
	}
	return running, nil
}

func hostDisplayAvailable(environment orchestrator.EngineEnvironment) bool {
	if strings.TrimSpace(environment["DISPLAY"]) != "" ||
		strings.TrimSpace(environment["WAYLAND_DISPLAY"]) != "" {
		return true
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return true
	}
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func decodeOptions(raw json.RawMessage) (options, error) {
	if len(raw) == 0 {
		return options{}, nil
	}
	var value options
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return options{}, fmt.Errorf("decode Chromium engine options: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return options{}, errors.New("decode Chromium engine options: multiple JSON values")
	}
	return value, nil
}

func validateAddress(address string) (*url.URL, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parse Chromium CDP address: %w", err)
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("Chromium CDP address scheme must be http, got %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("Chromium CDP address must include a host")
	}
	if parsed.Port() == "" {
		return nil, errors.New("Chromium CDP address must include a port")
	}
	return parsed, nil
}

func parseTimeout(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return defaultStartupTimeout, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("invalid Chromium startupTimeout %q", value)
	}
	return timeout, nil
}

func validateLaunchHost(host string, allowRemote bool) error {
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	if allowRemote {
		return nil
	}
	return fmt.Errorf(
		"launched Chromium CDP address must use a loopback host unless allowRemoteDebugging is enabled, got %q",
		host,
	)
}

func findExecutable(configured string) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		path, err := exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("find Chromium executable %q: %w", configured, err)
		}
		return path, nil
	}
	for _, name := range []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
	} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	if runtime.GOOS == "darwin" {
		paths := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths,
				filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
				filepath.Join(home, "Applications/Chromium.app/Contents/MacOS/Chromium"),
			)
		}
		for _, path := range paths {
			if resolved, err := exec.LookPath(path); err == nil {
				return resolved, nil
			}
		}
	}
	return "", errors.New("Chromium executable not found")
}

func engineEnvironment(overrides orchestrator.EngineEnvironment) []string {
	values := make(map[string]string)
	for _, item := range os.Environ() {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			values[name] = value
		}
	}
	for name, value := range overrides {
		values[name] = value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	environment := make([]string, 0, len(names))
	for _, name := range names {
		environment = append(environment, name+"="+values[name])
	}
	return environment
}

func waitReady(ctx context.Context, address string, processDone <-chan error) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		if err := probeCDP(ctx, address); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case err, open := <-processDone:
			if !open || err == nil {
				return errors.New("Chromium exited before CDP became ready")
			}
			return fmt.Errorf("Chromium exited before CDP became ready: %w", err)
		case <-ctx.Done():
			if lastErr != nil {
				return errors.Join(ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func probeCDP(ctx context.Context, address string) error {
	endpoint := strings.TrimRight(address, "/") + "/json/version"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("CDP health returned %s", response.Status)
	}
	return nil
}

func (i *instance) CDPEndpoint() string {
	return i.address
}

func (i *instance) EngineEndpoints() []engineprovider.Endpoint {
	parsed, _ := url.Parse(i.address)
	port, _ := strconv.Atoi(parsed.Port())
	return []engineprovider.Endpoint{{
		Name:       "cdp",
		Protocol:   "cdp",
		URL:        i.address,
		TargetPort: port,
		Visibility: "private",
	}}
}

func (i *instance) EngineEvents() <-chan orchestrator.EngineEvent {
	return i.events
}

func (i *instance) EngineHealth(ctx context.Context) orchestrator.EngineHealth {
	health := orchestrator.EngineHealth{ObservedAt: time.Now().UTC()}
	if err := probeCDP(ctx, i.address); err != nil {
		health.Status = orchestrator.EngineHealthUnhealthy
		health.Message = err.Error()
		return health
	}
	health.Status = orchestrator.EngineHealthHealthy
	return health
}

func (i *instance) Stop(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.command == nil {
		return nil
	}
	command := i.command
	done := i.done
	i.command = nil
	if command.Process != nil {
		_ = command.Process.Signal(os.Interrupt)
	}

	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		<-done
		waitErr = ctx.Err()
	case <-time.After(5 * time.Second):
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		waitErr = <-done
	}
	if i.temporaryProfile != "" {
		if err := os.RemoveAll(filepath.Clean(i.temporaryProfile)); err != nil {
			waitErr = errors.Join(waitErr, fmt.Errorf("remove temporary Chromium profile: %w", err))
		}
		i.temporaryProfile = ""
	}
	if waitErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			return nil
		}
	}
	return waitErr
}
