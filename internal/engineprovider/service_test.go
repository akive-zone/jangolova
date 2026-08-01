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
	environment map[string]string
	handles     map[string]string
	instance    *fakeEngineInstance
}

func (f *fakeEngineAdapter) Start(
	_ context.Context,
	_ manifest.EngineSpec,
	launch orchestrator.EngineRuntime,
) (orchestrator.EngineInstance, error) {
	f.environment = launch.Environment
	f.handles = launch.Handles
	f.instance = &fakeEngineInstance{events: make(chan orchestrator.EngineEvent, 4)}
	return f.instance, nil
}

type fakeEngineInstance struct {
	stopped bool
	events  chan orchestrator.EngineEvent
	once    sync.Once
}

type nilEngineAdapter struct{}

type healthEngineAdapter struct{}

type healthEngineInstance struct{}

func (nilEngineAdapter) Start(
	context.Context,
	manifest.EngineSpec,
	orchestrator.EngineRuntime,
) (orchestrator.EngineInstance, error) {
	return nil, nil
}

func (healthEngineAdapter) Start(
	context.Context,
	manifest.EngineSpec,
	orchestrator.EngineRuntime,
) (orchestrator.EngineInstance, error) {
	return healthEngineInstance{}, nil
}

func (healthEngineInstance) Stop(context.Context) error { return nil }

func (healthEngineInstance) EngineHealth(context.Context) orchestrator.EngineHealth {
	return orchestrator.EngineHealth{
		Status:     orchestrator.EngineHealthUnhealthy,
		Message:    "fixture probe failed",
		ObservedAt: time.Now().UTC(),
	}
}

func (f *fakeEngineInstance) Stop(context.Context) error {
	f.stopped = true
	f.once.Do(func() { close(f.events) })
	return nil
}

func (f *fakeEngineInstance) EngineEvents() <-chan orchestrator.EngineEvent {
	return f.events
}

func (f *fakeEngineInstance) EngineEndpoints() []Endpoint {
	return []Endpoint{{
		Name:       "cdp",
		Protocol:   "cdp",
		URL:        "http://127.0.0.1:9222",
		Visibility: "private",
	}}
}

func TestServiceLaunchesAndStopsEngine(t *testing.T) {
	t.Parallel()

	registry := orchestrator.NewRegistry()
	adapter := &fakeEngineAdapter{}
	if err := registry.RegisterEngine("chromium", adapter); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(registry, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	handler := service.Routes()

	body := `{
		"apiVersion":"jangolova.engine/v1alpha1",
		"instanceId":"browser-one",
		"engine":{"adapter":"chromium","source":"about:blank"},
		"environment":{"DISPLAY":":99"},
		"handles":{"native.window":"window-1234"}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/instances",
		strings.NewReader(body),
	)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("launch status = %d", response.Code)
	}
	var instance Instance
	if err := json.NewDecoder(response.Body).Decode(&instance); err != nil {
		t.Fatal(err)
	}
	if instance.InstanceID != "browser-one" ||
		len(instance.Endpoints) != 1 ||
		instance.Endpoints[0].Protocol != "cdp" {
		t.Fatalf("instance = %#v", instance)
	}
	if instance.Health.Status != orchestrator.EngineHealthHealthy {
		t.Fatalf("instance health = %#v", instance.Health)
	}
	if adapter.environment["DISPLAY"] != ":99" {
		t.Fatalf("environment = %#v", adapter.environment)
	}
	if adapter.handles["native.window"] != "window-1234" {
		t.Fatalf("handles = %#v", adapter.handles)
	}

	adapter.instance.events <- orchestrator.EngineEvent{
		Type:       "engine.failed",
		Status:     "failed",
		Message:    "fixture exited",
		OccurredAt: time.Now().UTC(),
	}
	deadline := time.Now().Add(time.Second)
	for {
		request = httptest.NewRequest(
			http.MethodGet,
			"/v1/instances/browser-one/events?after=2",
			nil,
		)
		request.Header.Set("Authorization", "Bearer test-token")
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var batch InstanceEventBatch
		decodeErr := json.NewDecoder(response.Body).Decode(&batch)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if len(batch.Events) == 1 {
			if batch.Events[0].Type != "engine.failed" || batch.Cursor != "3" {
				t.Fatalf("event batch = %#v", batch)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal event was not observed: %#v", batch)
		}
		time.Sleep(time.Millisecond)
	}
	request = httptest.NewRequest(
		http.MethodGet,
		"/v1/instances/browser-one",
		nil,
	)
	request.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if err := json.NewDecoder(response.Body).Decode(&instance); err != nil {
		t.Fatal(err)
	}
	if instance.Status != "failed" {
		t.Fatalf("instance status = %q, want failed", instance.Status)
	}
	if instance.Health.Status != orchestrator.EngineHealthUnhealthy {
		t.Fatalf("instance health = %#v", instance.Health)
	}

	request = httptest.NewRequest(
		http.MethodDelete,
		"/v1/instances/browser-one",
		nil,
	)
	request.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("stop status = %d", response.Code)
	}
	if !adapter.instance.stopped {
		t.Fatal("engine was not stopped")
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
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/instances",
		strings.NewReader(`{
			"apiVersion":"jangolova.engine/v1alpha1",
			"instanceId":"empty-one",
			"engine":{"adapter":"empty"}
		}`),
	)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	service.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("launch status = %d, want %d", response.Code, http.StatusBadGateway)
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
	launch := httptest.NewRequest(
		http.MethodPost,
		"/v1/instances",
		strings.NewReader(`{
			"apiVersion":"jangolova.engine/v1alpha1",
			"instanceId":"health-one",
			"engine":{"adapter":"health-fixture"}
		}`),
	)
	launch.Header.Set("Authorization", "Bearer test-token")
	launchResponse := httptest.NewRecorder()
	service.Routes().ServeHTTP(launchResponse, launch)
	if launchResponse.Code != http.StatusCreated {
		t.Fatalf("launch status = %d", launchResponse.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/instances/health-one", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	service.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	var instance Instance
	if err := json.NewDecoder(response.Body).Decode(&instance); err != nil {
		t.Fatal(err)
	}
	if instance.Health.Status != orchestrator.EngineHealthUnhealthy ||
		instance.Health.Message != "fixture probe failed" {
		t.Fatalf("instance health = %#v", instance.Health)
	}

	eventsRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/instances/health-one/events?after=2",
		nil,
	)
	eventsRequest.Header.Set("Authorization", "Bearer test-token")
	eventsResponse := httptest.NewRecorder()
	service.Routes().ServeHTTP(eventsResponse, eventsRequest)
	var batch InstanceEventBatch
	if err := json.NewDecoder(eventsResponse.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 || batch.Events[0].Type != "engine.health.unhealthy" {
		t.Fatalf("health events = %#v", batch.Events)
	}
}

func TestValidateLaunchRequestRejectsInvalidHandles(t *testing.T) {
	t.Parallel()

	base := LaunchRequest{
		APIVersion: APIVersion,
		InstanceID: "engine-one",
		Engine:     EngineSpec{Adapter: "native-process"},
	}
	for name, value := range map[string]string{
		"native/window": "value",
		"native.window": "",
		"native.layer":  "bad\x00value",
	} {
		request := base
		request.Handles = map[string]string{name: value}
		if err := validateLaunchRequest(request); err == nil {
			t.Fatalf("handle %q=%q was accepted", name, value)
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
		t.Fatal("oversized event limit was accepted")
	}
}
