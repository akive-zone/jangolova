package displayinteraction_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"jangolova/adapters/displayinteraction"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

func TestDisplayInteractionInspectEngine(t *testing.T) {
	adapter := displayinteraction.Adapter{}
	inspection := adapter.InspectEngine(context.Background())

	if !inspection.Available {
		t.Fatalf("expected engine to be available")
	}

	expectedCaps := []string{
		"act", "capabilities", "describe", "events", "health",
		"display.describe", "display.capture",
		"pointer.move", "pointer.click", "pointer.drag", "pointer.scroll",
		"keyboard.type", "keyboard.press",
	}

	for _, cap := range expectedCaps {
		found := false
		for _, c := range inspection.Capabilities {
			if c == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected capability %q in inspection", cap)
		}
	}
}

func TestDisplayInteractionConnectAndActions(t *testing.T) {
	adapter := displayinteraction.Adapter{}

	target := orchestrator.EngineTarget{
		APIVersion: "interaction.target/v1alpha1",
		TargetID:   "test-linux-display",
		Kind:       "display",
		Endpoints: []orchestrator.TargetEndpoint{
			{
				Name:     "vnc-main",
				Protocol: "vnc",
				URL:      "vnc://127.0.0.1:5900",
			},
		},
	}

	ctx := context.Background()
	inst, err := adapter.Connect(ctx, manifest.EngineSpec{}, target)
	if err != nil {
		t.Fatalf("failed to connect display-interaction: %v", err)
	}

	caller, ok := inst.(interface {
		Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
	})
	if !ok {
		t.Fatalf("instance does not implement bridge.Caller")
	}

	// Test hello method
	helloRes, err := caller.Call(ctx, "hello", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("hello call failed: %v", err)
	}
	if !strings.Contains(string(helloRes), "jangolova.display/v1alpha1") {
		t.Errorf("expected hello protocol jangolova.display/v1alpha1, got %s", helloRes)
	}

	// Test display.describe action
	actDescribeRes, err := caller.Call(ctx, "act", json.RawMessage(`{"name":"display.describe","input":{}}`))
	if err != nil {
		t.Fatalf("act display.describe failed: %v", err)
	}
	if !strings.Contains(string(actDescribeRes), "1920") {
		t.Errorf("expected width 1920 in describe result, got %s", actDescribeRes)
	}

	// Test display.capture action
	actCaptureRes, err := caller.Call(ctx, "act", json.RawMessage(`{"name":"display.capture","input":{}}`))
	if err != nil {
		t.Fatalf("act display.capture failed: %v", err)
	}
	if !strings.Contains(string(actCaptureRes), "image/png") {
		t.Errorf("expected format image/png in capture result, got %s", actCaptureRes)
	}

	// Test pointer.click action
	actClickRes, err := caller.Call(ctx, "act", json.RawMessage(`{"name":"pointer.click","input":{"x":450,"y":320,"button":"left"}}`))
	if err != nil {
		t.Fatalf("act pointer.click failed: %v", err)
	}
	if !strings.Contains(string(actClickRes), "ok") {
		t.Errorf("expected ok status in click result, got %s", actClickRes)
	}

	// Test keyboard.type action
	actTypeRes, err := caller.Call(ctx, "act", json.RawMessage(`{"name":"keyboard.type","input":{"text":"hello world"}}`))
	if err != nil {
		t.Fatalf("act keyboard.type failed: %v", err)
	}
	if !strings.Contains(string(actTypeRes), "ok") {
		t.Errorf("expected ok status in type result, got %s", actTypeRes)
	}

	// Test Disconnect
	if err := inst.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
}
