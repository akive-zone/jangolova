package grimlock

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"

	"jangolova/internal/bridge"
)

type toolFixtureCall struct {
	method string
	input  json.RawMessage
}

type toolFixtureCaller struct {
	capabilities []bridge.Capability
	results      map[string]json.RawMessage
	err          error

	mu    sync.Mutex
	calls []toolFixtureCall
}

func (f *toolFixtureCaller) Call(_ context.Context, method string, input json.RawMessage) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls = append(f.calls, toolFixtureCall{method: method, input: append(json.RawMessage(nil), input...)})
	f.mu.Unlock()
	if method == bridge.MethodCapabilities {
		return json.Marshal(f.capabilities)
	}
	if f.err != nil {
		return nil, f.err
	}
	if result, ok := f.results[method]; ok {
		return append(json.RawMessage(nil), result...), nil
	}
	return json.RawMessage(`{"ok":true}`), nil
}

func fixtureCapabilities() []bridge.Capability {
	return []bridge.Capability{
		{
			Name: "scene.observe", Description: "Observe the scene.", Effect: bridge.EffectRead,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string"}},"required":["selector"],"additionalProperties":false}`),
		},
		{
			Name: "scene.click", Description: "Click a scene object.", Effect: bridge.EffectWrite,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`),
		},
	}
}

func interactionToolByName(t *testing.T, tools []tool.Tool, name string) *interactionTool {
	t.Helper()
	for _, candidate := range tools {
		if candidate.Name() == name {
			interactionTool, ok := candidate.(*interactionTool)
			if !ok {
				t.Fatalf("tool %q has type %T", name, candidate)
			}
			return interactionTool
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func toolNames(tools []tool.Tool) []string {
	names := make([]string, len(tools))
	for index, candidate := range tools {
		names[index] = candidate.Name()
	}
	return names
}

func TestInteractionToolsDefaultToReadOnly(t *testing.T) {
	caller := &toolFixtureCaller{capabilities: fixtureCapabilities()}
	tools, err := InteractionTools(t.Context(), InteractionToolSpec{
		SessionID: "application-one", InteractionID: "browser-one", Caller: caller,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"jangolova_browser_one_describe", "jangolova_browser_one_events", "jangolova_browser_one_scene_observe"}
	if got := toolNames(tools); !reflect.DeepEqual(got, want) {
		t.Fatalf("tool names = %v, want %v", got, want)
	}

	observe := interactionToolByName(t, tools, "jangolova_browser_one_scene_observe")
	var modelRequest model.LLMRequest
	if err := observe.ProcessRequest(nil, &modelRequest); err != nil {
		t.Fatal(err)
	}
	if modelRequest.Config == nil || len(modelRequest.Config.Tools) != 1 || modelRequest.Config.Tools[0].FunctionDeclarations[0].Name != observe.Name() {
		t.Fatalf("packed model request = %#v", modelRequest.Config)
	}
	result, err := observe.execute(t.Context(), map[string]any{"selector": "#score"})
	if err != nil {
		t.Fatal(err)
	}
	if result["interactionId"] != "browser-one" || result["effect"] != bridge.EffectRead {
		t.Fatalf("result = %#v", result)
	}
	caller.mu.Lock()
	calls := append([]toolFixtureCall(nil), caller.calls...)
	caller.mu.Unlock()
	if len(calls) != 2 || calls[1].method != bridge.MethodAct || !strings.Contains(string(calls[1].input), `"name":"scene.observe"`) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestInteractionToolRequiresSchemaPolicyAndApprovalForWrite(t *testing.T) {
	caller := &toolFixtureCaller{capabilities: fixtureCapabilities()}
	var authorizationInputs []string
	policy := CapabilityPolicyFuncs{
		AdvertiseFunc: func(context.Context, CapabilityRequest) bool { return true },
		AuthorizeFunc: func(_ context.Context, request CapabilityRequest) error {
			authorizationInputs = append(authorizationInputs, string(request.Input))
			if request.Capability.Name == "scene.click" && strings.Contains(string(request.Input), "denied") {
				return errors.New("application policy denied click")
			}
			return nil
		},
	}
	tools, err := InteractionTools(t.Context(), InteractionToolSpec{
		SessionID: "application-one", InteractionID: "unity-one", Caller: caller, Policy: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	click := interactionToolByName(t, tools, "jangolova_unity_one_scene_click")
	if !click.requireApproval {
		t.Fatal("write capability does not require approval")
	}
	if _, err := click.execute(t.Context(), map[string]any{}); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("missing required input error = %v", err)
	}
	if len(authorizationInputs) != 0 {
		t.Fatalf("authorization ran for invalid input: %v", authorizationInputs)
	}
	if _, err := click.execute(t.Context(), map[string]any{"id": "denied"}); err == nil || !strings.Contains(err.Error(), "application policy denied click") {
		t.Fatalf("policy denial = %v", err)
	}
	caller.mu.Lock()
	callCount := len(caller.calls)
	caller.mu.Unlock()
	if callCount != 1 { // capability discovery only
		t.Fatalf("caller invoked before authorization: %d calls", callCount)
	}
	if _, err := click.execute(t.Context(), map[string]any{"id": "play"}); err != nil {
		t.Fatal(err)
	}
	if len(authorizationInputs) != 2 || !strings.Contains(authorizationInputs[1], `"id":"play"`) {
		t.Fatalf("authorization inputs = %v", authorizationInputs)
	}
}

func TestInteractionToolsRejectMissingAllowlistedCapability(t *testing.T) {
	caller := &toolFixtureCaller{capabilities: fixtureCapabilities()}
	_, err := InteractionTools(t.Context(), InteractionToolSpec{
		SessionID: "application-one", InteractionID: "browser-one", Caller: caller,
		AllowedCapabilities: []string{"scene.missing"},
	})
	if err == nil || !strings.Contains(err.Error(), "was not advertised") {
		t.Fatalf("InteractionTools() error = %v", err)
	}
}

func TestInteractionToolsRejectAmbiguousToolNames(t *testing.T) {
	caller := &toolFixtureCaller{capabilities: []bridge.Capability{
		{Name: "scene.click", Effect: bridge.EffectRead, InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "scene-click", Effect: bridge.EffectRead, InputSchema: json.RawMessage(`{"type":"object"}`)},
	}}
	_, err := InteractionTools(t.Context(), InteractionToolSpec{
		SessionID: "application-one", InteractionID: "browser-one", Caller: caller,
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate tool name") {
		t.Fatalf("InteractionTools() error = %v", err)
	}
}
