package engineprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

type fakeEngineAdapter struct {
	target   orchestrator.EngineTarget
	instance *fakeEngineInstance
}

func (f *fakeEngineAdapter) Connect(
	_ context.Context,
	_ manifest.EngineSpec,
	target orchestrator.EngineTarget,
) (orchestrator.EngineInstance, error) {
	f.target = target
	f.instance = &fakeEngineInstance{events: make(chan orchestrator.EngineEvent, 4)}
	return f.instance, nil
}

type fakeEngineInstance struct {
	disconnected bool
	events       chan orchestrator.EngineEvent
	once         sync.Once
}

func (f *fakeEngineInstance) Disconnect(context.Context) error {
	f.disconnected = true
	f.once.Do(func() { close(f.events) })
	return nil
}

func (f *fakeEngineInstance) EngineEvents() <-chan orchestrator.EngineEvent { return f.events }
func (f *fakeEngineInstance) EngineCapabilities() []string                  { return []string{"describe", "act"} }
func (f *fakeEngineInstance) Call(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"method": method, "params": params})
}

type nilEngineAdapter struct{}

func (nilEngineAdapter) Connect(context.Context, manifest.EngineSpec, orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	return nil, nil
}

type healthEngineAdapter struct{}
type healthEngineInstance struct{}

func (healthEngineAdapter) Connect(context.Context, manifest.EngineSpec, orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	return healthEngineInstance{}, nil
}
func (healthEngineInstance) Disconnect(context.Context) error { return nil }
func (healthEngineInstance) EngineHealth(context.Context) orchestrator.EngineHealth {
	return orchestrator.EngineHealth{Status: orchestrator.EngineHealthUnhealthy, Message: "fixture probe failed", ObservedAt: time.Now().UTC()}
}

func TestServiceConnectsCallsAndDisconnectsEngine(t *testing.T) {
	t.Parallel()
	registry := orchestrator.NewRegistry()
	adapter := &fakeEngineAdapter{}
	if err := registry.RegisterEngine("playwright", adapter); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(registry, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	handler := service.Routes()

	body := `{
		"apiVersion":"interaction.engine/v1alpha1",
		"instanceId":"browser-one",
		"engine":{"adapter":"playwright"},
		"target":{"kind":"browser","endpoints":[{"name":"cdp","protocol":"cdp","url":"http://127.0.0.1:9222"}],"handles":{"native.window":"window-1234"}}
	}`
	response := performRequest(handler, http.MethodPost, "/v1/instances", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("connect status = %d: %s", response.Code, response.Body.String())
	}
	var instance Instance
	if err := json.NewDecoder(response.Body).Decode(&instance); err != nil {
		t.Fatal(err)
	}
	if instance.InstanceID != "browser-one" || instance.Status != "connected" || len(instance.Capabilities) != 2 {
		t.Fatalf("instance = %#v", instance)
	}
	if len(adapter.target.Endpoints) != 1 || adapter.target.Endpoints[0].URL != "http://127.0.0.1:9222" {
		t.Fatalf("target = %#v", adapter.target)
	}
	if adapter.target.Handles["native.window"] != "window-1234" {
		t.Fatalf("handles = %#v", adapter.target.Handles)
	}

	response = performRequest(handler, http.MethodPost, "/v1/instances/browser-one/call", `{"method":"describe","params":{}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("call status = %d: %s", response.Code, response.Body.String())
	}
	var call CallResponse
	if err := json.NewDecoder(response.Body).Decode(&call); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(call.Result), `"method":"describe"`) {
		t.Fatalf("call = %#v", call)
	}

	adapter.instance.events <- orchestrator.EngineEvent{Type: "interaction.failed", Status: "failed", Message: "fixture exited", OccurredAt: time.Now().UTC()}
	deadline := time.Now().Add(time.Second)
	for {
		response = performRequest(handler, http.MethodGet, "/v1/instances/browser-one/events?after=2", "")
		var batch InstanceEventBatch
		if err := json.NewDecoder(response.Body).Decode(&batch); err != nil {
			t.Fatal(err)
		}
		if len(batch.Events) == 1 {
			if batch.Events[0].Type != "interaction.failed" || batch.Cursor != "3" {
				t.Fatalf("events = %#v", batch)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal event not observed: %#v", batch)
		}
		time.Sleep(time.Millisecond)
	}

	response = performRequest(handler, http.MethodDelete, "/v1/instances/browser-one", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("disconnect status = %d", response.Code)
	}
	if !adapter.instance.disconnected {
		t.Fatal("interaction engine was not disconnected")
	}
}

func TestServiceRequiresAuthorization(t *testing.T) {
	t.Parallel()
	service, err := NewService(orchestrator.NewRegistry(), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/engines", nil)
	response := httptest.NewRecorder()
	service.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestServiceRejectsEmptyEngineInstance(t *testing.T) {
	t.Parallel()
	registry := orchestrator.NewRegistry()
	if err := registry.RegisterEngine("empty", nilEngineAdapter{}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(registry, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(service.Routes(), http.MethodPost, "/v1/instances", `{
		"apiVersion":"interaction.engine/v1alpha1","instanceId":"empty-one",
		"engine":{"adapter":"empty"},"target":{"kind":"browser"}
	}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("connect status = %d, want 502", response.Code)
	}
}

func TestServiceActivelyProbesInstanceHealth(t *testing.T) {
	t.Parallel()
	registry := orchestrator.NewRegistry()
	if err := registry.RegisterEngine("health-fixture", healthEngineAdapter{}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(registry, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(service.Routes(), http.MethodPost, "/v1/instances", `{
		"apiVersion":"interaction.engine/v1alpha1","instanceId":"health-one",
		"engine":{"adapter":"health-fixture"},"target":{"kind":"fixture"}
	}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("connect status = %d: %s", response.Code, response.Body.String())
	}
	response = performRequest(service.Routes(), http.MethodGet, "/v1/instances/health-one", "")
	var instance Instance
	if err := json.NewDecoder(response.Body).Decode(&instance); err != nil {
		t.Fatal(err)
	}
	if instance.Health.Status != orchestrator.EngineHealthUnhealthy || instance.Health.Message != "fixture probe failed" {
		t.Fatalf("health = %#v", instance.Health)
	}
}

func TestValidateConnectRequestRejectsInvalidTargets(t *testing.T) {
	t.Parallel()
	base := ConnectRequest{APIVersion: APIVersion, InstanceID: "engine-one", Engine: EngineSpec{Adapter: "playwright"}, Target: Target{Kind: "browser"}}
	tests := []ConnectRequest{base, base, base}
	tests[0].Target.Kind = ""
	tests[1].Target.Endpoints = []TargetEndpoint{{Name: "cdp/path", Protocol: "cdp", URL: "http://localhost:9222"}}
	tests[2].Target.Handles = map[string]string{"native.window": ""}
	for index, request := range tests {
		if err := validateConnectRequest(request); err == nil {
			t.Fatalf("invalid target %d was accepted", index)
		}
	}
}

func TestEventQueryValidation(t *testing.T) {
	t.Parallel()
	if cursor, err := parseCursor("12"); err != nil || cursor != 12 {
		t.Fatalf("parseCursor() = %d, %v", cursor, err)
	}
	if _, err := parseCursor("later"); err == nil {
		t.Fatal("invalid cursor was accepted")
	}
	if limit, err := parseEventLimit("256"); err != nil || limit != 256 {
		t.Fatalf("parseEventLimit() = %d, %v", limit, err)
	}
	if _, err := parseEventLimit("257"); err == nil {
		t.Fatal("oversized limit was accepted")
	}
}

func performRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer test-token")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
