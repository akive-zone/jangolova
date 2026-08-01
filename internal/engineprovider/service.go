package engineprovider

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"jangolova/internal/bridge"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
	"jangolova/targetconn"
)

var instanceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var handleNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,127}$`)
var targetIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)

const eventHistoryLimit = 256

type Service struct {
	mu        sync.Mutex
	registry  *orchestrator.Registry
	token     string
	resolver  targetconn.Resolver
	instances map[string]*runningInstance
}

type runningInstance struct {
	adapter    string
	status     string
	health     Health
	instance   orchestrator.EngineInstance
	release    func(context.Context) error
	redact     func(string) string
	redactJSON func(json.RawMessage) json.RawMessage
	events     []InstanceEvent
	nextEvent  uint64
}

type ServiceOption func(*Service)

func WithTargetResolver(resolver targetconn.Resolver) ServiceOption {
	return func(service *Service) { service.resolver = resolver }
}

func NewService(registry *orchestrator.Registry, token string, options ...ServiceOption) (*Service, error) {
	if registry == nil {
		return nil, errors.New("engine provider registry is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("engine provider token is required")
	}
	service := &Service{
		registry:  registry,
		token:     token,
		resolver:  targetconn.DefaultResolver(),
		instances: make(map[string]*runningInstance),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/engines", s.handleEngines)
	mux.HandleFunc("/v1/instances", s.handleInstances)
	mux.HandleFunc("/v1/instances/", s.handleInstance)
	return s.authorize(mux)
}

func (s *Service) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		supplied := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if !strings.HasPrefix(header, prefix) ||
			len(supplied) != len(s.token) ||
			subtle.ConstantTimeCompare([]byte(supplied), []byte(s.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authorization is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"service":    "jangolova-interaction-provider",
		"apiVersion": APIVersion,
	})
}

func (s *Service) handleEngines(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/engines" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion": APIVersion,
		"engines":    DiscoverEngines(r.Context(), s.registry),
	})
}

func (s *Service) handleInstances(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/instances" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var request ConnectRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid connection request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "connection request must contain one JSON value")
		return
	}
	if err := validateConnectRequest(request); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	adapterName := strings.TrimSpace(request.Engine.Adapter)
	if adapterName == "auto" {
		selected, selectErr := SelectAutomaticEngine(r.Context(), s.registry, request.Target, request.Engine.RequiredCapabilities)
		if selectErr != nil {
			writeError(w, http.StatusUnprocessableEntity, "engine_not_compatible", selectErr.Error())
			return
		}
		adapterName = selected
	}
	adapter, ok := s.registry.Engine(adapterName)
	if !ok {
		writeError(w, http.StatusNotFound, "engine_not_found", "interaction engine is not registered")
		return
	}

	s.mu.Lock()
	if _, exists := s.instances[request.InstanceID]; exists {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "instance_exists", "interaction instance already exists")
		return
	}
	record := &runningInstance{
		adapter: adapterName,
		status:  "connecting",
		health: Health{
			Status:     orchestrator.EngineHealthStarting,
			ObservedAt: time.Now().UTC(),
		},
	}
	appendInstanceEvent(record, orchestrator.EngineEvent{
		Type:       "instance.connecting",
		Status:     "connecting",
		OccurredAt: time.Now().UTC(),
	})
	s.instances[request.InstanceID] = record
	s.mu.Unlock()

	endpoints := make([]orchestrator.TargetEndpoint, 0, len(request.Target.Endpoints))
	for _, endpoint := range request.Target.Endpoints {
		endpoints = append(endpoints, orchestrator.TargetEndpoint{
			Name: endpoint.Name, Protocol: endpoint.Protocol, URL: endpoint.URL,
			CredentialRef: endpoint.CredentialRef, TLSRef: endpoint.TLSRef,
			Audience: endpoint.Audience, Metadata: cloneValues(endpoint.Metadata),
		})
	}
	target := orchestrator.EngineTarget{
		APIVersion: request.Target.APIVersion,
		TargetID:   request.Target.TargetID,
		Kind:       request.Target.Kind,
		Endpoints:  endpoints,
		Handles:    orchestrator.EngineHandles(cloneValues(request.Target.Handles)),
		Metadata:   cloneValues(request.Target.Metadata),
	}
	preparedTarget, release, err := targetconn.Prepare(r.Context(), s.resolver, target)
	if err != nil {
		s.mu.Lock()
		delete(s.instances, request.InstanceID)
		s.mu.Unlock()
		writeError(w, http.StatusBadGateway, "target_resolution_failed", err.Error())
		return
	}
	s.mu.Lock()
	record.release = release
	record.redact = func(message string) string { return targetconn.RedactString(message, preparedTarget) }
	record.redactJSON = func(value json.RawMessage) json.RawMessage { return targetconn.RedactJSON(value, preparedTarget) }
	s.mu.Unlock()
	instance, err := adapter.Connect(r.Context(), manifest.EngineSpec{
		Adapter:              adapterName,
		RequiredCapabilities: append([]string(nil), request.Engine.RequiredCapabilities...),
		Source:               request.Engine.Source,
		Options:              request.Engine.Options,
	}, preparedTarget)
	if err != nil {
		err = targetconn.Redact(err, preparedTarget)
		_ = release(context.Background())
		s.mu.Lock()
		delete(s.instances, request.InstanceID)
		s.mu.Unlock()
		writeError(w, http.StatusBadGateway, "engine_connect_failed", err.Error())
		return
	}
	if instance == nil {
		_ = release(context.Background())
		s.mu.Lock()
		delete(s.instances, request.InstanceID)
		s.mu.Unlock()
		writeError(
			w,
			http.StatusBadGateway,
			"engine_connect_failed",
			"interaction engine returned no instance",
		)
		return
	}
	s.mu.Lock()
	record.instance = instance
	record.status = "connected"
	record.health = Health{
		Status:     orchestrator.EngineHealthHealthy,
		ObservedAt: time.Now().UTC(),
	}
	appendInstanceEvent(record, orchestrator.EngineEvent{
		Type:       "instance.connected",
		Status:     "connected",
		OccurredAt: time.Now().UTC(),
	})
	value := describeInstance(request.InstanceID, record)
	s.mu.Unlock()
	if source, ok := instance.(orchestrator.EngineEventSource); ok {
		if events := source.EngineEvents(); events != nil {
			go s.watchInstanceEvents(request.InstanceID, record, events)
		}
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Service) handleInstance(w http.ResponseWriter, r *http.Request) {
	relative := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/instances/"), "/")
	parts := strings.Split(relative, "/")
	id := parts[0]
	if !instanceIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		s.handleInstanceEvents(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "call" {
		s.handleInstanceCall(w, r, id)
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	record, ok := s.instances[id]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "instance_not_found", "interaction instance was not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if record.status != "connected" {
			value := describeInstance(id, record)
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, value)
			return
		}
		instance := record.instance
		s.mu.Unlock()
		health := probeInstanceHealth(r.Context(), instance)
		s.mu.Lock()
		current, exists := s.instances[id]
		if !exists || current != record {
			s.mu.Unlock()
			writeError(w, http.StatusNotFound, "instance_not_found", "interaction instance was not found")
			return
		}
		if record.redact != nil {
			health.Message = record.redact(health.Message)
		}
		updateInstanceHealth(record, health)
		value := describeInstance(id, record)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, value)
	case http.MethodDelete:
		if record.status == "disconnecting" {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "instance_disconnecting", "interaction instance is disconnecting")
			return
		}
		record.status = "disconnecting"
		record.health = Health{
			Status:     orchestrator.EngineHealthStopping,
			Message:    "interaction engine is disconnecting",
			ObservedAt: time.Now().UTC(),
		}
		appendInstanceEvent(record, orchestrator.EngineEvent{
			Type:       "instance.disconnecting",
			Status:     "disconnecting",
			OccurredAt: time.Now().UTC(),
		})
		instance := record.instance
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		var err error
		if instance != nil {
			err = instance.Disconnect(ctx)
		}
		if record.redact != nil && err != nil {
			err = errors.New(record.redact(err.Error()))
		}
		if record.release != nil {
			err = errors.Join(err, record.release(ctx))
		}
		s.mu.Lock()
		delete(s.instances, id)
		s.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusBadGateway, "engine_disconnect_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		s.mu.Unlock()
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Service) handleInstanceEvents(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	after, err := parseCursor(r.URL.Query().Get("after"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_cursor", err.Error())
		return
	}
	limit, err := parseEventLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_limit", err.Error())
		return
	}

	s.mu.Lock()
	record, ok := s.instances[id]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "instance_not_found", "interaction instance was not found")
		return
	}
	if after > record.nextEvent {
		s.mu.Unlock()
		writeError(w, http.StatusUnprocessableEntity, "invalid_cursor", "event cursor is ahead of the instance")
		return
	}
	if len(record.events) != 0 {
		first, _ := strconv.ParseUint(record.events[0].Cursor, 10, 64)
		if after+1 < first {
			s.mu.Unlock()
			writeError(w, http.StatusGone, "cursor_expired", "event cursor is older than retained history")
			return
		}
	}
	events := make([]InstanceEvent, 0, limit)
	cursor := record.nextEvent
	for _, event := range record.events {
		sequence, _ := strconv.ParseUint(event.Cursor, 10, 64)
		if sequence <= after {
			continue
		}
		events = append(events, event)
		cursor = sequence
		if len(events) == limit {
			break
		}
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, InstanceEventBatch{
		APIVersion: APIVersion,
		InstanceID: id,
		Events:     events,
		Cursor:     strconv.FormatUint(cursor, 10),
	})
}

func (s *Service) handleInstanceCall(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var request CallRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid call request")
		return
	}
	if strings.TrimSpace(request.Method) == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "method is required")
		return
	}
	if len(request.Params) == 0 {
		request.Params = json.RawMessage(`{}`)
	}
	if !json.Valid(request.Params) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "params must be valid JSON")
		return
	}

	s.mu.Lock()
	record, ok := s.instances[id]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "instance_not_found", "interaction instance was not found")
		return
	}
	if record.status != "connected" {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "instance_not_connected", "interaction instance is not connected")
		return
	}
	caller, ok := record.instance.(bridge.Caller)
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotImplemented, "calls_unsupported", "interaction engine does not accept bridge calls")
		return
	}
	callCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := caller.Call(callCtx, request.Method, request.Params)
	if err != nil {
		if record.redact != nil {
			err = errors.New(record.redact(err.Error()))
		}
		writeError(w, http.StatusBadGateway, "engine_call_failed", err.Error())
		return
	}
	if record.redactJSON != nil {
		result = record.redactJSON(result)
	}
	if !json.Valid(result) {
		writeError(w, http.StatusBadGateway, "invalid_engine_result", "interaction engine returned invalid JSON")
		return
	}
	writeJSON(w, http.StatusOK, CallResponse{
		APIVersion: APIVersion,
		InstanceID: id,
		Result:     result,
	})
}

func (s *Service) watchInstanceEvents(
	id string,
	record *runningInstance,
	events <-chan orchestrator.EngineEvent,
) {
	for event := range events {
		s.mu.Lock()
		current, ok := s.instances[id]
		if !ok || current != record {
			s.mu.Unlock()
			return
		}
		if record.redact != nil {
			event.Message = record.redact(event.Message)
		}
		if record.status == "disconnecting" &&
			(event.Status == "exited" || event.Status == "failed") {
			event.Type = "instance.disconnected"
			event.Status = "disconnected"
			event.Message = ""
			record.status = "stopped"
			record.health = Health{
				Status:     orchestrator.EngineHealthStopped,
				ObservedAt: time.Now().UTC(),
			}
		} else if event.Status != "" {
			record.status = event.Status
			if event.Status == "failed" {
				record.health = Health{
					Status:     orchestrator.EngineHealthUnhealthy,
					Message:    event.Message,
					ObservedAt: event.OccurredAt,
				}
			} else if event.Status == "exited" {
				record.health = Health{
					Status:     orchestrator.EngineHealthStopped,
					ObservedAt: event.OccurredAt,
				}
			}
		}
		appendInstanceEvent(record, event)
		s.mu.Unlock()
	}
}

func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	type managedInstance struct {
		instance orchestrator.EngineInstance
		release  func(context.Context) error
		redact   func(string) string
	}
	values := make([]managedInstance, 0, len(s.instances))
	for _, record := range s.instances {
		values = append(values, managedInstance{instance: record.instance, release: record.release, redact: record.redact})
	}
	s.instances = make(map[string]*runningInstance)
	s.mu.Unlock()
	var problems []error
	for index := len(values) - 1; index >= 0; index-- {
		value := values[index]
		if value.instance != nil {
			if err := value.instance.Disconnect(ctx); err != nil {
				if value.redact != nil {
					err = errors.New(value.redact(err.Error()))
				}
				problems = append(problems, err)
			}
		}
		if value.release != nil {
			if err := value.release(ctx); err != nil {
				problems = append(problems, errors.New("release target connection material"))
			}
		}
	}
	return errors.Join(problems...)
}

func validateConnectRequest(request ConnectRequest) error {
	if request.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion must be %q", APIVersion)
	}
	if !instanceIDPattern.MatchString(request.InstanceID) {
		return errors.New("instanceId must be a lowercase DNS-style name")
	}
	if strings.TrimSpace(request.Engine.Adapter) == "" {
		return errors.New("engine.adapter is required")
	}
	for _, capability := range request.Engine.RequiredCapabilities {
		if !handleNamePattern.MatchString(capability) {
			return fmt.Errorf("invalid required engine capability %q", capability)
		}
	}
	if request.Target.APIVersion != "" && request.Target.APIVersion != TargetAPIVersion {
		return fmt.Errorf("target.apiVersion must be %q", TargetAPIVersion)
	}
	if request.Target.APIVersion != "" && request.Target.TargetID == "" {
		return errors.New("target.targetId is required when target.apiVersion is supplied")
	}
	if request.Target.TargetID != "" && !targetIDPattern.MatchString(request.Target.TargetID) {
		return errors.New("target.targetId is invalid")
	}
	if strings.TrimSpace(request.Target.Kind) == "" {
		return errors.New("target.kind is required")
	}
	if len(request.Engine.Options) != 0 {
		var object map[string]any
		if err := json.Unmarshal(request.Engine.Options, &object); err != nil || object == nil {
			return errors.New("engine.options must be a JSON object")
		}
	}
	if len(request.Target.Endpoints) > 64 {
		return errors.New("target endpoints must not exceed 64 entries")
	}
	endpointNames := make(map[string]struct{}, len(request.Target.Endpoints))
	for index, endpoint := range request.Target.Endpoints {
		if !handleNamePattern.MatchString(endpoint.Name) {
			return fmt.Errorf("invalid target endpoint name %q", endpoint.Name)
		}
		if _, exists := endpointNames[endpoint.Name]; exists {
			return fmt.Errorf("duplicate target endpoint name %q", endpoint.Name)
		}
		endpointNames[endpoint.Name] = struct{}{}
		if strings.TrimSpace(endpoint.Protocol) == "" {
			return fmt.Errorf("target endpoint %d protocol is required", index)
		}
		if strings.TrimSpace(endpoint.URL) == "" || len(endpoint.URL) > 4096 || strings.ContainsRune(endpoint.URL, '\x00') {
			return fmt.Errorf("target endpoint %q URL is required", endpoint.Name)
		}
		if endpoint.CredentialRef != "" && !handleNamePattern.MatchString(endpoint.CredentialRef) {
			return fmt.Errorf("target endpoint %q credentialRef is invalid", endpoint.Name)
		}
		if endpoint.TLSRef != "" && !handleNamePattern.MatchString(endpoint.TLSRef) {
			return fmt.Errorf("target endpoint %q tlsRef is invalid", endpoint.Name)
		}
		if endpoint.Audience != "" && endpoint.Audience != "engine" && endpoint.Audience != "target" {
			return fmt.Errorf("target endpoint %q audience must be engine or target", endpoint.Name)
		}
		if err := validateStringMetadata("target endpoint "+endpoint.Name+" metadata", endpoint.Metadata); err != nil {
			return err
		}
	}
	for name, value := range request.Target.Handles {
		if !handleNamePattern.MatchString(name) {
			return fmt.Errorf("invalid handle name %q", name)
		}
		if value == "" {
			return fmt.Errorf("handle %q is empty", name)
		}
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("handle %q contains a null byte", name)
		}
	}
	if err := validateStringMetadata("target metadata", request.Target.Metadata); err != nil {
		return err
	}
	return nil
}

func validateStringMetadata(label string, values map[string]string) error {
	for name, value := range values {
		if !handleNamePattern.MatchString(name) {
			return fmt.Errorf("%s key %q is invalid", label, name)
		}
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("%s value %q contains a null byte", label, name)
		}
	}
	return nil
}

func describeInstance(id string, record *runningInstance) Instance {
	capabilities := []string{}
	if provider, ok := record.instance.(orchestrator.EngineCapabilityProvider); ok {
		capabilities = stableCapabilities(provider.EngineCapabilities())
	}
	return Instance{
		APIVersion:   APIVersion,
		InstanceID:   id,
		Adapter:      record.adapter,
		Status:       record.status,
		Health:       record.health,
		Capabilities: capabilities,
	}
}

func probeInstanceHealth(
	ctx context.Context,
	instance orchestrator.EngineInstance,
) orchestrator.EngineHealth {
	provider, ok := instance.(orchestrator.EngineHealthProvider)
	if !ok {
		return orchestrator.EngineHealth{
			Status:     orchestrator.EngineHealthUnknown,
			Message:    "interaction engine does not implement an active health probe",
			ObservedAt: time.Now().UTC(),
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return provider.EngineHealth(probeCtx)
}

func updateInstanceHealth(record *runningInstance, health orchestrator.EngineHealth) {
	if health.ObservedAt.IsZero() {
		health.ObservedAt = time.Now().UTC()
	}
	changed := record.health.Status != health.Status || record.health.Message != health.Message
	record.health = Health{
		Status:     health.Status,
		Message:    health.Message,
		ObservedAt: health.ObservedAt,
	}
	if changed {
		appendInstanceEvent(record, orchestrator.EngineEvent{
			Type:       "engine.health." + health.Status,
			Message:    health.Message,
			OccurredAt: health.ObservedAt,
		})
	}
}

func appendInstanceEvent(record *runningInstance, event orchestrator.EngineEvent) {
	record.nextEvent++
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	record.events = append(record.events, InstanceEvent{
		Cursor:     strconv.FormatUint(record.nextEvent, 10),
		Type:       event.Type,
		Status:     event.Status,
		Message:    event.Message,
		OccurredAt: event.OccurredAt,
	})
	if len(record.events) > eventHistoryLimit {
		record.events = append([]InstanceEvent(nil), record.events[len(record.events)-eventHistoryLimit:]...)
	}
}

func parseCursor(value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.New("event cursor must be a non-negative integer")
	}
	return cursor, nil
}

func parseEventLimit(value string) (int, error) {
	if value == "" {
		return 100, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 || limit > eventHistoryLimit {
		return 0, fmt.Errorf("event limit must be between 1 and %d", eventHistoryLimit)
	}
	return limit, nil
}

func cloneValues(value map[string]string) map[string]string {
	cloned := make(map[string]string, len(value))
	for name, item := range value {
		cloned[name] = item
	}
	return cloned
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Code: code, Message: message})
}
