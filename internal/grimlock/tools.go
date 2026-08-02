package grimlock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/toolutils"
	"google.golang.org/genai"

	"jangolova/internal/bridge"
)

var invalidToolNameCharacters = regexp.MustCompile(`[^a-z0-9_]+`)

// CapabilityRequest is the policy input for one interaction operation.
// Input is nil while a capability is being considered for advertisement and
// contains the model-supplied arguments immediately before execution.
type CapabilityRequest struct {
	SessionID     string
	InteractionID string
	Capability    bridge.Capability
	Input         json.RawMessage
}

// CapabilityPolicy is authoritative. Advertise controls which operations a
// model can see; Authorize is called again immediately before execution.
// Human confirmation is an additional control and never replaces Authorize.
type CapabilityPolicy interface {
	Advertise(context.Context, CapabilityRequest) bool
	Authorize(context.Context, CapabilityRequest) error
}

// CapabilityPolicyFuncs adapts functions into a CapabilityPolicy. Nil
// functions use the secure defaults: advertise reads and deny execution.
type CapabilityPolicyFuncs struct {
	AdvertiseFunc func(context.Context, CapabilityRequest) bool
	AuthorizeFunc func(context.Context, CapabilityRequest) error
}

func (p CapabilityPolicyFuncs) Advertise(ctx context.Context, request CapabilityRequest) bool {
	if p.AdvertiseFunc != nil {
		return p.AdvertiseFunc(ctx, request)
	}
	return request.Capability.Effect == bridge.EffectRead
}

func (p CapabilityPolicyFuncs) Authorize(ctx context.Context, request CapabilityRequest) error {
	if p.AuthorizeFunc != nil {
		return p.AuthorizeFunc(ctx, request)
	}
	return errors.New("Grimlock capability execution is not authorized")
}

// ReadOnlyCapabilityPolicy admits and authorizes only read capabilities.
// It is useful for observation-only agents and is the default when a toolset
// does not supply a policy.
type ReadOnlyCapabilityPolicy struct{}

func (ReadOnlyCapabilityPolicy) Advertise(_ context.Context, request CapabilityRequest) bool {
	return request.Capability.Effect == bridge.EffectRead
}

func (ReadOnlyCapabilityPolicy) Authorize(_ context.Context, request CapabilityRequest) error {
	if request.Capability.Effect != bridge.EffectRead {
		return fmt.Errorf("Grimlock capability %q with effect %q is not authorized", request.Capability.Name, request.Capability.Effect)
	}
	return nil
}

// InteractionToolSpec binds one caller-owned interaction instance to a
// Grimlock agent session. AllowedCapabilities is an additional allowlist. An
// empty allowlist admits every capability accepted by Policy.
type InteractionToolSpec struct {
	SessionID           string
	InteractionID       string
	Caller              bridge.Caller
	AllowedCapabilities []string
	Policy              CapabilityPolicy
	RequireApproval     func(bridge.Capability) bool
}

// InteractionTools discovers an interaction's capabilities and returns ADK
// tools for the admitted operations plus describe and event observation.
func InteractionTools(ctx context.Context, spec InteractionToolSpec) ([]tool.Tool, error) {
	if !sessionIDPattern.MatchString(spec.SessionID) {
		return nil, errors.New("Grimlock tool sessionId must be a lowercase DNS-style name")
	}
	if strings.TrimSpace(spec.InteractionID) == "" {
		return nil, errors.New("Grimlock interactionId is required")
	}
	if spec.Caller == nil {
		return nil, errors.New("Grimlock interaction caller is required")
	}
	policy := spec.Policy
	if policy == nil {
		policy = ReadOnlyCapabilityPolicy{}
	}
	allowed, err := capabilityAllowlist(spec.AllowedCapabilities)
	if err != nil {
		return nil, err
	}
	capabilities, err := discoverCapabilities(ctx, spec.Caller)
	if err != nil {
		return nil, err
	}

	tools := make([]tool.Tool, 0, len(capabilities)+2)
	discovered := make(map[string]struct{}, len(capabilities))
	describeName := capabilityToolName(spec.InteractionID, "describe")
	eventsName := capabilityToolName(spec.InteractionID, "events")
	usedNames := map[string]string{
		describeName: "describe",
		eventsName:   "events",
	}
	readOperations := []bridge.Capability{
		{Name: "describe", Description: "Describe the current interaction state.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), Effect: bridge.EffectRead},
		{Name: "events", Description: "Read interaction events after an optional cursor.", InputSchema: eventQuerySchema(), Effect: bridge.EffectRead},
	}
	for _, capability := range readOperations {
		request := CapabilityRequest{SessionID: spec.SessionID, InteractionID: spec.InteractionID, Capability: capability}
		if !policy.Advertise(ctx, request) {
			continue
		}
		method, name := bridge.MethodDescribe, describeName
		if capability.Name == "events" {
			method, name = bridge.MethodEvents, eventsName
		}
		interactionTool, err := newInteractionTool(spec, policy, capability, method, name, false)
		if err != nil {
			return nil, err
		}
		tools = append(tools, interactionTool)
	}

	for _, capability := range capabilities {
		discovered[capability.Name] = struct{}{}
		if len(allowed) != 0 {
			if _, ok := allowed[capability.Name]; !ok {
				continue
			}
		}
		request := CapabilityRequest{SessionID: spec.SessionID, InteractionID: spec.InteractionID, Capability: capability}
		if !policy.Advertise(ctx, request) {
			continue
		}
		name := capabilityToolName(spec.InteractionID, capability.Name)
		if prior, exists := usedNames[name]; exists {
			return nil, fmt.Errorf("Grimlock capabilities %q and %q produce duplicate tool name %q", prior, capability.Name, name)
		}
		usedNames[name] = capability.Name
		requireApproval := capability.Effect != bridge.EffectRead
		if spec.RequireApproval != nil {
			requireApproval = spec.RequireApproval(capability)
		}
		interactionTool, err := newInteractionTool(spec, policy, capability, bridge.MethodAct, name, requireApproval)
		if err != nil {
			return nil, err
		}
		tools = append(tools, interactionTool)
	}
	for name := range allowed {
		if _, ok := discovered[name]; !ok {
			return nil, fmt.Errorf("Grimlock allowed capability %q was not advertised by interaction %q", name, spec.InteractionID)
		}
	}
	return tools, nil
}

type interactionTool struct {
	spec            InteractionToolSpec
	policy          CapabilityPolicy
	capability      bridge.Capability
	method          string
	name            string
	schema          *jsonschema.Resolved
	requireApproval bool
}

var (
	_ tool.Tool = (*interactionTool)(nil)
	_ interface {
		Declaration() *genai.FunctionDeclaration
		ProcessRequest(agent.Context, *model.LLMRequest) error
		Run(agent.Context, any) (map[string]any, error)
	} = (*interactionTool)(nil)
)

func newInteractionTool(spec InteractionToolSpec, policy CapabilityPolicy, capability bridge.Capability, method, name string, requireApproval bool) (*interactionTool, error) {
	var schema jsonschema.Schema
	if err := json.Unmarshal(capability.InputSchema, &schema); err != nil {
		return nil, fmt.Errorf("Grimlock capability %q input schema: %w", capability.Name, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("resolve Grimlock capability %q input schema: %w", capability.Name, err)
	}
	return &interactionTool{
		spec: spec, policy: policy, capability: capability, method: method,
		name: name, schema: resolved, requireApproval: requireApproval,
	}, nil
}

func (t *interactionTool) Name() string        { return t.name }
func (t *interactionTool) Description() string { return t.capability.Description }
func (t *interactionTool) IsLongRunning() bool { return false }

func (t *interactionTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name: t.name, Description: t.capability.Description,
		ParametersJsonSchema: t.schema.Schema(),
	}
}

func (t *interactionTool) ProcessRequest(_ agent.Context, request *model.LLMRequest) error {
	return toolutils.PackTool(request, t)
}

func (t *interactionTool) Run(ctx agent.Context, arguments any) (map[string]any, error) {
	if t.requireApproval {
		if confirmation := ctx.ToolConfirmation(); confirmation != nil {
			if !confirmation.Confirmed {
				return nil, fmt.Errorf("Grimlock tool %q %w", t.name, tool.ErrConfirmationRejected)
			}
		} else {
			if err := ctx.RequestConfirmation(
				fmt.Sprintf("Approve Jangolova capability %s (%s)", t.capability.Name, t.capability.Effect),
				map[string]any{"interactionId": t.spec.InteractionID, "capability": t.capability.Name, "effect": t.capability.Effect},
			); err != nil {
				return nil, err
			}
			ctx.Actions().SkipSummarization = true
			return nil, fmt.Errorf("Grimlock tool %q %w", t.name, tool.ErrConfirmationRequired)
		}
	}
	return t.execute(ctx, arguments)
}

func (t *interactionTool) execute(ctx context.Context, arguments any) (map[string]any, error) {
	input, ok := arguments.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Grimlock tool %q expected object arguments, got %T", t.name, arguments)
	}
	if err := t.schema.Validate(input); err != nil {
		return nil, fmt.Errorf("Grimlock tool %q arguments: %w", t.name, err)
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode Grimlock tool %q arguments: %w", t.name, err)
	}
	request := CapabilityRequest{
		SessionID: t.spec.SessionID, InteractionID: t.spec.InteractionID,
		Capability: t.capability, Input: inputJSON,
	}
	if err := t.policy.Authorize(ctx, request); err != nil {
		return nil, fmt.Errorf("authorize Grimlock capability %q: %w", t.capability.Name, err)
	}

	wireInput := json.RawMessage(inputJSON)
	if t.method == bridge.MethodAct {
		wireInput, err = json.Marshal(map[string]any{"name": t.capability.Name, "input": json.RawMessage(inputJSON)})
		if err != nil {
			return nil, fmt.Errorf("encode Grimlock capability %q request: %w", t.capability.Name, err)
		}
	}
	result, err := t.spec.Caller.Call(ctx, t.method, wireInput)
	if err != nil {
		return nil, fmt.Errorf("execute Grimlock capability %q: %w", t.capability.Name, err)
	}
	if !json.Valid(result) {
		return nil, fmt.Errorf("Grimlock capability %q returned invalid JSON", t.capability.Name)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(result))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("Grimlock capability %q returned invalid JSON", t.capability.Name)
	}
	return map[string]any{
		"interactionId": t.spec.InteractionID,
		"capability":    t.capability.Name,
		"effect":        t.capability.Effect,
		"result":        value,
	}, nil
}

func discoverCapabilities(ctx context.Context, caller bridge.Caller) ([]bridge.Capability, error) {
	raw, err := caller.Call(ctx, bridge.MethodCapabilities, json.RawMessage(`{}`))
	if err != nil {
		return nil, fmt.Errorf("discover Grimlock interaction capabilities: %w", err)
	}
	if !json.Valid(raw) {
		return nil, errors.New("decode Grimlock interaction capabilities: invalid JSON")
	}
	var capabilities []bridge.Capability
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&capabilities); err != nil {
		return nil, fmt.Errorf("decode Grimlock interaction capabilities: %w", err)
	}
	seen := make(map[string]struct{}, len(capabilities))
	for index, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == "" {
			return nil, fmt.Errorf("Grimlock capability %d name is required", index)
		}
		if _, exists := seen[capability.Name]; exists {
			return nil, fmt.Errorf("Grimlock capability %q is duplicated", capability.Name)
		}
		seen[capability.Name] = struct{}{}
		switch capability.Effect {
		case bridge.EffectRead, bridge.EffectWrite, bridge.EffectExternal:
		default:
			return nil, fmt.Errorf("Grimlock capability %q has invalid effect %q", capability.Name, capability.Effect)
		}
		var schema map[string]any
		if err := json.Unmarshal(capability.InputSchema, &schema); err != nil || schema == nil {
			return nil, fmt.Errorf("Grimlock capability %q inputSchema must be a JSON object", capability.Name)
		}
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].Name < capabilities[j].Name })
	return capabilities, nil
}

func capabilityAllowlist(names []string) (map[string]struct{}, error) {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, errors.New("Grimlock allowed capability name is required")
		}
		if _, exists := allowed[name]; exists {
			return nil, fmt.Errorf("Grimlock allowed capability %q is duplicated", name)
		}
		allowed[name] = struct{}{}
	}
	return allowed, nil
}

func capabilityToolName(interactionID, capability string) string {
	rawName := interactionID + "_" + capability
	name := invalidToolNameCharacters.ReplaceAllString(strings.ToLower(strings.TrimSpace(rawName)), "_")
	name = strings.Trim(name, "_")
	if name == "" {
		name = "capability"
	}
	const prefix = "jangolova_"
	if len(prefix)+len(name) <= 64 {
		return prefix + name
	}
	digest := sha256.Sum256([]byte(interactionID + "\x00" + capability))
	suffix := "_" + hex.EncodeToString(digest[:4])
	return prefix + name[:64-len(prefix)-len(suffix)] + suffix
}

func eventQuerySchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"after":{"type":"string"},
			"types":{"type":"array","items":{"type":"string"},"maxItems":64},
			"limit":{"type":"integer","minimum":1,"maximum":100}
		},
		"additionalProperties":false
	}`)
}
