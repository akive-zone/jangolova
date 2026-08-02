package cymonkey

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"jangolova/internal/bridge"
	contract "jangolova/internal/cymonkey"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

const fixtureExtensionID = "abcdefghijklmnopabcdefghijklmnop"

func TestAdapterDefaultsToNoInstallCDPAndDisconnects(t *testing.T) {
	worker, err := filepath.Abs("../../tests/cymonkey-worker-fixture.mjs")
	if err != nil {
		t.Fatal(err)
	}
	options, _ := json.Marshal(map[string]string{"workerPath": worker})
	connected, err := (Adapter{}).Connect(context.Background(), manifest.EngineSpec{Options: options}, orchestrator.EngineTarget{
		Kind: "browser",
		Endpoints: []orchestrator.TargetEndpoint{{
			Name: "control", Protocol: "cdp", URL: "wss://browser.remote.example/devtools/browser/42",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	healthCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	health := connected.(orchestrator.EngineHealthProvider).EngineHealth(healthCtx)
	cancel()
	if health.Status != orchestrator.EngineHealthHealthy {
		t.Fatalf("EngineHealth() = %#v", health)
	}
	if !contains(connected.(orchestrator.EngineCapabilityProvider).EngineCapabilities(), "script.register") {
		t.Fatal("worker capability was not retained")
	}
	if err := connected.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterRequiresCallerOwnedCompatibleBrowserTarget(t *testing.T) {
	t.Parallel()
	adapter := Adapter{}
	for name, fixture := range map[string]struct {
		spec   manifest.EngineSpec
		target orchestrator.EngineTarget
	}{
		"wrong kind":  {target: orchestrator.EngineTarget{Kind: "native"}},
		"missing all": {target: orchestrator.EngineTarget{Kind: "browser"}},
		"invalid cdp": {target: orchestrator.EngineTarget{Kind: "browser", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "cdp", URL: "file:///browser"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := adapter.Connect(context.Background(), fixture.spec, fixture.target); err == nil {
				t.Fatal("Connect() error = nil")
			}
		})
	}
}

func TestDecodeOptionsRejectsUnknownAndInvalidValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		`{"extensionId":"abcdefghijklmnopabcdefghijklmnop","browserExecutable":"chromium"}`,
		`{"extension":{"mode":"required"}}`,
		`{"extension":{"id":"not-an-extension"}}`,
		`{"backend":"webdriver"}`,
	} {
		if _, err := decodeOptions(json.RawMessage(value)); err == nil {
			t.Fatalf("decodeOptions(%s) error = nil", value)
		}
	}
}

func TestDecodeOptionsDefaultsToAutoAndAcceptsNoExtension(t *testing.T) {
	config, err := decodeOptions(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if config.Profile != "" || config.Backend != "auto" || config.Extension.Mode != extensionAuto || config.Extension.ID != "" {
		t.Fatalf("decodeOptions() = %#v", config)
	}
}

func TestRuntimeProfileIsInferredFromCallerOwnedTarget(t *testing.T) {
	for name, fixture := range map[string]struct {
		requested contract.Profile
		target    orchestrator.EngineTarget
		want      contract.Profile
	}{
		"web":          {target: orchestrator.EngineTarget{Kind: "browser"}, want: contract.ProfileWeb},
		"macos":        {target: orchestrator.EngineTarget{Kind: "macos-application"}, want: contract.ProfileMacOS},
		"explicit web": {requested: contract.ProfileWeb, target: orchestrator.EngineTarget{Kind: "browser"}, want: contract.ProfileWeb},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := resolveProfile(fixture.requested, fixture.target)
			if err != nil || got != fixture.want {
				t.Fatalf("resolveProfile() = %q, %v", got, err)
			}
		})
	}
}

func TestMacOSProfileReturnsCallerOwnedNativeHelperLaunchMaterial(t *testing.T) {
	connected, err := (Adapter{}).Connect(context.Background(), manifest.EngineSpec{Options: json.RawMessage(`{"profile":"macos"}`)}, orchestrator.EngineTarget{Kind: "macos-application"})
	if err != nil {
		t.Fatal(err)
	}
	defer connected.Disconnect(context.Background())
	launch, ok := connected.(orchestrator.EngineCallerLaunchProvider)
	if !ok || !strings.HasPrefix(launch.EngineCallerLaunch().Environment["JANGOLOVA_CYMONKEY_CONTROL_URL"], "ws://127.0.0.1:") {
		t.Fatalf("caller launch = %#v", launch)
	}
}

func TestBackendSelectionPrefersCDPThenBiDiThenSafariMCP(t *testing.T) {
	for name, fixture := range map[string]struct {
		target orchestrator.EngineTarget
		want   BackendName
	}{
		"cdp":    {target: orchestrator.EngineTarget{Kind: "browser", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "webdriver-bidi"}, {Protocol: "cdp"}}}, want: BackendCDP},
		"bidi":   {target: orchestrator.EngineTarget{Kind: "browser", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "webdriver-bidi"}}}, want: BackendBiDi},
		"safari": {target: orchestrator.EngineTarget{Kind: "browser", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "mcp-streamable-http"}}}, want: BackendSafariMCP},
	} {
		t.Run(name, func(t *testing.T) {
			backend, err := selectBackend("auto", fixture.target)
			if err != nil || backend.Name() != fixture.want {
				t.Fatalf("selectBackend() = %v, %v", backend, err)
			}
		})
	}
}

func TestSafariMapperDoesNotInferAugmentationFromGenericInteractionTools(t *testing.T) {
	discovered := []bridge.Capability{
		{Name: "mcp.tool.click", Effect: bridge.EffectWrite, InputSchema: objectSchema("selector")},
		{Name: "mcp.tool.screenshot", Effect: bridge.EffectRead, InputSchema: objectSchema()},
		{Name: "browser.evaluate", Effect: bridge.EffectExternal, InputSchema: objectSchema("expression")},
		{Name: "mcp.tool.add_preload_script", Effect: bridge.EffectExternal, InputSchema: objectSchema("source")},
	}
	_, capabilities := mapSafariCapabilities(discovered, nil)
	if !contains(capabilityNamesFromDescriptors(capabilities), "script.execute") || !contains(capabilityNamesFromDescriptors(capabilities), "script.register") {
		t.Fatalf("mapped capabilities = %#v", capabilities)
	}
	if contains(capabilityNamesFromDescriptors(capabilities), "augmentation.install") {
		t.Fatalf("generic tools inferred augmentation support: %#v", capabilities)
	}
}

func TestInspectionFindsRepositoryWorker(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	worker := filepath.Join(filepath.Dir(file), "..", "..", "scripts", "cymonkey-worker.mjs")
	t.Setenv("JANGOLOVA_CYMONKEY_WORKER", worker)
	inspection := (Adapter{}).InspectEngine(context.Background())
	if !inspection.Available || !contains(inspection.Capabilities, "script.register") {
		t.Fatalf("InspectEngine() = %#v", inspection)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
