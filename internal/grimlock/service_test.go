package grimlock

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/session"
	"jangolova/internal/bridge"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
	"jangolova/targetconn"
)

func testService(t *testing.T) *Service {
	t.Helper()
	registry, err := NewConnectorRegistry(&fakeConnector{protocol: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime(targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{Headers: map[string]string{"Authorization": "Bearer fixture-secret"}, ExpiresAt: time.Now().Add(time.Hour)}, nil
	}), registry)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(runtime, orchestrator.NewRegistry(), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(context.Background()) })
	return service
}

func testServiceWithStore(t *testing.T, dir string) *Service {
	t.Helper()
	registry, err := NewConnectorRegistry(&fakeConnector{protocol: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	engineReg := orchestrator.NewRegistry()
	if err := engineReg.RegisterEngine("persist-fixture", &persistFixtureAdapter{}); err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime(targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{Headers: map[string]string{"Authorization": "Bearer fixture-secret"}, ExpiresAt: time.Now().Add(time.Hour)}, nil
	}), registry)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(runtime, engineReg, "test-token", WithStoreDirectory(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(context.Background()) })
	return service
}

type persistFixtureAdapter struct{}

func (f *persistFixtureAdapter) Connect(ctx context.Context, spec manifest.EngineSpec, target orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	return &persistFixtureInstance{}, nil
}

type persistFixtureInstance struct{}

func (f *persistFixtureInstance) Disconnect(ctx context.Context) error { return nil }
func (f *persistFixtureInstance) Authorize(ctx context.Context, req orchestrator.AuthorizeRequest) (orchestrator.AuthorizeDecision, error) {
	return orchestrator.AuthorizeDecision{Authorized: true}, nil
}
func (f *persistFixtureInstance) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if method == bridge.MethodCapabilities {
		return json.RawMessage(`[{"name":"act","effect":"read","inputSchema":{}}]`), nil
	}
	return json.RawMessage("{}"), nil
}

func init() {
	var _ bridge.Caller = (*persistFixtureInstance)(nil)
	var _ orchestrator.EngineInstance = (*persistFixtureInstance)(nil)
}

func TestServiceHealthAndAuthorization(t *testing.T) {
	service := testService(t)
	handler := service.Routes()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/connectors", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "fixture") {
		t.Fatalf("connector discovery status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestServiceRejectsInvalidSessionBeforeConnecting(t *testing.T) {
	service := testService(t)
	request := `{"apiVersion":"agent.grimlock/v1alpha1","userId":"app","agent":{"sessionId":"bad_id","model":{"apiVersion":"agent.model/v1alpha1","profileId":"profile","protocol":"fixture","endpoint":"https://gateway.example","model":"test","credentialRef":"credential"}},"bindings":[]}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(request))
	httpRequest.Header.Set("Authorization", "Bearer test-token")
	recorder := httptest.NewRecorder()
	service.Routes().ServeHTTP(recorder, httpRequest)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var value ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(value.Message, "sessionId") {
		t.Fatalf("error = %#v", value)
	}
}

func TestGrimlockCursorRetentionValidation(t *testing.T) {
	if _, err := parseGrimlockCursor("later"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
	if _, err := parseGrimlockLimit("257"); err == nil {
		t.Fatal("oversized event limit accepted")
	}
}

func TestMCPStdioNegotiatesAndListsTools(t *testing.T) {
	service := testService(t)
	mcp, err := NewMCPServer(service)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	input := strings.NewReader("{" +
		`"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	if err := mcp.ServeStdio(context.Background(), input, &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("MCP response lines = %d, output = %s", len(lines), output.String())
	}
	var initialize map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &initialize); err != nil {
		t.Fatal(err)
	}
	if initialize["result"] == nil {
		t.Fatalf("initialize response = %#v", initialize)
	}
	if !strings.Contains(lines[1], "grimlock_session_create") {
		t.Fatalf("tools/list response does not contain session tool: %s", lines[1])
	}
}

func TestMCPHTTPNegotiatesTransportSession(t *testing.T) {
	service := testService(t)
	mcp, err := NewMCPServer(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := mcp.Routes()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Mcp-Session-Id") == "" {
		t.Fatalf("MCP initialize status = %d, headers = %#v", recorder.Code, recorder.Header())
	}
	request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Mcp-Session-Id", recorder.Header().Get("Mcp-Session-Id"))
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "grimlock_session_run") {
		t.Fatalf("MCP tools/list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestMCPHTTPRequiresGrimlockAuthorization(t *testing.T) {
	service := testService(t)
	mcp, err := NewMCPServer(service)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	mcp.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized MCP status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestACPStdioNegotiatesAndRequiresSessionConfiguration(t *testing.T) {
	service := testService(t)
	acp, err := NewACPServer(service)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	input := strings.NewReader("{" +
		`"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"userId":"app"}}` + "\n")
	if err := acp.ServeStdio(context.Background(), input, &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"protocolVersion":1`) {
		t.Fatalf("ACP output = %s", output.String())
	}
	if !strings.Contains(lines[1], "model apiVersion") {
		t.Fatalf("ACP session/new error = %s", lines[1])
	}
}

func TestSessionPersistenceReloadsAfterRestart(t *testing.T) {
	dir := t.TempDir()
	service := testServiceWithStore(t, dir)

	createPayload := `{"apiVersion":"agent.grimlock/v1alpha1","userId":"app","agent":{"sessionId":"persist-1","model":{"apiVersion":"agent.model/v1alpha1","profileId":"profile","protocol":"fixture","endpoint":"https://gateway.example","model":"test","credentialRef":"credential"}},"bindings":[{"interactionId":"bind-1","engine":{"adapter":"persist-fixture"},"target":{"targetId":"target-1","kind":"web","endpoints":[{"name":"primary","protocol":"http","url":"https://example.com"}]},"allowedCapabilities":[]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	service.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create session status = %d, body = %s", rec.Code, rec.Body.String())
	}

	runPayload := `{"text":"hello"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/persist-1/run", strings.NewReader(runPayload))
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	service.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run session status = %d, body = %s", rec.Code, rec.Body.String())
	}

	record, ok := service.lookupSession("persist-1")
	if !ok {
		t.Fatal("session not found after run")
	}
	event := &session.Event{
		Author:  "test",
		Actions: session.EventActions{},
	}
	envelope, err := record.appendEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	firstCursor := envelope.Cursor

	restarted, err := NewService(service.runtime, orchestrator.NewRegistry(), "test-token", WithStoreDirectory(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restarted.Close(context.Background()) }()

	req = httptest.NewRequest(http.MethodGet, "/v1/sessions/persist-1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	restarted.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reloaded session status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var view SessionView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Status != "ready" {
		t.Fatalf("reloaded session status = %s", view.Status)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/sessions/persist-1/events", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	restarted.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reloaded events status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var eventsResp EventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &eventsResp); err != nil {
		t.Fatal(err)
	}
	if eventsResp.Cursor != firstCursor {
		t.Fatalf("reloaded cursor = %s, want %s", eventsResp.Cursor, firstCursor)
	}
	if len(eventsResp.Events) != 1 {
		t.Fatalf("reloaded event count = %d, want 1", len(eventsResp.Events))
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/sessions/persist-1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	restarted.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete session status = %d", rec.Code)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "persist-1") {
			t.Fatalf("persisted file not removed: %s", entry.Name())
		}
	}
}
