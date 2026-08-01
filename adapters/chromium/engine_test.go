package chromium

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

func TestAdapterAttachesToExistingChromium(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != "/json/version" {
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"Browser": "Fake Chromium",
		})
	}))
	defer server.Close()

	raw, err := json.Marshal(map[string]any{
		"address": server.URL,
		"attach":  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handle, err := (Adapter{}).Start(
		context.Background(),
		manifest.EngineSpec{Options: raw},
		orchestrator.EngineRuntime{},
	)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	instance, ok := handle.(*instance)
	if !ok {
		t.Fatalf("Start() returned %T, want *instance", handle)
	}
	if got := instance.CDPEndpoint(); got != server.URL {
		t.Fatalf("CDPEndpoint() = %q, want %q", got, server.URL)
	}
	health := instance.EngineHealth(context.Background())
	if health.Status != orchestrator.EngineHealthHealthy {
		t.Fatalf("EngineHealth() = %#v", health)
	}
	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestInspectionAlwaysAdvertisesAttach(t *testing.T) {
	t.Parallel()

	inspection := (Adapter{}).InspectEngine(context.Background())
	if !inspection.Available {
		t.Fatalf("InspectEngine() = %#v", inspection)
	}
	found := false
	for _, capability := range inspection.Capabilities {
		if capability == "attach" {
			found = true
		}
	}
	if !found {
		t.Fatalf("InspectEngine() capabilities = %#v", inspection.Capabilities)
	}
}

func TestAdapterRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]json.RawMessage{
		"unknown field": json.RawMessage(`{"unknown":true}`),
		"missing port":  json.RawMessage(`{"address":"http://localhost","attach":true}`),
		"bad timeout": json.RawMessage(
			`{"address":"http://localhost:9222","startupTimeout":"later","attach":true}`,
		),
		"exposed launch": json.RawMessage(
			`{"address":"http://0.0.0.0:9222","executable":"does-not-matter"}`,
		),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := (Adapter{}).Start(
				context.Background(),
				manifest.EngineSpec{Options: raw},
				orchestrator.EngineRuntime{},
			)
			if err == nil {
				t.Fatal("Start() error = nil")
			}
		})
	}
}

func TestFindExecutableReportsConfiguredBinary(t *testing.T) {
	t.Parallel()

	_, err := findExecutable("definitely-not-a-jangolova-browser")
	if err == nil || !strings.Contains(err.Error(), "find Chromium executable") {
		t.Fatalf("findExecutable() error = %v", err)
	}
}

func TestCallerEnvironmentSelectsExternalDisplay(t *testing.T) {
	t.Parallel()

	if !hostDisplayAvailable(orchestrator.EngineEnvironment{"DISPLAY": ":99"}) {
		t.Fatal("caller-supplied DISPLAY was not recognized")
	}
	if !hostDisplayAvailable(orchestrator.EngineEnvironment{
		"WAYLAND_DISPLAY": "wayland-1",
	}) {
		t.Fatal("caller-supplied WAYLAND_DISPLAY was not recognized")
	}
}

func TestCallerEnvironmentOverridesInheritedValue(t *testing.T) {
	t.Setenv("JANGOLOVA_ENVIRONMENT_TEST", "inherited")

	environment := engineEnvironment(orchestrator.EngineEnvironment{
		"JANGOLOVA_ENVIRONMENT_TEST": "caller",
	})
	var matches []string
	for _, item := range environment {
		if strings.HasPrefix(item, "JANGOLOVA_ENVIRONMENT_TEST=") {
			matches = append(matches, item)
		}
	}
	if len(matches) != 1 || matches[0] != "JANGOLOVA_ENVIRONMENT_TEST=caller" {
		t.Fatalf("environment override = %#v", matches)
	}
}
