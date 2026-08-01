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
	jsonOutput := flags.Bool("json", false, "write the interaction-engine inventory as JSON")
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
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", engine.Adapter, availability, strings.Join(engine.Capabilities, ","))
	}
	return nil
}

func connectEngineCommand(args []string) error {
	flags := flag.NewFlagSet("connect-engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	adapterName := flags.String("adapter", "auto", "interaction-engine adapter name or auto")
	targetKind := flags.String("target-kind", "", "caller-owned target kind")
	source := flags.String("source", "", "optional resource to present after connecting")
	optionsText := flags.String("options", "{}", "engine-specific options as a JSON object")
	disconnectTimeout := flags.Duration("disconnect-timeout", 15*time.Second, "maximum disconnect duration")
	var endpoints endpointFlags
	flags.Var(&endpoints, "endpoint", "caller-owned target endpoint in PROTOCOL=URL form; may be repeated")
	var handles handleFlags
	flags.Var(&handles, "handle", "opaque caller-owned target handle in NAME=VALUE form; may be repeated")
	var requiredCapabilities stringFlags
	flags.Var(&requiredCapabilities, "require-capability", "capability required when selecting an adapter; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("connect-engine accepts flags only")
	}
	if strings.TrimSpace(*targetKind) == "" {
		return errors.New("--target-kind is required")
	}
	if *disconnectTimeout <= 0 {
		return errors.New("--disconnect-timeout must be positive")
	}
	options, err := decodeEngineOptions(*optionsText)
	if err != nil {
		return err
	}
	registry, err := builtin.EngineRegistry()
	if err != nil {
		return err
	}
	selectedAdapter := strings.TrimSpace(*adapterName)
	if selectedAdapter == "auto" {
		providerEndpoints := make([]engineprovider.TargetEndpoint, 0, len(endpoints))
		for _, endpoint := range endpoints {
			providerEndpoints = append(providerEndpoints, engineprovider.TargetEndpoint{
				Name: endpoint.Name, Protocol: endpoint.Protocol, URL: endpoint.URL,
			})
		}
		selectedAdapter, err = engineprovider.SelectAutomaticEngine(context.Background(), registry, engineprovider.Target{
			Kind: strings.TrimSpace(*targetKind), Endpoints: providerEndpoints,
		}, requiredCapabilities)
		if err != nil {
			return err
		}
	}
	adapter, ok := registry.Engine(selectedAdapter)
	if !ok {
		return fmt.Errorf("interaction engine %q is not registered", selectedAdapter)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	instance, err := adapter.Connect(ctx, manifest.EngineSpec{
		Adapter:              selectedAdapter,
		RequiredCapabilities: append([]string(nil), requiredCapabilities...),
		Source:               strings.TrimSpace(*source),
		Options:              options,
	}, orchestrator.EngineTarget{
		Kind:      strings.TrimSpace(*targetKind),
		Endpoints: endpoints.clone(),
		Handles:   orchestrator.EngineHandles(handles.clone()),
	})
	if err != nil {
		return fmt.Errorf("connect interaction engine: %w", err)
	}
	if instance == nil {
		return errors.New("connect interaction engine: adapter returned no instance")
	}

	health := orchestrator.EngineHealth{
		Status:     orchestrator.EngineHealthUnknown,
		Message:    "interaction engine does not implement an active health probe",
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
		Adapter:    selectedAdapter,
		Status:     "connected",
		Health: engineprovider.Health{
			Status: health.Status, Message: health.Message, ObservedAt: health.ObservedAt,
		},
	}
	if provider, ok := instance.(orchestrator.EngineCapabilityProvider); ok {
		result.Capabilities = provider.EngineCapabilities()
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		_ = instance.Disconnect(context.Background())
		return err
	}

	var terminal *orchestrator.EngineEvent
	if source, ok := instance.(orchestrator.EngineEventSource); ok {
		for events := source.EngineEvents(); events != nil && terminal == nil; {
			select {
			case <-ctx.Done():
				events = nil
			case event, open := <-events:
				if !open {
					events = nil
					continue
				}
				if err := json.NewEncoder(os.Stdout).Encode(engineEventValue(event)); err != nil {
					_ = instance.Disconnect(context.Background())
					return err
				}
				if event.Status == "disconnected" || event.Status == "failed" {
					terminal = &event
				}
			}
		}
	}
	if terminal == nil && ctx.Err() == nil {
		<-ctx.Done()
	}
	disconnectCtx, cancel := context.WithTimeout(context.Background(), *disconnectTimeout)
	defer cancel()
	if err := instance.Disconnect(disconnectCtx); err != nil && terminal == nil {
		return err
	}
	if terminal != nil && terminal.Status == "failed" {
		return fmt.Errorf("interaction engine failed: %s", terminal.Message)
	}
	return nil
}

func engineEventValue(event orchestrator.EngineEvent) engineprovider.InstanceEvent {
	return engineprovider.InstanceEvent{Type: event.Type, Status: event.Status, Message: event.Message, OccurredAt: event.OccurredAt}
}

func decodeEngineOptions(value string) (json.RawMessage, error) {
	raw := json.RawMessage(strings.TrimSpace(value))
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, errors.New("--options must be a JSON object")
	}
	return raw, nil
}

var handleFlagNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,127}$`)

type endpointFlags []orchestrator.TargetEndpoint

type stringFlags []string

func (s *stringFlags) String() string { return strings.Join(*s, ",") }
func (s *stringFlags) Set(value string) error {
	value = strings.TrimSpace(value)
	if !handleFlagNamePattern.MatchString(value) {
		return errors.New("capability must use a protocol-style name")
	}
	*s = append(*s, value)
	return nil
}

func (e *endpointFlags) String() string { return fmt.Sprint([]orchestrator.TargetEndpoint(*e)) }

func (e *endpointFlags) Set(value string) error {
	protocol, endpointURL, ok := strings.Cut(value, "=")
	if !ok || !handleFlagNamePattern.MatchString(protocol) || strings.TrimSpace(endpointURL) == "" {
		return errors.New("--endpoint must use PROTOCOL=URL")
	}
	*e = append(*e, orchestrator.TargetEndpoint{Name: protocol, Protocol: protocol, URL: endpointURL})
	return nil
}

func (e endpointFlags) clone() []orchestrator.TargetEndpoint {
	return append([]orchestrator.TargetEndpoint(nil), e...)
}

type handleFlags map[string]string

func (h *handleFlags) String() string {
	values := make([]string, 0, len(*h))
	for name, value := range *h {
		values = append(values, name+"="+value)
	}
	return strings.Join(values, ",")
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
	values := make(map[string]string, len(h))
	for name, value := range h {
		values[name] = value
	}
	return values
}
