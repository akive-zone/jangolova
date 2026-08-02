package grimlock

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"google.golang.org/genai"
)

// ACPProtocolVersion is the stable Agent Client Protocol major version. The
// adapter implements the stdio lifecycle used by local editors: initialize,
// session/new/load/close, session/prompt, session/update, and cancellation.
const ACPProtocolVersion = 1

type ACPServer struct {
	service     *Service
	writeMu     sync.Mutex
	initialized bool
	output      *json.Encoder
}

func NewACPServer(service *Service) (*ACPServer, error) {
	if service == nil {
		return nil, errors.New("Grimlock ACP service is required")
	}
	return &ACPServer{service: service}, nil
}

// ServeStdio serves ACP's JSON-RPC line protocol. The model profile and
// target bindings are carried in the Jangolova extension fields of
// session/new; no model gateway or target is inferred by this adapter.
func (a *ACPServer) ServeStdio(ctx context.Context, input io.Reader, output io.Writer) error {
	a.output = json.NewEncoder(output)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 2*maxJSONBody)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var request acpRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			if err := a.write(acpErrorResponse(nil, -32700, "parse error")); err != nil {
				return err
			}
			continue
		}
		response, err := a.dispatch(ctx, request)
		if err != nil {
			if request.ID != nil {
				if writeErr := a.write(acpErrorResponse(request.ID, -32000, err.Error())); writeErr != nil {
					return writeErr
				}
			}
			continue
		}
		if response != nil {
			if err := a.write(response); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

type acpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type acpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *acpError       `json:"error,omitempty"`
}

type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func acpErrorResponse(id json.RawMessage, code int, message string) *acpResponse {
	return &acpResponse{JSONRPC: "2.0", ID: id, Error: &acpError{Code: code, Message: message}}
}

func (a *ACPServer) write(value any) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.output.Encode(value)
}

func (a *ACPServer) notify(sessionID string, update map[string]any) error {
	return a.write(map[string]any{
		"jsonrpc": "2.0", "method": "session/update",
		"params": map[string]any{"sessionId": sessionID, "update": update},
	})
}

func (a *ACPServer) dispatch(ctx context.Context, request acpRequest) (*acpResponse, error) {
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		return nil, errors.New("invalid JSON-RPC request")
	}
	if request.Method != "initialize" && !a.initialized {
		return nil, errors.New("ACP initialize is required before other methods")
	}
	switch request.Method {
	case "initialize":
		if a.initialized {
			return nil, errors.New("ACP connection is already initialized")
		}
		a.initialized = true
		return &acpResponse{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{
			"protocolVersion": ACPProtocolVersion,
			"agentCapabilities": map[string]any{
				"loadSession":         true,
				"promptCapabilities":  map[string]any{"image": false, "audio": false, "embeddedContext": false},
				"mcpCapabilities":     map[string]any{"http": false, "sse": false},
				"sessionCapabilities": map[string]any{"close": map[string]any{}},
			},
			"agentInfo":   map[string]any{"name": "jangolova-grimlock", "title": "Jangolova Grimlock", "version": "0.1.0"},
			"authMethods": []any{},
		}}, nil
	case "initialized", "notifications/initialized":
		return nil, nil
	case "authenticate":
		return nil, errors.New("ACP authentication is provided by the process or transport boundary")
	case "session/new":
		return a.newSession(ctx, request)
	case "session/load":
		return a.loadSession(ctx, request)
	case "session/close":
		return a.closeSession(ctx, request)
	case "session/prompt":
		return a.promptSession(ctx, request)
	case "session/cancel":
		return a.cancelSession(request)
	default:
		return nil, fmt.Errorf("ACP method %q is not supported", request.Method)
	}
}

type acpSessionParams struct {
	CWD       string          `json:"cwd,omitempty"`
	UserID    string          `json:"userId,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Agent     AgentSpec       `json:"agent,omitempty"`
	Model     ModelProfile    `json:"model,omitempty"`
	Bindings  []BindingSpec   `json:"bindings,omitempty"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

func (a *ACPServer) newSession(ctx context.Context, request acpRequest) (*acpResponse, error) {
	var params acpSessionParams
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, fmt.Errorf("decode session/new params: %w", err)
	}
	if len(params.Meta) != 0 {
		var meta struct {
			UserID   string        `json:"userId"`
			Agent    AgentSpec     `json:"agent"`
			Model    ModelProfile  `json:"model"`
			Bindings []BindingSpec `json:"bindings"`
		}
		if err := json.Unmarshal(params.Meta, &meta); err == nil {
			if params.UserID == "" {
				params.UserID = meta.UserID
			}
			if params.Agent.Model.ProfileID == "" {
				params.Agent = meta.Agent
			}
			if params.Model.ProfileID == "" {
				params.Model = meta.Model
			}
			if len(params.Bindings) == 0 {
				params.Bindings = meta.Bindings
			}
		}
	}
	if params.SessionID == "" {
		params.SessionID = params.Agent.SessionID
	}
	if params.SessionID == "" {
		params.SessionID = newOpaqueID("acp-")
	}
	if params.Agent.SessionID == "" {
		params.Agent.SessionID = params.SessionID
	}
	if params.Agent.Model.ProfileID == "" {
		params.Agent.Model = params.Model
	}
	if params.UserID == "" {
		params.UserID = "default"
	}
	createRequest := CreateSessionRequest{APIVersion: APIVersion, UserID: params.UserID, Agent: params.Agent, Bindings: params.Bindings}
	if err := validateCreateSessionRequest(createRequest); err != nil {
		return nil, err
	}
	if err := a.service.createSession(ctx, createRequest); err != nil {
		return nil, err
	}
	return &acpResponse{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{"sessionId": params.SessionID}}, nil
}

func (a *ACPServer) loadSession(ctx context.Context, request acpRequest) (*acpResponse, error) {
	var params struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || !sessionIDPattern.MatchString(params.SessionID) {
		return nil, errors.New("session/load requires a valid sessionId")
	}
	record, ok := a.service.lookupSession(params.SessionID)
	if !ok {
		return nil, errors.New("Grimlock session was not found")
	}
	record.mu.Lock()
	events := append([]EventEnvelope(nil), record.events...)
	record.mu.Unlock()
	for _, event := range events {
		if err := a.notifyEvent(params.SessionID, event); err != nil {
			return nil, err
		}
	}
	return &acpResponse{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{"sessionId": params.SessionID}}, nil
}

func (a *ACPServer) closeSession(ctx context.Context, request acpRequest) (*acpResponse, error) {
	var params struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || params.SessionID == "" {
		return nil, errors.New("session/close requires sessionId")
	}
	if err := a.service.deleteSession(ctx, params.SessionID); err != nil {
		return nil, err
	}
	return &acpResponse{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{}}, nil
}

func (a *ACPServer) promptSession(ctx context.Context, request acpRequest) (*acpResponse, error) {
	var params struct {
		SessionID string          `json:"sessionId"`
		Prompt    json.RawMessage `json:"prompt"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || !sessionIDPattern.MatchString(params.SessionID) {
		return nil, errors.New("session/prompt requires a valid sessionId")
	}
	text, err := acpPromptText(params.Prompt)
	if err != nil {
		return nil, err
	}
	record, ok := a.service.lookupSession(params.SessionID)
	if !ok {
		return nil, errors.New("Grimlock session was not found")
	}
	_, err = a.service.execute(ctx, record, &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: text}}}, true, func(event EventEnvelope) bool {
		return a.notifyEvent(params.SessionID, event) == nil
	})
	if err != nil {
		return nil, err
	}
	return &acpResponse{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{"sessionId": params.SessionID, "stopReason": "end_turn"}}, nil
}

func (a *ACPServer) cancelSession(request acpRequest) (*acpResponse, error) {
	var params struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || params.SessionID == "" {
		return nil, errors.New("session/cancel requires sessionId")
	}
	if _, err := a.service.cancelSession(params.SessionID); err != nil {
		return nil, err
	}
	if len(request.ID) == 0 {
		return nil, nil
	}
	return &acpResponse{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{"sessionId": params.SessionID}}, nil
}

func acpPromptText(raw json.RawMessage) (string, error) {
	var plain string
	if json.Unmarshal(raw, &plain) == nil && strings.TrimSpace(plain) != "" {
		return plain, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", errors.New("session/prompt prompt must be text or an array of text content blocks")
	}
	var parts []string
	for _, block := range blocks {
		if block.Type != "text" || block.Text == "" {
			return "", errors.New("ACP Grimlock supports text prompt blocks only")
		}
		parts = append(parts, block.Text)
	}
	if len(parts) == 0 {
		return "", errors.New("session/prompt requires text content")
	}
	return strings.Join(parts, ""), nil
}

func (a *ACPServer) notifyEvent(sessionID string, event EventEnvelope) error {
	text := acpEventText(event.Event)
	if text == "" {
		text = "Grimlock execution event " + event.Cursor
	}
	return a.notify(sessionID, map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]any{"type": "text", "text": text},
		"_meta":         map[string]any{"jangolovaCursor": event.Cursor, "event": json.RawMessage(event.Event)},
	})
}

func acpEventText(raw json.RawMessage) string {
	var event struct {
		Content struct {
			Parts []struct {
				Text string `json:"text,omitempty"`
			} `json:"parts,omitempty"`
		} `json:"content,omitempty"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return ""
	}
	var parts []string
	for _, part := range event.Content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "")
}
