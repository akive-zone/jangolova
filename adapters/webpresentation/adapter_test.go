package webpresentation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

func TestAdapterRequiresCallerOwnedCDPBrowser(t *testing.T) {
	t.Parallel()
	adapter := Adapter{}
	for name, target := range map[string]orchestrator.EngineTarget{
		"wrong kind":  {Kind: "native"},
		"missing cdp": {Kind: "browser"},
		"invalid cdp": {Kind: "browser", Endpoints: []orchestrator.TargetEndpoint{{Name: "cdp", Protocol: "cdp", URL: "file:///browser"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := adapter.Connect(context.Background(), manifest.EngineSpec{}, target); err == nil {
				t.Fatal("Connect() error = nil")
			}
		})
	}
}

func TestDecodeOptionsRejectsRuntimeLaunchFields(t *testing.T) {
	t.Parallel()
	if _, err := decodeOptions(json.RawMessage(`{"browserExecutable":"chromium"}`)); err == nil {
		t.Fatal("runtime-launch option was accepted")
	}
}

func TestInspectionFindsRepositoryWorker(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	worker := filepath.Join(filepath.Dir(file), "..", "..", "scripts", "presentation-worker.mjs")
	t.Setenv("JANGOLOVA_PRESENTATION_WORKER", worker)
	inspection := (Adapter{}).InspectEngine(context.Background())
	if !inspection.Available {
		t.Fatalf("InspectEngine() = %#v", inspection)
	}
}

func TestCapabilityNamesExposePresentationContract(t *testing.T) {
	values := capabilityNames()
	for _, required := range []string{"presentation.create", "presentation.replace", "presentation.write", "presentation.execute", "presentation.patch", "presentation.describe", "presentation.capture", "presentation.activate"} {
		found := false
		for _, value := range values {
			if value == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("capabilityNames() missing %q: %v", required, values)
		}
	}
}

func TestConnectedCapabilitiesRetainCommonInteractionMethods(t *testing.T) {
	values := stableStrings(append(capabilityNames(), "presentation.write", "custom.presentation.action"))
	for _, required := range []string{"describe", "act", "events", "presentation.write", "custom.presentation.action"} {
		found := false
		for _, value := range values {
			if value == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("connected capabilities missing %q: %v", required, values)
		}
	}
}
