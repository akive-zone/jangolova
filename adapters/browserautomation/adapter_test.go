package browserautomation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

func TestAdapterPassesResolvedHeadersToRemoteCDPWorker(t *testing.T) {
	worker, err := filepath.Abs("../../tests/connection-material-worker.mjs")
	if err != nil {
		t.Fatal(err)
	}
	options, _ := json.Marshal(map[string]string{"workerPath": worker})
	instance, err := Playwright().Connect(context.Background(), manifest.EngineSpec{Options: options}, orchestrator.EngineTarget{
		Kind: "browser",
		Endpoints: []orchestrator.TargetEndpoint{{
			Name: "control", Protocol: "cdp", URL: "wss://browser.remote.example/devtools/browser/42",
			Connection: &orchestrator.EndpointConnection{
				Headers:   map[string]string{"Authorization": "Bearer fixture-secret"},
				ExpiresAt: time.Now().Add(time.Minute),
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterRequiresCallerOwnedBrowserTarget(t *testing.T) {
	t.Parallel()
	adapter := Playwright()
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

func TestDecodeOptionsRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	if _, err := decodeOptions(json.RawMessage(`{"browserExecutable":"chromium"}`)); err == nil {
		t.Fatal("target-launch option was accepted")
	}
}

func TestPuppeteerAcceptsBiDiWhilePlaywrightRequiresCDP(t *testing.T) {
	t.Parallel()
	target := orchestrator.EngineTarget{
		Kind: "browser",
		Endpoints: []orchestrator.TargetEndpoint{{
			Name: "bidi", Protocol: "webdriver-bidi", URL: "ws://127.0.0.1:9222/session",
		}},
	}
	endpoint, protocol, ok := Puppeteer().targetEndpoint(target)
	if !ok || protocol != "webdriver-bidi" || endpoint.URL == "" {
		t.Fatalf("Puppeteer targetEndpoint() = %#v, %q, %v", endpoint, protocol, ok)
	}
	if _, _, ok := Playwright().targetEndpoint(target); ok {
		t.Fatal("Playwright accepted a WebDriver BiDi-only target")
	}
}

func TestInspectionFindsRepositoryWorker(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	worker := filepath.Join(filepath.Dir(file), "..", "..", "scripts", "browser-worker.mjs")
	t.Setenv("JANGOLOVA_BROWSER_WORKER", worker)
	inspection := Puppeteer().InspectEngine(context.Background())
	if !inspection.Available {
		t.Fatalf("InspectEngine() = %#v", inspection)
	}
}
