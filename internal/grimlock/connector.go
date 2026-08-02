package grimlock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"

	"jangolova/internal/orchestrator"
	"jangolova/targetconn"
)

const OpenAICompatibleProtocol = "openai-compatible"

// ModelConnection gives a connector access to one prepared model endpoint.
// Secret material remains inside the connection and its isolated HTTP client.
type ModelConnection struct {
	profile  ModelProfile
	endpoint orchestrator.TargetEndpoint
}

func (c ModelConnection) Profile() ModelProfile { return cloneModelProfile(c.profile) }

func (c ModelConnection) HTTPClient(timeout time.Duration) (*http.Client, error) {
	return targetconn.HTTPClient(c.endpoint, timeout)
}

// BearerToken is available only to model connectors that require an API key
// constructor in addition to the rotating authenticated HTTP transport.
func (c ModelConnection) BearerToken() (string, error) {
	value := targetconn.Headers(c.endpoint)["Authorization"]
	prefix := "Bearer "
	if len(value) <= len(prefix) || !strings.EqualFold(value[:len(prefix)], prefix) {
		return "", errors.New("model credential must provide an Authorization Bearer header")
	}
	return strings.TrimSpace(value[len(prefix):]), nil
}

// ModelConnector turns one prepared caller-supplied model profile into ADK's
// model-neutral LLM interface.
type ModelConnector interface {
	Protocol() string
	Connect(context.Context, ModelConnection) (ConnectedModel, error)
}

// ConnectedModel pairs ADK's model interface with connector-owned cleanup.
type ConnectedModel struct {
	LLM   model.LLM
	Close func()
}

type ConnectorRegistry struct {
	mu         sync.RWMutex
	connectors map[string]ModelConnector
}

func NewConnectorRegistry(values ...ModelConnector) (*ConnectorRegistry, error) {
	registry := &ConnectorRegistry{connectors: make(map[string]ModelConnector)}
	for _, connector := range values {
		if err := registry.Register(connector); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *ConnectorRegistry) Register(connector ModelConnector) error {
	if connector == nil {
		return errors.New("Grimlock model connector is required")
	}
	protocol := strings.TrimSpace(connector.Protocol())
	if !protocolPattern.MatchString(protocol) {
		return errors.New("Grimlock model connector protocol is invalid")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.connectors == nil {
		r.connectors = make(map[string]ModelConnector)
	}
	if _, exists := r.connectors[protocol]; exists {
		return fmt.Errorf("Grimlock model connector %q is already registered", protocol)
	}
	r.connectors[protocol] = connector
	return nil
}

func (r *ConnectorRegistry) Connector(protocol string) (ModelConnector, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	connector, ok := r.connectors[protocol]
	return connector, ok
}

func (r *ConnectorRegistry) Protocols() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	values := make([]string, 0, len(r.connectors))
	for protocol := range r.connectors {
		values = append(values, protocol)
	}
	sort.Strings(values)
	return values
}

// OpenAICompatibleConnector supports caller-owned gateways implementing the
// OpenAI Responses API surface accepted by ADK's openaimodel package.
type OpenAICompatibleConnector struct {
	Timeout time.Duration
}

func (OpenAICompatibleConnector) Protocol() string { return OpenAICompatibleProtocol }

func (c OpenAICompatibleConnector) Connect(ctx context.Context, connection ModelConnection) (ConnectedModel, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	client, err := connection.HTTPClient(timeout)
	if err != nil {
		return ConnectedModel{}, err
	}
	token, err := connection.BearerToken()
	if err != nil {
		client.CloseIdleConnections()
		return ConnectedModel{}, err
	}
	profile := connection.Profile()
	llm, err := openaimodel.NewModel(ctx, profile.Model, &openaimodel.ClientConfig{
		APIKey: token, BaseURL: profile.Endpoint, HTTPClient: client,
	})
	if err != nil {
		client.CloseIdleConnections()
		return ConnectedModel{}, err
	}
	return ConnectedModel{LLM: llm, Close: client.CloseIdleConnections}, nil
}
