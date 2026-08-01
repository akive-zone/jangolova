package pacman

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"jangolova/internal/bridge"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
	protocol "jangolova/internal/pacman"
)

func TestAdapterConformsAndPreservesCallerOwnedTarget(t *testing.T) {
	fixture := newFixtureProvider(t, "Bearer fixture-token", protocol.ProtocolVersion)
	defer fixture.Close()
	target := orchestrator.EngineTarget{Kind: "unity", Endpoints: []orchestrator.TargetEndpoint{{
		Name: "semantic", Protocol: "pacman-ws", URL: fixture.wsURL(),
		Connection: &orchestrator.EndpointConnection{Headers: map[string]string{"Authorization": "Bearer fixture-token"}, ExpiresAt: time.Now().Add(time.Minute)},
	}}}
	connected, err := (Adapter{}).Connect(context.Background(), manifest.EngineSpec{}, target)
	if err != nil {
		t.Fatal(err)
	}
	caller := connected.(bridge.Caller)
	result, err := caller.Call(context.Background(), protocol.MethodAct, json.RawMessage(`{"name":"object.visibility.set","targetId":"object:hero","input":{"visible":false}}`))
	if err != nil || !strings.Contains(string(result), `"ok":true`) {
		t.Fatalf("act result=%s error=%v", result, err)
	}
	if _, err := caller.Call(context.Background(), protocol.MethodAct, json.RawMessage(`{"name":"object.destroy","targetId":"object:hero","input":{}}`)); err == nil || !strings.Contains(err.Error(), "not advertised") {
		t.Fatalf("unadvertised action error = %v", err)
	}
	if err := connected.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for fixture.closed.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if fixture.closed.Load() != 1 {
		t.Fatalf("fixture connections closed = %d", fixture.closed.Load())
	}
	response, err := http.Get(fixture.URL + "/healthz")
	if err != nil {
		t.Fatalf("caller-owned target stopped after detach: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("target health status = %d", response.StatusCode)
	}
}

func TestAdapterRejectsUnauthenticatedAndIncompatibleTargets(t *testing.T) {
	t.Run("authentication", func(t *testing.T) {
		fixture := newFixtureProvider(t, "Bearer expected", protocol.ProtocolVersion)
		defer fixture.Close()
		_, err := (Adapter{}).Connect(context.Background(), manifest.EngineSpec{}, orchestrator.EngineTarget{Kind: "unity", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "pacman-ws", URL: fixture.wsURL()}}})
		if err == nil || strings.Contains(err.Error(), "expected") {
			t.Fatalf("Connect() error = %v", err)
		}
	})
	t.Run("protocol", func(t *testing.T) {
		fixture := newFixtureProvider(t, "", "jangolova.pacman/v9")
		defer fixture.Close()
		_, err := (Adapter{}).Connect(context.Background(), manifest.EngineSpec{}, orchestrator.EngineTarget{Kind: "unreal", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "pacman-ws", URL: fixture.wsURL()}}})
		if err == nil || !strings.Contains(err.Error(), "incompatible") {
			t.Fatalf("Connect() error = %v", err)
		}
	})
}

func TestInspectionAdvertisesAutomaticSelectionProtocol(t *testing.T) {
	inspection := (Adapter{}).InspectEngine(context.Background())
	if !inspection.Available || !contains(inspection.Capabilities, "target.pacman-ws") || !contains(inspection.Capabilities, "health") {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestAdapterAcceptsNonWebSocketTransportConnector(t *testing.T) {
	transport := &memoryTransport{fixture: newFixtureResponses(protocol.ProtocolVersion)}
	adapter := Adapter{Connector: memoryConnector{transport: transport}}
	inspection := adapter.InspectEngine(context.Background())
	if !contains(inspection.Capabilities, "target.pacman-memory") {
		t.Fatalf("inspection = %#v", inspection)
	}
	connected, err := adapter.Connect(context.Background(), manifest.EngineSpec{}, orchestrator.EngineTarget{
		Kind: "unity", Endpoints: []orchestrator.TargetEndpoint{{Protocol: "pacman-memory", URL: "memory://fixture"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := connected.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !transport.closed.Load() {
		t.Fatal("custom Pacman transport was not closed")
	}
}

func TestWebSocketBindingReconnectsAndAcknowledgesRenewedCredential(t *testing.T) {
	fixture := newFixtureProvider(t, "Bearer initial-token", protocol.ProtocolVersion)
	defer fixture.Close()
	connection := &orchestrator.EndpointConnection{}
	connection.ReplaceCredential(map[string]string{"Authorization": "Bearer initial-token"}, time.Now().Add(time.Minute))
	connected, err := (Adapter{}).Connect(context.Background(), manifest.EngineSpec{}, orchestrator.EngineTarget{
		Kind: "unity", Endpoints: []orchestrator.TargetEndpoint{{
			Name: "semantic", Protocol: "pacman-ws", URL: fixture.wsURL(), Connection: connection,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer connected.Disconnect(context.Background())
	fixture.setAuth("Bearer renewed-token")
	revision := connection.ReplaceCredential(map[string]string{"Authorization": "Bearer renewed-token"}, time.Now().Add(2*time.Minute))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, acknowledged := connection.Acknowledgements()
		if acknowledged >= revision && fixture.accepted.Load() >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, acknowledged := connection.Acknowledgements()
	t.Fatalf("renewed revision %d was not applied: acknowledged=%d accepted=%d", revision, acknowledged, fixture.accepted.Load())
}

type fixtureProvider struct {
	*httptest.Server
	auth     atomic.Value
	protocol string
	closed   atomic.Int32
	accepted atomic.Int32
}

func newFixtureProvider(t *testing.T, auth, version string) *fixtureProvider {
	t.Helper()
	fixture := &fixtureProvider{protocol: version}
	fixture.auth.Store(auth)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/pacman", fixture.accept)
	fixture.Server = httptest.NewServer(mux)
	return fixture
}

func (f *fixtureProvider) wsURL() string { return "ws" + strings.TrimPrefix(f.URL, "http") + "/pacman" }

func (f *fixtureProvider) setAuth(value string) { f.auth.Store(value) }

func (f *fixtureProvider) accept(w http.ResponseWriter, r *http.Request) {
	auth, _ := f.auth.Load().(string)
	if auth != "" && r.Header.Get("Authorization") != auth {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	f.accepted.Add(1)
	defer func() { connection.Close(); f.closed.Add(1) }()
	for {
		var request request
		if connection.ReadJSON(&request) != nil {
			return
		}
		result := f.call(request.Method)
		if connection.WriteJSON(response{ID: request.ID, Result: result}) != nil {
			return
		}
	}
}

func (f *fixtureProvider) call(method string) json.RawMessage {
	now, _ := json.Marshal(time.Now().UTC())
	switch method {
	case protocol.MethodHello:
		return json.RawMessage(`{"protocolVersion":"` + f.protocol + `","implementation":{"engine":"unity","name":"pacman-fixture","version":"1"},"features":["events.cursor"]}`)
	case protocol.MethodCapabilities:
		return json.RawMessage(`[{"name":"object.visibility.set","effect":"write","targetKinds":["object"],"inputSchema":{"type":"object"}}]`)
	case protocol.MethodDescribe:
		return json.RawMessage(`{"revision":"1","resources":[{"id":"scene:main","kind":"scene"},{"id":"object:hero","kind":"object"}]}`)
	case protocol.MethodEvents:
		return json.RawMessage(`{"events":[],"cursor":"0"}`)
	case protocol.MethodHealth:
		return json.RawMessage(`{"status":"ready","observedAt":` + string(now) + `}`)
	case protocol.MethodAct:
		return json.RawMessage(`{"ok":true}`)
	default:
		return json.RawMessage(`{}`)
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

type memoryConnector struct{ transport Transport }

func (memoryConnector) Protocol() string { return "pacman-memory" }
func (m memoryConnector) Connect(context.Context, orchestrator.TargetEndpoint) (Transport, error) {
	return m.transport, nil
}

type memoryTransport struct {
	fixture map[string]json.RawMessage
	closed  atomic.Bool
}

func (m *memoryTransport) Call(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
	return m.fixture[method], nil
}
func (m *memoryTransport) Close() error { m.closed.Store(true); return nil }

func newFixtureResponses(version string) map[string]json.RawMessage {
	now, _ := json.Marshal(time.Now().UTC())
	return map[string]json.RawMessage{
		protocol.MethodHello:        json.RawMessage(`{"protocolVersion":"` + version + `","implementation":{"engine":"unity","name":"memory-fixture"}}`),
		protocol.MethodCapabilities: json.RawMessage(`[{"name":"resource.describe","effect":"read","targetKinds":["object"],"inputSchema":{"type":"object"}}]`),
		protocol.MethodDescribe:     json.RawMessage(`{"revision":"1","resources":[{"id":"object:hero","kind":"object"}]}`),
		protocol.MethodEvents:       json.RawMessage(`{"events":[],"cursor":"0"}`),
		protocol.MethodHealth:       json.RawMessage(`{"status":"ready","observedAt":` + string(now) + `}`),
	}
}
