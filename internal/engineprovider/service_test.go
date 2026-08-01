package engineprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
	"jangolova/targetconn"
)

type fakeEngineAdapter struct {
	target       orchestrator.EngineTarget
	spec         manifest.EngineSpec
	instance     *fakeEngineInstance
	capabilities []string
	callResult   string
}

func (f *fakeEngineAdapter) Connect(
	_ context.Context,
	spec manifest.EngineSpec,
	target orchestrator.EngineTarget,
) (orchestrator.EngineInstance, error) {
	f.spec = spec
	f.target = target
	f.instance = &fakeEngineInstance{events: make(chan orchestrator.EngineEvent, 4), callResult: f.callResult}
	return f.instance, nil
}

func (f *fakeEngineAdapter) InspectEngine(context.Context) orchestrator.EngineInspection {
	return orchestrator.EngineInspection{Available: true, Capabilities: append([]string(nil), f.capabilities...)}
}

type fakeEngineInstance struct {
	disconnected bool
	events       chan orchestrator.EngineEvent
	once         sync.Once
	callResult   string
}

func (f *fakeEngineInstance) Disconnect(context.Context) error {
	f.disconnected = true
	f.once.Do(func() { close(f.events) })
	return nil
}

func (f *fakeEngineInstance) EngineEvents() <-chan orchestrator.EngineEvent { return f.events }
func (f *fakeEngineInstance) EngineCapabilities() []string                  { return []string{"describe", "act"} }
func (f *fakeEngineInstance) Call(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if f.callResult != "" {
		return json.Marshal(map[string]string{"value": f.callResult})
	}
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

type leakingEngineAdapter struct{}

func (leakingEngineAdapter) Connect(context.Context, manifest.EngineSpec, orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	return nil, errors.New("remote handshake echoed Bearer resolved-secret")
}

func TestServiceRedactsResolvedCredentialFromAdapterFailure(t *testing.T) {
	registry := orchestrator.NewRegistry()
	if err := registry.RegisterEngine("leaking", leakingEngineAdapter{}); err != nil {
		t.Fatal(err)
	}
	released := 0
	resolver := targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{
			Headers:   map[string]string{"Authorization": "Bearer resolved-secret"},
			ExpiresAt: time.Now().Add(time.Minute),
			Release:   func(context.Context) error { released++; return nil },
		}, nil
	})
	service, err := NewService(registry, "test-token", WithTargetResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(service.Routes(), http.MethodPost, "/v1/instances", `{
		"apiVersion":"interaction.engine/v1alpha1","instanceId":"redaction-one",
		"engine":{"adapter":"leaking"},
		"target":{"kind":"browser","endpoints":[{"name":"control","protocol":"cdp","url":"wss://browser.example/control","credentialRef":"session"}]}
	}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "resolved-secret") || !strings.Contains(response.Body.String(), "[REDACTED]") {
		t.Fatalf("response was not redacted: %s", response.Body.String())
	}
	if released != 1 {
		t.Fatalf("release count = %d", released)
	}
}

func TestServiceRedactsResolvedCredentialFromSuccessfulResult(t *testing.T) {
	registry := orchestrator.NewRegistry()
	adapter := &fakeEngineAdapter{callResult: "Bearer resolved-secret"}
	if err := registry.RegisterEngine("echo", adapter); err != nil {
		t.Fatal(err)
	}
	resolver := targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{
			Headers:   map[string]string{"Authorization": "Bearer resolved-secret"},
			ExpiresAt: time.Now().Add(time.Minute),
		}, nil
	})
	service, err := NewService(registry, "test-token", WithTargetResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}
	connected := performRequest(service.Routes(), http.MethodPost, "/v1/instances", `{
		"apiVersion":"interaction.engine/v1alpha1","instanceId":"redaction-result",
		"engine":{"adapter":"echo"},
		"target":{"kind":"browser","endpoints":[{"name":"control","protocol":"cdp","url":"wss://browser.example/control","credentialRef":"session"}]}
	}`)
	if connected.Code != http.StatusCreated {
		t.Fatalf("connect status = %d: %s", connected.Code, connected.Body.String())
	}
	response := performRequest(service.Routes(), http.MethodPost, "/v1/instances/redaction-result/call", `{"method":"describe","params":{}}`)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "resolved-secret") || !strings.Contains(response.Body.String(), "[REDACTED]") {
		t.Fatalf("response was not redacted: %d %s", response.Code, response.Body.String())
	}
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

func TestServiceAutomaticallySelectsEngineFromCallerSuppliedTarget(t *testing.T) {
	registry := orchestrator.NewRegistry()
	playwright := &fakeEngineAdapter{capabilities: []string{"target.cdp", "browser.evaluate"}}
	presentation := &fakeEngineAdapter{capabilities: []string{"target.cdp", "presentation.mount"}}
	if err := registry.RegisterEngine("playwright", playwright); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterEngine("web-presentation", presentation); err != nil {
		t.Fatal(err)
	}
	releaseCount := 0
	resolver := targetconn.ResolverFunc(func(_ context.Context, request targetconn.Request) (targetconn.Material, error) {
		switch request.Kind {
		case targetconn.CredentialReference:
			return targetconn.Material{
				Headers:   map[string]string{"Authorization": "Bearer resolved-secret"},
				ExpiresAt: time.Now().Add(time.Hour),
				Release:   func(context.Context) error { releaseCount++; return nil },
			}, nil
		case targetconn.TLSReference:
			return targetconn.Material{TLS: &orchestrator.TLSConnection{CAFile: "/caller/ca.pem"}}, nil
		default:
			return targetconn.Material{}, targetconn.ErrReferenceNotFound
		}
	})
	service, err := NewService(registry, "test-token", WithTargetResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(service.Routes(), http.MethodPost, "/v1/instances", `{
		"apiVersion":"interaction.engine/v1alpha1",
		"instanceId":"remote-presentation",
		"engine":{"adapter":"auto","requiredCapabilities":["presentation.mount"]},
		"target":{
			"apiVersion":"interaction.target/v1alpha1",
			"targetId":"remote-browser-42",
			"kind":"browser",
			"endpoints":[{
				"name":"browser-control",
				"protocol":"cdp",
				"url":"wss://browser.example/control/42",
				"credentialRef":"browser-session-42",
				"tlsRef":"browser-cluster-ca",
				"audience":"engine",
				"metadata":{"network.scope":"private"}
			}],
			"metadata":{"owner.kind":"external"}
		}
	}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("connect status = %d: %s", response.Code, response.Body.String())
	}
	var connected Instance
	if err := json.NewDecoder(response.Body).Decode(&connected); err != nil {
		t.Fatal(err)
	}
	if connected.Adapter != "web-presentation" {
		t.Fatalf("selected adapter = %q", connected.Adapter)
	}
	if playwright.instance != nil {
		t.Fatal("non-matching engine was connected")
	}
	if presentation.spec.Adapter != "web-presentation" || len(presentation.spec.RequiredCapabilities) != 1 {
		t.Fatalf("selected spec = %#v", presentation.spec)
	}
	if presentation.target.TargetID != "remote-browser-42" || presentation.target.APIVersion != TargetAPIVersion {
		t.Fatalf("target identity = %#v", presentation.target)
	}
	endpoint := presentation.target.Endpoints[0]
	if endpoint.URL != "wss://browser.example/control/42" || endpoint.CredentialRef != "browser-session-42" || endpoint.TLSRef != "browser-cluster-ca" || endpoint.Metadata["network.scope"] != "private" {
		t.Fatalf("forwarded endpoint = %#v", endpoint)
	}
	if endpoint.Connection == nil || endpoint.Connection.Headers["Authorization"] != "Bearer resolved-secret" || endpoint.Connection.TLS.CAFile != "/caller/ca.pem" {
		t.Fatalf("resolved connection = %#v", endpoint.Connection)
	}
	if strings.Contains(response.Body.String(), "resolved-secret") {
		t.Fatal("connection secret leaked in provider response")
	}
	response = performRequest(service.Routes(), http.MethodDelete, "/v1/instances/remote-presentation", "")
	if response.Code != http.StatusNoContent || releaseCount != 1 {
		t.Fatalf("disconnect status = %d, releases = %d", response.Code, releaseCount)
	}
}

func TestAutomaticSelectionDoesNotDependOnTargetLocation(t *testing.T) {
	registry := orchestrator.NewRegistry()
	adapter := &fakeEngineAdapter{capabilities: []string{"target.cdp", "browser.evaluate"}}
	if err := registry.RegisterEngine("playwright", adapter); err != nil {
		t.Fatal(err)
	}
	for _, endpointURL := range []string{
		"http://127.0.0.1:9222",
		"http://chromium:9222",
		"http://10.30.0.14:9222",
		"wss://browser.remote.example/control",
	} {
		selected, err := SelectAutomaticEngine(context.Background(), registry, Target{
			Kind:      "browser",
			Endpoints: []TargetEndpoint{{Name: "control", Protocol: "cdp", URL: endpointURL}},
		}, nil)
		if err != nil {
			t.Fatalf("select %s: %v", endpointURL, err)
		}
		if selected != "playwright" {
			t.Fatalf("select %s = %q", endpointURL, selected)
		}
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
