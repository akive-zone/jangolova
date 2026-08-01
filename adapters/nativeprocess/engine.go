// Package nativeprocess launches an arbitrary native executable as a
// Jangolova engine without imposing engine-specific behavior.
package nativeprocess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"jangolova/internal/bridge"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

const (
	defaultStartupGrace = 100 * time.Millisecond
	defaultStopTimeout  = 5 * time.Second
)

type Adapter struct{}

type options struct {
	Args               []string          `json:"args,omitempty"`
	WorkingDir         string            `json:"workingDir,omitempty"`
	Environment        map[string]string `json:"environment,omitempty"`
	InheritEnvironment *bool             `json:"inheritEnvironment,omitempty"`
	StartupGrace       string            `json:"startupGrace,omitempty"`
	StopTimeout        string            `json:"stopTimeout,omitempty"`
	Bridge             *bridgeOptions    `json:"bridge,omitempty"`
}

type bridgeOptions struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address,omitempty"`
}

type instance struct {
	mu          sync.Mutex
	healthMu    sync.RWMutex
	command     *exec.Cmd
	done        chan error
	events      chan orchestrator.EngineEvent
	stopTimeout time.Duration
	bridgeHost  *bridge.WebSocketHost
	health      orchestrator.EngineHealth
}

var _ orchestrator.EngineAdapter = Adapter{}
var _ orchestrator.EngineInstance = (*instance)(nil)
var _ orchestrator.EngineEventSource = (*instance)(nil)
var _ orchestrator.EngineHealthProvider = (*instance)(nil)
var _ orchestrator.EngineInspector = Adapter{}
var _ bridge.WebSocketHostProvider = (*instance)(nil)

func (Adapter) InspectEngine(context.Context) orchestrator.EngineInspection {
	return orchestrator.EngineInspection{
		Available: true,
		Capabilities: []string{
			"bridge.websocket",
			"events",
			"health",
			"launch",
			"runtime.environment",
			"runtime.handles",
			"stop",
		},
	}
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
	executable := strings.TrimSpace(spec.Source)
	if executable == "" {
		return nil, errors.New("native-process engine source executable is required")
	}
	executable, err = exec.LookPath(executable)
	if err != nil {
		return nil, fmt.Errorf("find native-process executable %q: %w", spec.Source, err)
	}
	workingDir, err := resolveWorkingDir(config.WorkingDir)
	if err != nil {
		return nil, err
	}
	startupGrace, err := parseDuration(
		"startupGrace",
		config.StartupGrace,
		defaultStartupGrace,
		true,
	)
	if err != nil {
		return nil, err
	}
	stopTimeout, err := parseDuration(
		"stopTimeout",
		config.StopTimeout,
		defaultStopTimeout,
		false,
	)
	if err != nil {
		return nil, err
	}
	var bridgeHost *bridge.WebSocketHost
	if config.Bridge != nil && config.Bridge.Enabled {
		bridgeHost, err = bridge.NewWebSocketHost(config.Bridge.Address)
		if err != nil {
			return nil, fmt.Errorf("start native-process bridge host: %w", err)
		}
	}
	environment, err := buildEnvironment(config, launch.Environment, bridgeHost)
	if err != nil {
		if bridgeHost != nil {
			_ = bridgeHost.Close(context.Background())
		}
		return nil, err
	}

	command := exec.Command(executable, config.Args...)
	command.Dir = workingDir
	command.Env = environment
	// Structured CLI and provider output is written on Jangolova's stdout.
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		if bridgeHost != nil {
			_ = bridgeHost.Close(context.Background())
		}
		return nil, fmt.Errorf("start native process: %w", err)
	}
	running := &instance{
		command:     command,
		done:        make(chan error, 1),
		events:      make(chan orchestrator.EngineEvent, 1),
		stopTimeout: stopTimeout,
		bridgeHost:  bridgeHost,
		health: orchestrator.EngineHealth{
			Status:     orchestrator.EngineHealthStarting,
			ObservedAt: time.Now().UTC(),
		},
	}
	go func() {
		waitErr := command.Wait()
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
		running.setHealth(event.Status, event.Message)
		running.done <- waitErr
		close(running.done)
		running.events <- event
		close(running.events)
	}()

	if startupGrace == 0 {
		running.markHealthy()
		return running, nil
	}
	timer := time.NewTimer(startupGrace)
	defer timer.Stop()
	select {
	case waitErr := <-running.done:
		running.command = nil
		if bridgeHost != nil {
			_ = bridgeHost.Close(context.Background())
		}
		if waitErr == nil {
			return nil, errors.New("native process exited during startup")
		}
		return nil, fmt.Errorf("native process exited during startup: %w", waitErr)
	case <-ctx.Done():
		_ = running.Stop(context.Background())
		return nil, fmt.Errorf("start native process: %w", ctx.Err())
	case <-timer.C:
		running.markHealthy()
		return running, nil
	}
}

func decodeOptions(raw json.RawMessage) (options, error) {
	if len(raw) == 0 {
		return options{}, nil
	}
	var value options
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return options{}, fmt.Errorf("decode native-process engine options: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return options{}, errors.New(
			"decode native-process engine options: multiple JSON values",
		)
	}
	return value, nil
}

func resolveWorkingDir(configured string) (string, error) {
	if strings.TrimSpace(configured) == "" {
		return "", nil
	}
	path, err := filepath.Abs(configured)
	if err != nil {
		return "", fmt.Errorf("resolve native-process workingDir: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("open native-process workingDir: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("native-process workingDir %q is not a directory", path)
	}
	return path, nil
}

func parseDuration(
	name string,
	configured string,
	fallback time.Duration,
	allowZero bool,
) (time.Duration, error) {
	if strings.TrimSpace(configured) == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(configured)
	if err != nil || value < 0 || (!allowZero && value == 0) {
		return 0, fmt.Errorf("invalid native-process %s %q", name, configured)
	}
	return value, nil
}

func buildEnvironment(
	config options,
	engineEnvironment orchestrator.EngineEnvironment,
	bridgeHost *bridge.WebSocketHost,
) ([]string, error) {
	inherit := true
	if config.InheritEnvironment != nil {
		inherit = *config.InheritEnvironment
	}
	values := make(map[string]string)
	if inherit {
		for _, item := range os.Environ() {
			name, value, ok := strings.Cut(item, "=")
			if ok {
				values[name] = value
			}
		}
	}
	for name, value := range engineEnvironment {
		if err := validateEnvironmentValue(name, value); err != nil {
			return nil, fmt.Errorf("engine environment: %w", err)
		}
		values[name] = value
	}
	for name, value := range config.Environment {
		if err := validateEnvironmentValue(name, value); err != nil {
			return nil, fmt.Errorf("native-process environment: %w", err)
		}
		values[name] = value
	}
	values["JANGOLOVA_ENGINE_ADAPTER"] = "native-process"
	if bridgeHost != nil {
		values["JANGOLOVA_BRIDGE_URL"] = bridgeHost.Endpoint()
		values["JANGOLOVA_BRIDGE_TOKEN"] = bridgeHost.Token()
		values["JANGOLOVA_BRIDGE_PROTOCOL"] = bridge.ProtocolVersion
	}

	environment := make([]string, 0, len(values))
	for name, value := range values {
		environment = append(environment, name+"="+value)
	}
	return environment, nil
}

func validateEnvironmentValue(name, value string) error {
	if strings.TrimSpace(name) == "" ||
		strings.Contains(name, "=") ||
		strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("invalid environment variable name %q", name)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("environment variable %q contains a null byte", name)
	}
	return nil
}

func (i *instance) ProcessID() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.command == nil || i.command.Process == nil {
		return 0
	}
	return i.command.Process.Pid
}

func (i *instance) BridgeWebSocketHost() *bridge.WebSocketHost {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.bridgeHost
}

func (i *instance) EngineEvents() <-chan orchestrator.EngineEvent {
	return i.events
}

func (i *instance) EngineHealth(context.Context) orchestrator.EngineHealth {
	i.healthMu.RLock()
	defer i.healthMu.RUnlock()
	return i.health
}

func (i *instance) markHealthy() {
	i.healthMu.Lock()
	defer i.healthMu.Unlock()
	if i.health.Status == orchestrator.EngineHealthStarting {
		i.health = orchestrator.EngineHealth{
			Status:     orchestrator.EngineHealthHealthy,
			ObservedAt: time.Now().UTC(),
		}
	}
}

func (i *instance) setHealth(status, message string) {
	healthStatus := status
	if status == "exited" {
		healthStatus = orchestrator.EngineHealthStopped
	} else if status == "failed" {
		healthStatus = orchestrator.EngineHealthUnhealthy
	}
	i.healthMu.Lock()
	i.health = orchestrator.EngineHealth{
		Status:     healthStatus,
		Message:    message,
		ObservedAt: time.Now().UTC(),
	}
	i.healthMu.Unlock()
}

func (i *instance) Stop(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	command := i.command
	done := i.done
	bridgeHost := i.bridgeHost
	i.command = nil
	i.bridgeHost = nil

	var processErr error
	if command != nil {
		processErr = stopProcess(ctx, command, done, i.stopTimeout)
	}
	var bridgeErr error
	if bridgeHost != nil {
		bridgeErr = bridgeHost.Close(ctx)
	}
	result := errors.Join(processErr, bridgeErr)
	if result != nil {
		i.setHealth(orchestrator.EngineHealthUnhealthy, result.Error())
	} else {
		i.setHealth(orchestrator.EngineHealthStopped, "")
	}
	return result
}

func stopProcess(
	ctx context.Context,
	command *exec.Cmd,
	done <-chan error,
	stopTimeout time.Duration,
) error {
	select {
	case waitErr := <-done:
		return normalizeWaitError(waitErr)
	default:
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		if killErr := command.Process.Kill(); killErr != nil {
			return fmt.Errorf(
				"stop native process after interrupt failed: %w",
				errors.Join(err, killErr),
			)
		}
	}
	timer := time.NewTimer(stopTimeout)
	defer timer.Stop()
	select {
	case waitErr := <-done:
		return normalizeWaitError(waitErr)
	case <-ctx.Done():
		_ = command.Process.Kill()
		<-done
		return ctx.Err()
	case <-timer.C:
		if err := command.Process.Kill(); err != nil {
			return fmt.Errorf("kill native process: %w", err)
		}
		waitErr := <-done
		return normalizeWaitError(waitErr)
	}
}

func normalizeWaitError(err error) error {
	if err == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return nil
	}
	return err
}
