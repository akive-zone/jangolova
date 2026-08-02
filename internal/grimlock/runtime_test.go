package grimlock

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"

	"jangolova/internal/bridge"
	"jangolova/targetconn"
)

type fakeLLM struct{ name string }

func (f fakeLLM) Name() string { return f.name }
func (fakeLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(func(*model.LLMResponse, error) bool) {}
}

type fakeConnector struct {
	protocol string
	err      error

	mu      sync.Mutex
	profile ModelProfile
	token   string
}

func (f *fakeConnector) Protocol() string { return f.protocol }
func (f *fakeConnector) Connect(_ context.Context, connection ModelConnection) (ConnectedModel, error) {
	token, tokenErr := connection.BearerToken()
	if tokenErr != nil {
		return ConnectedModel{}, tokenErr
	}
	f.mu.Lock()
	f.profile, f.token = connection.Profile(), token
	f.mu.Unlock()
	if f.err != nil {
		return ConnectedModel{}, f.err
	}
	return ConnectedModel{LLM: fakeLLM{name: connection.Profile().Model}}, nil
}

func testAgentSpec() AgentSpec {
	return AgentSpec{SessionID: "application-one", Model: ModelProfile{
		APIVersion: ModelAPIVersion, ProfileID: "application-model",
		Protocol: "fixture", Endpoint: "https://gateway.example/v1",
		Model: "approved-model", CredentialRef: "application-credential",
	}}
}

func TestRuntimeCreatesADKAgentFromCallerSuppliedModel(t *testing.T) {
	connector := &fakeConnector{protocol: "fixture"}
	registry, err := NewConnectorRegistry(connector)
	if err != nil {
		t.Fatal(err)
	}
	releases := 0
	resolver := targetconn.ResolverFunc(func(_ context.Context, request targetconn.Request) (targetconn.Material, error) {
		if request.Reference != "application-credential" || request.Audience != "model" || request.Protocol != "fixture" {
			t.Fatalf("resolver request = %#v", request)
		}
		return targetconn.Material{
			Headers:   map[string]string{"Authorization": "Bearer caller-secret"},
			ExpiresAt: time.Now().Add(time.Hour),
			Release:   func(context.Context) error { releases++; return nil },
		}, nil
	})
	runtime, err := NewRuntime(resolver, registry)
	if err != nil {
		t.Fatal(err)
	}
	session, err := runtime.CreateAgent(context.Background(), testAgentSpec(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "application-one" || session.Agent == nil || session.Profile.Model != "approved-model" {
		t.Fatalf("session = %#v", session)
	}
	connector.mu.Lock()
	profile, token := connector.profile, connector.token
	connector.mu.Unlock()
	if profile.Endpoint != "https://gateway.example/v1" || token != "caller-secret" {
		t.Fatalf("connector profile = %#v, token = %q", profile, token)
	}
	if err := session.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(context.Background()); err != nil || releases != 1 {
		t.Fatalf("second Close() = %v, releases = %d", err, releases)
	}
}

func TestRuntimeRedactsModelCredentialFromConnectorFailure(t *testing.T) {
	connector := &fakeConnector{protocol: "fixture", err: errors.New("gateway rejected caller-secret")}
	registry, err := NewConnectorRegistry(connector)
	if err != nil {
		t.Fatal(err)
	}
	resolver := targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{
			Headers:   map[string]string{"Authorization": "Bearer caller-secret"},
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	})
	runtime, err := NewRuntime(resolver, registry)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.CreateAgent(context.Background(), testAgentSpec(), nil)
	if err == nil || strings.Contains(err.Error(), "caller-secret") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("CreateAgent() error = %v", err)
	}
}

func TestConnectorRegistryRejectsDuplicateProtocols(t *testing.T) {
	t.Parallel()
	_, err := NewConnectorRegistry(&fakeConnector{protocol: "fixture"}, &fakeConnector{protocol: "fixture"})
	if err == nil {
		t.Fatal("duplicate connector error = nil")
	}
}

func TestConnectorRegistryZeroValueCanRegister(t *testing.T) {
	t.Parallel()
	var registry ConnectorRegistry
	if err := registry.Register(&fakeConnector{protocol: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if connector, ok := registry.Connector("fixture"); !ok || connector == nil {
		t.Fatal("registered connector was not found")
	}
}

func TestRuntimeCreatesAgentWithNamespacedInteractionTools(t *testing.T) {
	connector := &fakeConnector{protocol: "fixture"}
	registry, err := NewConnectorRegistry(connector)
	if err != nil {
		t.Fatal(err)
	}
	resolver := targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{
			Headers:   map[string]string{"Authorization": "Bearer caller-secret"},
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	})
	runtime, err := NewRuntime(resolver, registry)
	if err != nil {
		t.Fatal(err)
	}
	first := &toolFixtureCaller{capabilities: []bridge.Capability{{
		Name: "scene.observe", Effect: bridge.EffectRead, InputSchema: []byte(`{"type":"object"}`),
	}}}
	second := &toolFixtureCaller{capabilities: []bridge.Capability{{
		Name: "scene.observe", Effect: bridge.EffectRead, InputSchema: []byte(`{"type":"object"}`),
	}}}
	session, err := runtime.CreateInteractionAgent(t.Context(), testAgentSpec(), []InteractionBinding{
		{InteractionID: "browser-one", Caller: first},
		{InteractionID: "unity-one", Caller: second},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close(context.Background())
	if session.Agent == nil {
		t.Fatal("agent = nil")
	}
	if len(first.calls) != 1 || len(second.calls) != 1 {
		t.Fatalf("capability discovery calls = %d, %d", len(first.calls), len(second.calls))
	}
}
