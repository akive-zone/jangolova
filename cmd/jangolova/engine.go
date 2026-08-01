package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"jangolova/internal/builtin"
	"jangolova/internal/engineprovider"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

func enginesCommand(args []string) error {
	flags := flag.NewFlagSet("engines", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write the engine inventory as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("engines accepts flags only")
	}
	registry, err := builtin.EngineRegistry()
	if err != nil {
		return err
	}
	engines := engineprovider.DiscoverEngines(context.Background(), registry)
	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"apiVersion": engineprovider.APIVersion,
			"engines":    engines,
		})
	}
	for _, engine := range engines {
		availability := "available"
		if !engine.Available {
			availability = "unavailable"
		}
		fmt.Fprintf(
			os.Stdout,
			"%s\t%s\t%s\n",
			engine.Adapter,
			availability,
			strings.Join(engine.Capabilities, ","),
		)
	}
	return nil
}

func launchEngineCommand(args []string) error {
	flags := flag.NewFlagSet("launch-engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	adapterName := flags.String("adapter", "", "engine adapter name")
	source := flags.String("source", "", "engine source, URL, project, or executable")
	optionsText := flags.String("options", "{}", "engine-specific options as a JSON object")
	stopTimeout := flags.Duration("stop-timeout", 15*time.Second, "maximum graceful stop duration")
	var environment environmentFlags
	flags.Var(&environment, "env", "environment override in KEY=VALUE form; may be repeated")
	var handles handleFlags
	flags.Var(&handles, "handle", "opaque runtime handle in NAME=VALUE form; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("launch-engine accepts flags only")
	}
	if strings.TrimSpace(*adapterName) == "" {
		return errors.New("--adapter is required")
	}
	if *stopTimeout <= 0 {
		return errors.New("--stop-timeout must be positive")
	}
	options, err := decodeEngineOptions(*optionsText)
	if err != nil {
		return err
	}
	registry, err := builtin.EngineRegistry()
	if err != nil {
		return err
	}
	adapter, ok := registry.Engine(strings.TrimSpace(*adapterName))
	if !ok {
		return fmt.Errorf("engine adapter %q is not registered", *adapterName)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	instance, err := adapter.Start(ctx, manifest.EngineSpec{
		Adapter: strings.TrimSpace(*adapterName),
		Source:  strings.TrimSpace(*source),
		Options: options,
	}, orchestrator.EngineRuntime{
		Environment: orchestrator.EngineEnvironment(environment.clone()),
		Handles:     orchestrator.EngineHandles(handles.clone()),
	})
	if err != nil {
		return fmt.Errorf("launch engine: %w", err)
	}
	if instance == nil {
		return errors.New("launch engine: engine adapter returned no instance")
	}

	health := orchestrator.EngineHealth{
		Status:     orchestrator.EngineHealthUnknown,
		Message:    "engine adapter does not implement an active health probe",
		ObservedAt: time.Now().UTC(),
	}
	if provider, ok := instance.(orchestrator.EngineHealthProvider); ok {
		healthCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		health = provider.EngineHealth(healthCtx)
		cancel()
	}
	result := engineprovider.Instance{
		APIVersion: engineprovider.APIVersion,
		InstanceID: "standalone",
		Adapter:    strings.TrimSpace(*adapterName),
		Status:     "running",
		Health: engineprovider.Health{
			Status:     health.Status,
			Message:    health.Message,
			ObservedAt: health.ObservedAt,
		},
		Endpoints: []engineprovider.Endpoint{},
	}
	if endpoints, ok := instance.(engineprovider.EndpointProvider); ok {
		result.Endpoints = endpoints.EngineEndpoints()
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		_ = instance.Stop(context.Background())
		return err
	}

	var terminal *orchestrator.EngineEvent
	if source, ok := instance.(orchestrator.EngineEventSource); ok {
		events := source.EngineEvents()
		for events != nil && terminal == nil {
			select {
			case <-ctx.Done():
				events = nil
			case event, open := <-events:
				if !open {
					events = nil
					continue
				}
				if err := json.NewEncoder(os.Stdout).Encode(engineEventValue(event)); err != nil {
					_ = instance.Stop(context.Background())
					return err
				}
				if event.Status == "exited" || event.Status == "failed" {
					terminal = &event
				}
			}
		}
	}
	if terminal == nil && ctx.Err() == nil {
		<-ctx.Done()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), *stopTimeout)
	defer cancel()
	if err := instance.Stop(shutdownCtx); err != nil {
		return err
	}
	if terminal != nil && terminal.Status == "failed" {
		return fmt.Errorf("engine failed: %s", terminal.Message)
	}
	return nil
}

func engineEventValue(event orchestrator.EngineEvent) engineprovider.InstanceEvent {
	return engineprovider.InstanceEvent{
		Type:       event.Type,
		Status:     event.Status,
		Message:    event.Message,
		OccurredAt: event.OccurredAt,
	}
}

func decodeEngineOptions(value string) (json.RawMessage, error) {
	raw := json.RawMessage(strings.TrimSpace(value))
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, errors.New("--options must be a JSON object")
	}
	return raw, nil
}

type environmentFlags map[string]string

func (e *environmentFlags) String() string {
	if e == nil || len(*e) == 0 {
		return ""
	}
	values := make([]string, 0, len(*e))
	for name, value := range *e {
		values = append(values, name+"="+value)
	}
	return strings.Join(values, ",")
}

func (e *environmentFlags) Set(value string) error {
	name, item, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.ContainsAny(name, "=\x00") {
		return errors.New("--env must use KEY=VALUE with a valid key")
	}
	if strings.ContainsRune(item, '\x00') {
		return fmt.Errorf("--env %s contains a null byte", name)
	}
	if *e == nil {
		*e = environmentFlags{}
	}
	(*e)[name] = item
	return nil
}

func (e environmentFlags) clone() map[string]string {
	values := make(map[string]string, len(e))
	for name, value := range e {
		values[name] = value
	}
	return values
}

var handleFlagNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,127}$`)

type handleFlags map[string]string

func (h *handleFlags) String() string {
	return (*environmentFlags)(h).String()
}

func (h *handleFlags) Set(value string) error {
	name, item, ok := strings.Cut(value, "=")
	if !ok || !handleFlagNamePattern.MatchString(name) || item == "" {
		return errors.New("--handle must use NAME=VALUE with a valid name and non-empty value")
	}
	if strings.ContainsRune(item, '\x00') {
		return fmt.Errorf("--handle %s contains a null byte", name)
	}
	if *h == nil {
		*h = handleFlags{}
	}
	(*h)[name] = item
	return nil
}

func (h handleFlags) clone() map[string]string {
	return environmentFlags(h).clone()
}
