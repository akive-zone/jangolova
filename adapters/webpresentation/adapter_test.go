package webpresentation

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	for _, required := range []string{"presentation.create", "presentation.replace", "presentation.write", "presentation.mount", "artifact.kind.web.entrypoint", "artifact.transport.http", "artifact.transport.target-file", "presentation.execute", "presentation.patch", "presentation.describe", "presentation.capture", "presentation.activate"} {
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

func TestPresentationPolicyDefaultsAndOverrides(t *testing.T) {
	defaults, err := decodeOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.resolved.MaxHTMLBytes != 1024*1024 || defaults.resolved.MaxTotalBytes != 1536*1024 {
		t.Fatalf("default policy = %#v", defaults.resolved)
	}
	if !defaults.resolved.actionAuthorized("presentation.execute") || !defaults.resolved.actionAuthorized("presentation.capture") || !defaults.resolved.actionAuthorized("presentation.mount") {
		t.Fatalf("default sensitive action authorization = %#v", defaults.resolved.AuthorizedActions)
	}

	configured, err := decodeOptions(json.RawMessage(`{
		"policy": {
			"maxHTMLBytes": 16,
			"maxTotalBytes": 32,
			"allowedSourceOrigins": ["https://PRESENTATION.example/"],
			"allowedAssetOrigins": ["self", "https://ASSETS.example", "self"],
			"allowedArtifactTransports": ["http", "target-file", "http"],
			"authorizedActions": ["presentation.capture"],
			"executeTimeoutMillis": 250,
			"captureTimeoutMillis": 500,
			"mountTimeoutMillis": 750
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if configured.resolved.MaxHTMLBytes != 16 || configured.resolved.MaxTotalBytes != 32 {
		t.Fatalf("configured policy = %#v", configured.resolved)
	}
	if got := configured.resolved.AllowedSourceOrigins; len(got) != 1 || got[0] != "https://presentation.example" {
		t.Fatalf("source origins = %v", got)
	}
	if got := configured.resolved.AllowedAssetOrigins; len(got) != 2 || got[1] != "https://assets.example" {
		t.Fatalf("asset origins = %v", got)
	}
	if got := configured.resolved.AllowedArtifactTransports; len(got) != 2 || got[1] != "target-file" {
		t.Fatalf("artifact transports = %v", got)
	}
	if configured.resolved.actionAuthorized("presentation.execute") || !configured.resolved.actionAuthorized("presentation.capture") {
		t.Fatalf("authorized actions = %v", configured.resolved.AuthorizedActions)
	}
	if configured.resolved.ExecuteTimeoutMillis != 250 || configured.resolved.CaptureTimeoutMillis != 500 || configured.resolved.MountTimeoutMillis != 750 {
		t.Fatalf("timeouts = %#v", configured.resolved)
	}
}

func TestPresentationPolicyRejectsDisallowedSourceAndOversizedArtifact(t *testing.T) {
	policy, err := resolvePolicy(policyConfig{
		MaxHTMLBytes:         4,
		MaxTotalBytes:        8,
		AllowedSourceOrigins: []string{"https://presentation.example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.validateSource("https://other.example/presentation"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("validateSource() error = %v", err)
	}
	params := json.RawMessage(`{"name":"presentation.write","input":{"expectedStateRevision":"0","html":"12345"}}`)
	if err := policy.validateCall("act", params); err == nil || !strings.Contains(err.Error(), "presentation HTML") {
		t.Fatalf("validateCall() error = %v", err)
	}
}

func TestPresentationPolicyRejectsInvalidConfiguration(t *testing.T) {
	for _, raw := range []string{
		`{"policy":{"maxHTMLBytes":-1}}`,
		`{"policy":{"allowedSourceOrigins":["https://example.com/path"]}}`,
		`{"policy":{"allowedAssetOrigins":["javascript:"]}}`,
		`{"policy":{"authorizedActions":["presentation.delete"]}}`,
		`{"policy":{"executeTimeoutMillis":-1}}`,
		`{"policy":{"captureTimeoutMillis":120001}}`,
		`{"policy":{"mountTimeoutMillis":-1}}`,
		`{"policy":{"allowedArtifactTransports":["xallet-volume"]}}`,
	} {
		if _, err := decodeOptions(json.RawMessage(raw)); err == nil {
			t.Fatalf("decodeOptions(%s) error = nil", raw)
		}
	}
}

func TestPresentationArtifactMountUsesProviderNeutralAllowedLocation(t *testing.T) {
	policy, err := resolvePolicy(policyConfig{
		AllowedSourceOrigins:      []string{"http://127.0.0.1:8082"},
		AllowedArtifactTransports: []string{"http"},
	})
	if err != nil {
		t.Fatal(err)
	}
	params := json.RawMessage(`{
		"name":"presentation.mount",
		"input":{
			"expectedStateRevision":"1",
			"artifact":{
				"apiVersion":"interaction.presentation/v1alpha1",
				"artifactId":"experience-42",
				"revision":"sha256:abc123",
				"kind":"web.entrypoint",
				"locations":[
					{"transport":"provider-handle","uri":"artifact://experience-42"},
					{"transport":"http","uri":"http://127.0.0.1:8082/"}
				]
			}
		}
	}`)
	if err := policy.validateCall("act", params); err != nil {
		t.Fatalf("validateCall() error = %v", err)
	}
}

func TestPresentationArtifactMountRejectsDisallowedLocation(t *testing.T) {
	policy := defaultPresentationPolicy()
	params := json.RawMessage(`{
		"name":"presentation.mount",
		"input":{
			"expectedStateRevision":"0",
			"artifact":{
				"apiVersion":"interaction.presentation/v1alpha1",
				"artifactId":"experience-42",
				"revision":"v1",
				"kind":"web.entrypoint",
				"locations":[{"transport":"target-file","uri":"file:///presentations/experience/index.html"}]
			}
		}
	}`)
	if err := policy.validateCall("act", params); err == nil || !strings.Contains(err.Error(), "allowed transport") {
		t.Fatalf("validateCall() error = %v", err)
	}
}

func TestPresentationArtifactSchemaIsProviderNeutral(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "protocol", "presentation", "v1", "artifact.schema.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(contents) {
		t.Fatal("artifact schema is invalid JSON")
	}
	lower := strings.ToLower(string(contents))
	for _, forbidden := range []string{"jangolova", "xallet"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("provider-neutral artifact schema contains %q", forbidden)
		}
	}
}

func TestSensitivePresentationActionsAreAuthorizedByPolicy(t *testing.T) {
	policy, err := resolvePolicy(policyConfig{AuthorizedActions: []string{"presentation.capture"}})
	if err != nil {
		t.Fatal(err)
	}
	params := json.RawMessage(`{"name":"presentation.execute","input":{"code":"return 1"}}`)
	if err := policy.validateCall("act", params); err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("validateCall() error = %v", err)
	}
}

func TestSensitivePresentationActionsEmitAuditEvents(t *testing.T) {
	instance := testInstance(defaultPresentationPolicy())
	instance.responses <- rpcResponse{ID: 1, Result: json.RawMessage(`{"ok":true}`)}
	result, err := instance.Call(context.Background(), "act", json.RawMessage(`{"name":"presentation.execute","input":{"code":"return 1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"ok":true}` {
		t.Fatalf("result = %s", result)
	}
	assertAuditEvent(t, instance.events, "presentation.execute.requested")
	assertAuditEvent(t, instance.events, "presentation.execute.succeeded")
}

func TestDeniedSensitivePresentationActionEmitsAuditEvent(t *testing.T) {
	policy := defaultPresentationPolicy()
	policy.AuthorizedActions = []string{"presentation.capture"}
	instance := testInstance(policy)
	_, err := instance.Call(context.Background(), "act", json.RawMessage(`{"name":"presentation.execute","input":{"code":"return 1"}}`))
	if err == nil {
		t.Fatal("Call() error = nil")
	}
	assertAuditEvent(t, instance.events, "presentation.execute.requested")
	assertAuditEvent(t, instance.events, "presentation.execute.denied")
	if instance.stdin.(*testWriteCloser).Len() != 0 {
		t.Fatal("denied action was sent to worker")
	}
}

func TestSensitivePresentationActionTimeoutCancelsRequest(t *testing.T) {
	policy := defaultPresentationPolicy()
	policy.ExecuteTimeoutMillis = 1
	instance := testInstance(policy)
	_, err := instance.Call(context.Background(), "act", json.RawMessage(`{"name":"presentation.execute","input":{"code":"while(true){}"}}`))
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("Call() error = %v", err)
	}
	assertAuditEvent(t, instance.events, "presentation.execute.requested")
	assertAuditEvent(t, instance.events, "presentation.execute.cancelled")
}

type testWriteCloser struct{ bytes.Buffer }

func (w *testWriteCloser) Close() error { return nil }

func testInstance(policy presentationPolicy) *instance {
	return &instance{
		stdin:     &testWriteCloser{},
		responses: make(chan rpcResponse, 1),
		done:      make(chan error, 1),
		events:    make(chan orchestrator.EngineEvent, 8),
		stderr:    &lockedBuffer{},
		policy:    policy,
	}
}

func assertAuditEvent(t *testing.T, events <-chan orchestrator.EngineEvent, eventType string) {
	t.Helper()
	select {
	case event := <-events:
		if event.Type != eventType || !event.OccurredAt.After(time.Time{}) {
			t.Fatalf("audit event = %#v, want %q", event, eventType)
		}
	default:
		t.Fatalf("missing audit event %q", eventType)
	}
}
