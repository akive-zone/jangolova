package grimlock

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/tool"

	"jangolova/internal/bridge"
	"jangolova/internal/orchestrator"
	"jangolova/targetconn"
)

const (
	DefaultAgentName   = "grimlock"
	DefaultDescription = "Operates Jangolova interaction instances through bounded, verifiable capabilities."
	DefaultInstruction = `You are Grimlock, Jangolova's model-powered agent subsystem.
Use only the capabilities provided as tools.
Observe before acting whenever current state matters.
Treat effect classifications, budgets, and approval requirements as hard boundaries.
Report returned evidence and never claim an action succeeded without it.`
)

var sessionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type AgentSpec struct {
	SessionID   string       `json:"sessionId"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Instruction string       `json:"instruction,omitempty"`
	Model       ModelProfile `json:"model"`
}

func (s AgentSpec) Validate() error {
	if !sessionIDPattern.MatchString(s.SessionID) {
		return errors.New("Grimlock sessionId must be a lowercase DNS-style name")
	}
	if s.Name != "" && !profileIDPattern.MatchString(s.Name) {
		return errors.New("Grimlock agent name is invalid")
	}
	if len(s.Description) > 1024 || len(s.Instruction) > 64*1024 {
		return errors.New("Grimlock agent description or instruction is too large")
	}
	return s.Model.Validate()
}

type Runtime struct {
	resolver   targetconn.Resolver
	connectors *ConnectorRegistry
}

func NewRuntime(resolver targetconn.Resolver, connectors *ConnectorRegistry) (*Runtime, error) {
	if resolver == nil {
		return nil, errors.New("Grimlock model connection resolver is required")
	}
	if connectors == nil || len(connectors.Protocols()) == 0 {
		return nil, errors.New("Grimlock requires at least one model connector")
	}
	return &Runtime{resolver: resolver, connectors: connectors}, nil
}

func NewDefaultRuntime(resolver targetconn.Resolver) (*Runtime, error) {
	registry, err := NewConnectorRegistry(OpenAICompatibleConnector{})
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		resolver = targetconn.DefaultResolver()
	}
	return NewRuntime(resolver, registry)
}

type AgentSession struct {
	ID      string
	Profile ModelProfile
	Agent   agent.Agent

	release func(context.Context) error
	once    sync.Once
	err     error
}

// InteractionBinding grants one agent session a policy-filtered view of a
// caller-owned Jangolova interaction instance.
type InteractionBinding struct {
	InteractionID       string
	Caller              bridge.Caller
	AllowedCapabilities []string
	Policy              CapabilityPolicy
	RequireApproval     func(bridge.Capability) bool
}

func (s *AgentSession) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		if s.release != nil {
			s.err = s.release(ctx)
		}
	})
	return s.err
}

// CreateAgent resolves caller-owned model material, chooses a connector, and
// creates an ADK agent. Northbound HTTP/MCP/ACP adapters will call this same
// method; none of them owns model-selection or credential logic.
func (r *Runtime) CreateAgent(ctx context.Context, spec AgentSpec, tools []tool.Tool) (*AgentSession, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	connector, ok := r.connectors.Connector(spec.Model.Protocol)
	if !ok {
		return nil, fmt.Errorf("Grimlock model protocol %q is not registered", spec.Model.Protocol)
	}
	target := orchestrator.EngineTarget{
		TargetID: "grimlock:" + spec.SessionID + ":" + spec.Model.ProfileID,
		Kind:     "model-gateway",
		Endpoints: []orchestrator.TargetEndpoint{{
			Name: "model", Protocol: spec.Model.Protocol, URL: spec.Model.Endpoint,
			CredentialRef: spec.Model.CredentialRef, TLSRef: spec.Model.TLSRef, Audience: "model",
		}},
	}
	prepared, release, err := targetconn.Prepare(ctx, r.resolver, target)
	if err != nil {
		return nil, err
	}
	connection := ModelConnection{profile: cloneModelProfile(spec.Model), endpoint: prepared.Endpoints[0]}
	connected, err := connector.Connect(ctx, connection)
	if err != nil {
		redacted := targetconn.Redact(err, prepared)
		_ = release(context.Background())
		return nil, fmt.Errorf("connect Grimlock model: %w", redacted)
	}
	if connected.LLM == nil {
		if connected.Close != nil {
			connected.Close()
		}
		_ = release(context.Background())
		return nil, errors.New("Grimlock model connector returned no model")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = DefaultAgentName
	}
	description := strings.TrimSpace(spec.Description)
	if description == "" {
		description = DefaultDescription
	}
	instruction := strings.TrimSpace(spec.Instruction)
	if instruction == "" {
		instruction = DefaultInstruction
	}
	root, err := llmagent.New(llmagent.Config{
		Name: name, Description: description, Instruction: instruction, Model: connected.LLM,
		Tools: append([]tool.Tool(nil), tools...),
	})
	if err != nil {
		if connected.Close != nil {
			connected.Close()
		}
		_ = release(context.Background())
		return nil, fmt.Errorf("create Grimlock ADK agent: %w", err)
	}
	cleanup := func(cleanupCtx context.Context) error {
		if connected.Close != nil {
			connected.Close()
		}
		return release(cleanupCtx)
	}
	return &AgentSession{ID: spec.SessionID, Profile: cloneModelProfile(spec.Model), Agent: root, release: cleanup}, nil
}

// CreateInteractionAgent discovers admitted capabilities from every binding,
// creates namespaced ADK tools, and connects them to the caller-selected model.
// Protocol adapters call this application boundary rather than implementing
// capability discovery or policy themselves.
func (r *Runtime) CreateInteractionAgent(ctx context.Context, spec AgentSpec, bindings []InteractionBinding) (*AgentSession, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return nil, errors.New("Grimlock requires at least one interaction binding")
	}
	var agentTools []tool.Tool
	usedInteractions := make(map[string]struct{}, len(bindings))
	usedTools := make(map[string]struct{})
	for _, binding := range bindings {
		interactionID := strings.TrimSpace(binding.InteractionID)
		if _, exists := usedInteractions[interactionID]; exists {
			return nil, fmt.Errorf("Grimlock interaction binding %q is duplicated", interactionID)
		}
		usedInteractions[interactionID] = struct{}{}
		bindingTools, err := InteractionTools(ctx, InteractionToolSpec{
			SessionID: spec.SessionID, InteractionID: interactionID,
			Caller: binding.Caller, AllowedCapabilities: binding.AllowedCapabilities,
			Policy: binding.Policy, RequireApproval: binding.RequireApproval,
		})
		if err != nil {
			return nil, fmt.Errorf("bind Grimlock interaction %q: %w", interactionID, err)
		}
		for _, candidate := range bindingTools {
			if _, exists := usedTools[candidate.Name()]; exists {
				return nil, fmt.Errorf("Grimlock interaction bindings produce duplicate tool %q", candidate.Name())
			}
			usedTools[candidate.Name()] = struct{}{}
			agentTools = append(agentTools, candidate)
		}
	}
	return r.CreateAgent(ctx, spec, agentTools)
}
