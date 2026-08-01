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

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

var instanceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var handleNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,127}$`)

const eventHistoryLimit = 256

type Service struct {
	mu        sync.Mutex
	registry  *orchestrator.Registry
	token     string
	instances map[string]*runningInstance
}

type runningInstance struct {
	adapter   string
	status    string
	health    Health
	instance  orchestrator.EngineInstance
	events    []InstanceEvent
	nextEvent uint64
}

func NewService(
	registry *orchestrator.Registry,
	token string,
) (*Service, error) {
	if registry == nil {
		return nil, errors.New("engine provider registry is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("engine provider token is required")
	}
	return &Service{
		registry:  registry,
		token:     token,
		instances: make(map[string]*runningInstance),
	}, nil
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
		"service":    "jangolova-engine-provider",
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
	var request LaunchRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid launch request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "launch request must contain one JSON value")
		return
	}
	if err := validateLaunchRequest(request); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	adapter, ok := s.registry.Engine(request.Engine.Adapter)
	if !ok {
		writeError(w, http.StatusNotFound, "engine_not_found", "engine adapter is not registered")
		return
	}

	s.mu.Lock()
	if _, exists := s.instances[request.InstanceID]; exists {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "instance_exists", "engine instance already exists")
		return
	}
	record := &runningInstance{
		adapter: request.Engine.Adapter,
		status:  "starting",
		health: Health{
			Status:     orchestrator.EngineHealthStarting,
			ObservedAt: time.Now().UTC(),
		},
	}
	appendInstanceEvent(record, orchestrator.EngineEvent{
		Type:       "instance.starting",
		Status:     "starting",
		OccurredAt: time.Now().UTC(),
	})
	s.instances[request.InstanceID] = record
	s.mu.Unlock()

	instance, err := adapter.Start(r.Context(), manifest.EngineSpec{
		Adapter: request.Engine.Adapter,
		Source:  request.Engine.Source,
		Options: request.Engine.Options,
	}, orchestrator.EngineRuntime{
		Environment: orchestrator.EngineEnvironment(cloneValues(request.Environment)),
		Handles:     orchestrator.EngineHandles(cloneValues(request.Handles)),
	})
	if err != nil {
		s.mu.Lock()
		delete(s.instances, request.InstanceID)
		s.mu.Unlock()
		writeError(w, http.StatusBadGateway, "engine_start_failed", err.Error())
		return
	}
	if instance == nil {
		s.mu.Lock()
		delete(s.instances, request.InstanceID)
		s.mu.Unlock()
		writeError(
			w,
			http.StatusBadGateway,
			"engine_start_failed",
			"engine adapter returned no instance",
		)
		return
	}
	s.mu.Lock()
	record.instance = instance
	record.status = "running"
	record.health = Health{
		Status:     orchestrator.EngineHealthHealthy,
		ObservedAt: time.Now().UTC(),
	}
	appendInstanceEvent(record, orchestrator.EngineEvent{
		Type:       "instance.ready",
		Status:     "running",
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
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	record, ok := s.instances[id]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "instance_not_found", "engine instance was not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if record.status != "running" {
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
			writeError(w, http.StatusNotFound, "instance_not_found", "engine instance was not found")
			return
		}
		updateInstanceHealth(record, health)
		value := describeInstance(id, record)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, value)
	case http.MethodDelete:
		if record.status == "stopping" {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "instance_stopping", "engine instance is stopping")
			return
		}
		record.status = "stopping"
		record.health = Health{
			Status:     orchestrator.EngineHealthStopping,
			Message:    "engine is stopping",
			ObservedAt: time.Now().UTC(),
		}
		appendInstanceEvent(record, orchestrator.EngineEvent{
			Type:       "instance.stopping",
			Status:     "stopping",
			OccurredAt: time.Now().UTC(),
		})
		instance := record.instance
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		var err error
		if instance != nil {
			err = instance.Stop(ctx)
		}
		s.mu.Lock()
		delete(s.instances, id)
		s.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusBadGateway, "engine_stop_failed", err.Error())
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
		writeError(w, http.StatusNotFound, "instance_not_found", "engine instance was not found")
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
		if record.status == "stopping" &&
			(event.Status == "exited" || event.Status == "failed") {
			event.Type = "instance.stopped"
			event.Status = "stopped"
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
	values := make([]orchestrator.EngineInstance, 0, len(s.instances))
	for _, record := range s.instances {
		if record.instance != nil {
			values = append(values, record.instance)
		}
	}
	s.instances = make(map[string]*runningInstance)
	s.mu.Unlock()
	var problems []error
	for index := len(values) - 1; index >= 0; index-- {
		if err := values[index].Stop(ctx); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func validateLaunchRequest(request LaunchRequest) error {
	if request.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion must be %q", APIVersion)
	}
	if !instanceIDPattern.MatchString(request.InstanceID) {
		return errors.New("instanceId must be a lowercase DNS-style name")
	}
	if strings.TrimSpace(request.Engine.Adapter) == "" {
		return errors.New("engine.adapter is required")
	}
	if len(request.Engine.Options) != 0 {
		var object map[string]any
		if err := json.Unmarshal(request.Engine.Options, &object); err != nil || object == nil {
			return errors.New("engine.options must be a JSON object")
		}
	}
	for name, value := range request.Environment {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return fmt.Errorf("invalid environment variable name %q", name)
		}
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("environment variable %q contains a null byte", name)
		}
	}
	for name, value := range request.Handles {
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
	return nil
}

func describeInstance(id string, record *runningInstance) Instance {
	endpoints := []Endpoint{}
	if provider, ok := record.instance.(EndpointProvider); ok {
		endpoints = append(endpoints, provider.EngineEndpoints()...)
	}
	return Instance{
		APIVersion: APIVersion,
		InstanceID: id,
		Adapter:    record.adapter,
		Status:     record.status,
		Health:     record.health,
		Endpoints:  endpoints,
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
			Message:    "engine adapter does not implement an active health probe",
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
