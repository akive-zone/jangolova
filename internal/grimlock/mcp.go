package grimlock

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/genai"
)

// MCPProtocolVersion is the MCP revision negotiated by Grimlock's native
// adapter. The adapter supports JSON-RPC over stdio and single-request POSTs;
// all calls still execute through the shared Grimlock service.
const MCPProtocolVersion = "2025-11-25"

type MCPServer struct {
	service *Service
	mu      sync.Mutex
	streams map[string]struct{}
}

func NewMCPServer(service *Service) (*MCPServer, error) {
	if service == nil {
		return nil, errors.New("Grimlock MCP service is required")
	}
	return &MCPServer{service: service, streams: make(map[string]struct{})}, nil
}

// Routes serves the MCP Streamable HTTP request endpoint at /mcp. Long-lived
// GET/SSE streams are intentionally deferred; stdio remains the primary local
// transport and POST requests are useful for simple remote integrations.
func (m *MCPServer) Routes() http.Handler {
	return m.service.authorize(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		m.handleHTTP(w, r)
	}))
}

func (m *MCPServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "MCP supports POST requests; use stdio for bidirectional streaming")
		return
	}
	if contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); contentType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "invalid_content_type", "MCP requests must use application/json")
		return
	}
	var request mcpRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, mcpErrorResponse(nil, -32700, "parse error"))
		return
	}
	if request.Method == "initialize" {
		if r.Header.Get("Mcp-Session-Id") != "" {
			writeJSON(w, http.StatusBadRequest, mcpErrorResponse(request.ID, -32600, "initialize must not include Mcp-Session-Id"))
			return
		}
		sessionID := newOpaqueID("mcp-")
		m.mu.Lock()
		m.streams[sessionID] = struct{}{}
		m.mu.Unlock()
		response := m.dispatch(r.Context(), request)
		if response == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Mcp-Session-Id", sessionID)
		writeJSON(w, http.StatusOK, response)
		return
	}
	streamID := strings.TrimSpace(r.Header.Get("Mcp-Session-Id"))
	if streamID == "" {
		writeJSON(w, http.StatusBadRequest, mcpErrorResponse(request.ID, -32600, "Mcp-Session-Id is required after initialize"))
		return
	}
	m.mu.Lock()
	_, known := m.streams[streamID]
	m.mu.Unlock()
	if !known {
		writeJSON(w, http.StatusNotFound, mcpErrorResponse(request.ID, -32000, "unknown MCP session"))
		return
	}
	response := m.dispatch(r.Context(), request)
	if response == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// ServeStdio runs the MCP JSON-RPC line protocol. Process ownership provides
// the trust boundary, so model and target credentials remain caller-supplied
// in the Grimlock session request and are never read from this transport.
func (m *MCPServer) ServeStdio(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 2*maxJSONBody)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var request mcpRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			if err := encoder.Encode(mcpErrorResponse(nil, -32700, "parse error")); err != nil {
				return err
			}
			continue
		}
		response := m.dispatch(ctx, request)
		if response == nil {
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func mcpErrorResponse(id json.RawMessage, code int, message string) mcpResponse {
	return mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpRPCError{Code: code, Message: message}}
}

func (m *MCPServer) dispatch(ctx context.Context, request mcpRequest) *mcpResponse {
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32600, Message: "invalid JSON-RPC request"}}
	}
	if len(request.ID) == 0 && strings.HasPrefix(request.Method, "notifications/") {
		return nil
	}
	switch request.Method {
	case "initialize":
		return mcpResult(request.ID, map[string]any{
			"protocolVersion": MCPProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"serverInfo":      map[string]any{"name": "jangolova-grimlock", "version": "0.1.0"},
		})
	case "notifications/initialized":
		return nil
	case "ping":
		return mcpResult(request.ID, map[string]any{})
	case "tools/list":
		return mcpResult(request.ID, map[string]any{"tools": grimlockMCPTools()})
	case "resources/list":
		return mcpResult(request.ID, map[string]any{
			"resources": []map[string]any{{"uri": "grimlock://connectors", "name": "Model connectors", "mimeType": "application/json"}},
			"resourceTemplates": []map[string]any{
				{"uriTemplate": "grimlock://sessions/{sessionId}", "name": "Grimlock session", "mimeType": "application/json"},
				{"uriTemplate": "grimlock://sessions/{sessionId}/events?after={cursor}", "name": "Grimlock event history", "mimeType": "application/json"},
			},
		})
	case "resources/read":
		return m.dispatchMCPResourcesRead(ctx, request)
	case "tools/call":
		return m.dispatchMCPToolCall(ctx, request)
	default:
		return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32601, Message: "method not found"}}
	}
}

func mcpResult(id json.RawMessage, result any) *mcpResponse {
	return &mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func grimlockMCPTools() []mcpTool {
	object := func(properties map[string]any, required ...string) map[string]any {
		value := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) != 0 {
			value["required"] = required
		}
		return value
	}
	return []mcpTool{
		{Name: "grimlock_connectors", Description: "List caller-selectable Grimlock model connector protocols.", InputSchema: object(nil)},
		{Name: "grimlock_session_create", Description: "Create a Grimlock session with a caller-supplied model profile and target bindings.", InputSchema: object(map[string]any{"request": map[string]any{"type": "object"}}, "request")},
		{Name: "grimlock_session_run", Description: "Submit a prompt to a Grimlock session and return execution events.", InputSchema: object(map[string]any{"sessionId": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}}, "sessionId", "text")},
		{Name: "grimlock_session_events", Description: "Read retained Grimlock execution events after a cursor.", InputSchema: object(map[string]any{"sessionId": map[string]any{"type": "string"}, "after": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}, "sessionId")},
		{Name: "grimlock_session_confirm", Description: "Approve or reject a pending Grimlock capability confirmation.", InputSchema: object(map[string]any{"sessionId": map[string]any{"type": "string"}, "approvalId": map[string]any{"type": "string"}, "confirmed": map[string]any{"type": "boolean"}}, "sessionId", "approvalId", "confirmed")},
		{Name: "grimlock_session_cancel", Description: "Cancel the active run for a Grimlock session.", InputSchema: object(map[string]any{"sessionId": map[string]any{"type": "string"}}, "sessionId")},
		{Name: "grimlock_session_close", Description: "Close a Grimlock session and release its adapters without stopping its target.", InputSchema: object(map[string]any{"sessionId": map[string]any{"type": "string"}}, "sessionId")},
	}
}

func (m *MCPServer) dispatchMCPToolCall(ctx context.Context, request mcpRequest) *mcpResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
		return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32602, Message: "tools/call requires name and arguments"}}
	}
	if len(params.Arguments) == 0 {
		params.Arguments = json.RawMessage(`{}`)
	}
	value, err := m.callMCPTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return mcpResult(request.ID, map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": err.Error()}}})
	}
	encoded, _ := json.Marshal(value)
	return mcpResult(request.ID, map[string]any{"isError": false, "content": []map[string]any{{"type": "text", "text": string(encoded)}}, "structuredContent": value})
}

func (m *MCPServer) callMCPTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "grimlock_connectors":
		protocols := m.service.runtime.ModelProtocols()
		return map[string]any{"apiVersion": APIVersion, "connectors": protocols}, nil
	case "grimlock_session_create":
		var args struct {
			Request CreateSessionRequest `json:"request"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode session create arguments: %w", err)
		}
		if err := validateCreateSessionRequest(args.Request); err != nil {
			return nil, err
		}
		if err := m.service.createSession(ctx, args.Request); err != nil {
			return nil, err
		}
		record, ok := m.service.lookupSession(args.Request.Agent.SessionID)
		if !ok {
			return nil, errors.New("created Grimlock session disappeared")
		}
		return record.snapshot(), nil
	case "grimlock_session_run":
		var args struct {
			SessionID string `json:"sessionId"`
			Text      string `json:"text"`
		}
		if err := json.Unmarshal(raw, &args); err != nil || strings.TrimSpace(args.SessionID) == "" || strings.TrimSpace(args.Text) == "" {
			return nil, errors.New("sessionId and text are required")
		}
		record, ok := m.service.lookupSession(args.SessionID)
		if !ok {
			return nil, errors.New("Grimlock session was not found")
		}
		events, err := m.service.execute(ctx, record, &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: args.Text}}}, false, nil)
		if err != nil {
			return nil, err
		}
		return RunResponse{APIVersion: APIVersion, SessionID: args.SessionID, Cursor: strconv.FormatUint(record.cursor(), 10), Events: events}, nil
	case "grimlock_session_events":
		var args struct {
			SessionID string `json:"sessionId"`
			After     string `json:"after,omitempty"`
			Limit     string `json:"limit,omitempty"`
		}
		if err := json.Unmarshal(raw, &args); err != nil || strings.TrimSpace(args.SessionID) == "" {
			return nil, errors.New("sessionId is required")
		}
		after, err := parseGrimlockCursor(args.After)
		if err != nil {
			return nil, err
		}
		limit, err := parseGrimlockLimit(args.Limit)
		if err != nil {
			return nil, err
		}
		record, ok := m.service.lookupSession(args.SessionID)
		if !ok {
			return nil, errors.New("Grimlock session was not found")
		}
		record.mu.Lock()
		defer record.mu.Unlock()
		if after > record.nextCursor {
			return nil, errors.New("event cursor is ahead of the session")
		}
		value := EventsResponse{APIVersion: APIVersion, SessionID: args.SessionID, Cursor: strconv.FormatUint(record.nextCursor, 10), Events: []EventEnvelope{}}
		for _, event := range record.events {
			sequence, _ := strconv.ParseUint(event.Cursor, 10, 64)
			if sequence > after {
				value.Events = append(value.Events, cloneEventEnvelope(event))
				if len(value.Events) == limit {
					break
				}
			}
		}
		return value, nil
	case "grimlock_session_confirm":
		var args struct {
			SessionID  string `json:"sessionId"`
			ApprovalID string `json:"approvalId"`
			Confirmed  bool   `json:"confirmed"`
		}
		if err := json.Unmarshal(raw, &args); err != nil || args.SessionID == "" || args.ApprovalID == "" {
			return nil, errors.New("sessionId and approvalId are required")
		}
		record, ok := m.service.lookupSession(args.SessionID)
		if !ok {
			return nil, errors.New("Grimlock session was not found")
		}
		record.mu.Lock()
		if _, exists := record.pending[args.ApprovalID]; !exists {
			record.mu.Unlock()
			return nil, errors.New("approval request was not found or already resolved")
		}
		delete(record.pending, args.ApprovalID)
		record.mu.Unlock()
		events, err := m.service.execute(ctx, record, &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: args.ApprovalID, Name: "adk_request_confirmation", Response: map[string]any{"confirmed": args.Confirmed}}}}}, false, nil)
		if err != nil {
			return nil, err
		}
		return RunResponse{APIVersion: APIVersion, SessionID: args.SessionID, Cursor: strconv.FormatUint(record.cursor(), 10), Events: events}, nil
	case "grimlock_session_cancel":
		var args struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(raw, &args); err != nil || args.SessionID == "" {
			return nil, errors.New("sessionId is required")
		}
		return m.service.cancelSession(args.SessionID)
	case "grimlock_session_close":
		var args struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(raw, &args); err != nil || args.SessionID == "" {
			return nil, errors.New("sessionId is required")
		}
		return nil, m.service.deleteSession(ctx, args.SessionID)
	default:
		return nil, fmt.Errorf("unknown Grimlock MCP tool %q", name)
	}
}

func (m *MCPServer) dispatchMCPResourcesRead(ctx context.Context, request mcpRequest) *mcpResponse {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || params.URI == "" {
		return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32602, Message: "resources/read requires uri"}}
	}
	parsed, err := url.Parse(params.URI)
	if err != nil || parsed.Scheme != "grimlock" {
		return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32602, Message: "invalid Grimlock resource URI"}}
	}
	var value any
	switch {
	case parsed.Host == "connectors":
		value = map[string]any{"apiVersion": APIVersion, "connectors": m.service.runtime.ModelProtocols()}
	case parsed.Host == "sessions":
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) == 0 || !sessionIDPattern.MatchString(parts[0]) {
			return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32602, Message: "invalid session resource URI"}}
		}
		record, ok := m.service.lookupSession(parts[0])
		if !ok {
			return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32004, Message: "Grimlock session was not found"}}
		}
		if len(parts) == 1 {
			value = record.snapshot()
		} else if len(parts) == 2 && parts[1] == "events" {
			requestArgs := map[string]any{"sessionId": parts[0], "after": parsed.Query().Get("after")}
			if limit := parsed.Query().Get("limit"); limit != "" {
				requestArgs["limit"] = limit
			}
			raw, _ := json.Marshal(requestArgs)
			result, callErr := m.callMCPTool(ctx, "grimlock_session_events", raw)
			if callErr != nil {
				return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32000, Message: callErr.Error()}}
			}
			value = result
		} else {
			return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32602, Message: "unknown Grimlock resource"}}
		}
	default:
		return &mcpResponse{JSONRPC: "2.0", ID: request.ID, Error: &mcpRPCError{Code: -32602, Message: "unknown Grimlock resource"}}
	}
	encoded, _ := json.Marshal(value)
	return mcpResult(request.ID, map[string]any{"contents": []map[string]any{{"uri": params.URI, "mimeType": "application/json", "text": string(encoded)}}})
}

func newOpaqueID(prefix string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return prefix + "fallback"
	}
	return prefix + hex.EncodeToString(value)
}

func (s *Service) cancelSession(id string) (SessionView, error) {
	record, ok := s.lookupSession(id)
	if !ok {
		return SessionView{}, errors.New("Grimlock session was not found")
	}
	record.mu.Lock()
	cancel := record.activeCancel
	summary := record.snapshotLocked()
	record.mu.Unlock()
	if cancel == nil {
		return SessionView{}, errors.New("Grimlock session has no active run")
	}
	cancel()
	return summary, nil
}

func (s *Service) deleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	record, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	if !ok {
		return errors.New("Grimlock session was not found")
	}
	return s.closeSession(ctx, record)
}
