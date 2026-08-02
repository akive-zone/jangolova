package cymonkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"jangolova/internal/bridge"
)

type ConformanceReport struct {
	Backends     []BackendName `json:"backends"`
	Capabilities []Capability  `json:"capabilities"`
	EventCursor  string        `json:"eventCursor"`
}

// ValidateConformance applies the same semantic handshake checks to CDP,
// BiDi, Safari MCP mappings, and WebExtension-backed Cymonkey callers.
func ValidateConformance(ctx context.Context, caller bridge.Caller) (ConformanceReport, error) {
	if caller == nil {
		return ConformanceReport{}, errors.New("Cymonkey conformance caller is required")
	}
	var hello Hello
	if err := cymonkeyCall(ctx, caller, bridge.MethodHello, `{}`, &hello); err != nil {
		return ConformanceReport{}, err
	}
	if hello.ProtocolVersion != ProtocolVersion || strings.TrimSpace(hello.Implementation.Name) == "" || len(hello.Backends) == 0 {
		return ConformanceReport{}, fmt.Errorf("incompatible Cymonkey hello: %#v", hello)
	}
	var capabilities []Capability
	if err := cymonkeyCall(ctx, caller, bridge.MethodCapabilities, `{}`, &capabilities); err != nil {
		return ConformanceReport{}, err
	}
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == "" || capability.Backend == "" || capability.Support == "" || capability.Lifetime == "" || capability.Persistence == "" || capability.Effect == "" || !json.Valid(capability.InputSchema) {
			return ConformanceReport{}, fmt.Errorf("invalid Cymonkey capability descriptor %#v", capability)
		}
		if _, exists := seen[capability.Name]; exists {
			return ConformanceReport{}, fmt.Errorf("duplicate Cymonkey capability %q", capability.Name)
		}
		seen[capability.Name] = struct{}{}
	}
	var description any
	if err := cymonkeyCall(ctx, caller, bridge.MethodDescribe, `{}`, &description); err != nil {
		return ConformanceReport{}, err
	}
	var batch bridge.EventBatch
	if err := cymonkeyCall(ctx, caller, bridge.MethodEvents, `{"limit":1}`, &batch); err != nil {
		return ConformanceReport{}, err
	}
	if strings.TrimSpace(batch.Cursor) == "" {
		return ConformanceReport{}, errors.New("Cymonkey events cursor is required")
	}
	return ConformanceReport{Backends: hello.Backends, Capabilities: capabilities, EventCursor: batch.Cursor}, nil
}

func cymonkeyCall(ctx context.Context, caller bridge.Caller, method, params string, destination any) error {
	raw, err := caller.Call(ctx, method, json.RawMessage(params))
	if err != nil {
		return fmt.Errorf("Cymonkey %s: %w", method, err)
	}
	if !json.Valid(raw) {
		return fmt.Errorf("Cymonkey %s returned invalid JSON", method)
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return fmt.Errorf("decode Cymonkey %s: %w", method, err)
	}
	return nil
}
