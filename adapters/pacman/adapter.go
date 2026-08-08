// Package pacman attaches Jangolova's transport-neutral Pacman semantics to a
// caller-owned target. The built-in binding is pacman-ws; other bindings can
// provide the same Connector contract without changing the wire methods.
package pacman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"jangolova/internal/bridge"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
	protocol "jangolova/internal/pacman"
)

type Adapter struct {
	Connector Connector
}

type instance struct {
	callMu       sync.Mutex
	stateMu      sync.RWMutex
	transport    Transport
	connector    Connector
	endpoint     orchestrator.TargetEndpoint
	capabilities []string
	actions      map[string]struct{}
	disconnected bool
	events       chan orchestrator.EngineEvent
	renewalStop  chan struct{}
	renewalOnce  sync.Once
	renewalWG    sync.WaitGroup
}

var _ orchestrator.EngineAdapter = Adapter{}
var _ orchestrator.EngineInspector = Adapter{}
var _ orchestrator.EngineInstance = (*instance)(nil)
var _ orchestrator.EngineHealthProvider = (*instance)(nil)
var _ orchestrator.EngineCapabilityProvider = (*instance)(nil)
var _ orchestrator.EngineEventSource = (*instance)(nil)
var _ bridge.Caller = (*instance)(nil)

func (a Adapter) connector() Connector {
	if a.Connector != nil {
		return a.Connector
	}
	return WebSocketConnector{}
}

func (a Adapter) InspectEngine(context.Context) orchestrator.EngineInspection {
	connector := a.connector()
	return orchestrator.EngineInspection{Available: true, Capabilities: []string{
		"act", "capabilities", "describe", "events", "health", "target." + connector.Protocol(),
	}}
}

func (a Adapter) Connect(ctx context.Context, _ manifest.EngineSpec, target orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	if target.Kind != "native-presentation" && target.Kind != "unity" && target.Kind != "unreal" {
		return nil, errors.New("Pacman requires target.kind native-presentation, unity, or unreal")
	}
	connector := a.connector()
	endpoint, ok := target.Endpoint(connector.Protocol())
	if !ok {
		return nil, fmt.Errorf("Pacman requires a caller-owned %s endpoint", connector.Protocol())
	}
	transport, err := connector.Connect(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	report, err := protocol.ValidateConformance(ctx, transport)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	running := &instance{
		transport: transport, connector: connector, endpoint: endpoint,
		events: make(chan orchestrator.EngineEvent, 8), renewalStop: make(chan struct{}),
	}
	running.applyReport(report)
	running.emit(orchestrator.EngineEvent{Type: "pacman.connected", Status: "connected", OccurredAt: time.Now().UTC()})
	if endpoint.Connection != nil {
		updates := endpoint.Connection.Updates()
		connectedRevision := endpoint.Connection.Snapshot().Revision
		running.renewalWG.Add(1)
		go func() {
			defer running.renewalWG.Done()
			running.watchConnectionMaterial(updates, connectedRevision)
		}()
	}
	return running, nil
}

func (i *instance) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case protocol.MethodHello, protocol.MethodCapabilities, protocol.MethodDescribe, protocol.MethodAct, protocol.MethodEvents, protocol.MethodHealth:
	default:
		return nil, fmt.Errorf("unsupported Pacman method %q", method)
	}
	if method == protocol.MethodAct {
		var action protocol.ActionRequest
		if err := json.Unmarshal(params, &action); err != nil {
			return nil, errors.New("Pacman action request is invalid")
		}
		decision, err := i.Authorize(ctx, orchestrator.AuthorizeRequest{
			TargetID:     action.TargetID,
			Action:       action.Name,
			Input:        action.Input,
			Capabilities: nil,
		})
		if err != nil || !decision.Authorized {
			if err == nil {
				err = errors.New(decision.Reason)
			}
			return nil, err
		}
	}
	i.callMu.Lock()
	defer i.callMu.Unlock()
	i.stateMu.RLock()
	transport := i.transport
	disconnected := i.disconnected
	i.stateMu.RUnlock()
	if disconnected || transport == nil {
		return nil, errors.New("Pacman connection is disconnected")
	}
	return transport.Call(ctx, method, params)
}

func (i *instance) Disconnect(ctx context.Context) error {
	i.renewalOnce.Do(func() { close(i.renewalStop) })
	i.renewalWG.Wait()
	i.callMu.Lock()
	i.stateMu.Lock()
	if i.disconnected {
		i.stateMu.Unlock()
		i.callMu.Unlock()
		return nil
	}
	i.disconnected = true
	transport := i.transport
	i.transport = nil
	i.stateMu.Unlock()
	i.callMu.Unlock()
	var err error
	if transport != nil {
		err = transport.Close()
	}
	i.emit(orchestrator.EngineEvent{Type: "pacman.disconnected", Status: "disconnected", OccurredAt: time.Now().UTC()})
	close(i.events)
	return err
}

func (i *instance) Authorize(ctx context.Context, request orchestrator.AuthorizeRequest) (orchestrator.AuthorizeDecision, error) {
	action := protocol.ActionRequest{
		Name:     request.Action,
		TargetID: request.TargetID,
		Input:    request.Input,
	}
	capabilities := request.Capabilities
	if len(capabilities) == 0 {
		i.stateMu.RLock()
		capabilities = append([]string(nil), i.capabilities...)
		i.stateMu.RUnlock()
	}
	if err := protocol.ValidateActionRequest(action, actionSet(capabilities)); err != nil {
		return orchestrator.AuthorizeDecision{Authorized: false}, err
	}
	return orchestrator.AuthorizeDecision{Authorized: true}, nil
}

func actionSet(capabilities []string) map[string]struct{} {
	set := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if strings.TrimSpace(capability) != "" {
			set[capability] = struct{}{}
		}
	}
	return set
}

func (i *instance) EngineCapabilities() []string {
	i.stateMu.RLock()
	defer i.stateMu.RUnlock()
	return append([]string(nil), i.capabilities...)
}

func (i *instance) EngineEvents() <-chan orchestrator.EngineEvent { return i.events }

func (i *instance) EngineHealth(ctx context.Context) orchestrator.EngineHealth {
	raw, err := i.Call(ctx, protocol.MethodHealth, json.RawMessage(`{}`))
	if err != nil {
		return orchestrator.EngineHealth{Status: orchestrator.EngineHealthUnhealthy, Message: err.Error(), ObservedAt: time.Now().UTC()}
	}
	var health protocol.Health
	if json.Unmarshal(raw, &health) != nil {
		return orchestrator.EngineHealth{Status: orchestrator.EngineHealthUnhealthy, Message: "invalid Pacman health response", ObservedAt: time.Now().UTC()}
	}
	status := orchestrator.EngineHealthHealthy
	if health.Status != "ready" {
		status = orchestrator.EngineHealthUnhealthy
	}
	return orchestrator.EngineHealth{Status: status, Message: health.Message, ObservedAt: health.ObservedAt}
}

func (i *instance) applyReport(report protocol.ConformanceReport) {
	capabilities := []string{"act", "capabilities", "describe", "events", "health", "target." + i.connector.Protocol()}
	actions := make(map[string]struct{}, len(report.Capabilities))
	for _, capability := range report.Capabilities {
		capabilities = append(capabilities, capability.Name)
		actions[capability.Name] = struct{}{}
	}
	sort.Strings(capabilities)
	i.stateMu.Lock()
	i.capabilities = capabilities
	i.actions = actions
	i.stateMu.Unlock()
}

func (i *instance) watchConnectionMaterial(updates <-chan uint64, connectedRevision uint64) {
	for {
		current := i.endpoint.Connection.Snapshot().Revision
		if current > connectedRevision {
			if !i.reconnectWithMaterial(current) {
				return
			}
			connectedRevision = current
			continue
		}
		select {
		case <-i.renewalStop:
			return
		case revision, open := <-updates:
			if !open {
				return
			}
			if revision > connectedRevision {
				if !i.reconnectWithMaterial(revision) {
					return
				}
				connectedRevision = revision
			}
		}
	}
}

func (i *instance) reconnectWithMaterial(revision uint64) bool {
	reported := false
	for {
		select {
		case <-i.renewalStop:
			return false
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		candidate, err := i.connector.Connect(ctx, i.endpoint)
		if err == nil {
			var report protocol.ConformanceReport
			report, err = protocol.ValidateConformance(ctx, candidate)
			if err == nil {
				i.callMu.Lock()
				i.stateMu.Lock()
				if i.disconnected {
					i.stateMu.Unlock()
					i.callMu.Unlock()
					cancel()
					_ = candidate.Close()
					return false
				}
				previous := i.transport
				i.transport = candidate
				i.stateMu.Unlock()
				i.callMu.Unlock()
				cancel()
				if previous != nil {
					_ = previous.Close()
				}
				i.applyReport(report)
				i.endpoint.Connection.Acknowledge(revision)
				i.emit(orchestrator.EngineEvent{Type: "pacman.connection.renewed", Status: "connected", OccurredAt: time.Now().UTC()})
				return true
			}
			_ = candidate.Close()
		}
		cancel()
		if !reported {
			reported = true
			i.emit(orchestrator.EngineEvent{Type: "pacman.connection.renewal_failed", Status: "unhealthy", Message: "Pacman transport could not apply renewed connection material", OccurredAt: time.Now().UTC()})
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-timer.C:
		case <-i.renewalStop:
			timer.Stop()
			return false
		}
	}
}

func (i *instance) emit(event orchestrator.EngineEvent) {
	select {
	case i.events <- event:
	default:
	}
}
