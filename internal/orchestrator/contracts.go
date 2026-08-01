package orchestrator

import (
	"context"
	"time"

	"jangolova/internal/manifest"
)

// EngineInstance is a Jangolova-owned interaction session attached to a
// caller-owned target. Disconnect must release only Jangolova resources; it
// must never terminate the target runtime.
type EngineInstance interface {
	Disconnect(context.Context) error
}

// TargetEndpoint identifies a caller-owned service that an interaction engine
// can attach to, such as a Chromium CDP endpoint or a Unity bridge endpoint.
type TargetEndpoint struct {
	Name          string
	Protocol      string
	URL           string
	CredentialRef string
	TLSRef        string
	Audience      string
	Metadata      map[string]string
}

// EngineHandles contains opaque target handles supplied and owned by the
// caller. An adapter may interpret a handle it understands, but never owns the
// underlying resource.
type EngineHandles map[string]string

// EngineTarget is the complete caller-resolved target description. A person,
// native launcher, container/VM manager, Xallet, or another target owner may
// supply it. Location and lifecycle are deliberately outside this contract.
type EngineTarget struct {
	APIVersion string
	TargetID   string
	Kind       string
	Endpoints  []TargetEndpoint
	Handles    EngineHandles
	Metadata   map[string]string
}

func (t EngineTarget) Endpoint(protocol string) (TargetEndpoint, bool) {
	for _, endpoint := range t.Endpoints {
		if endpoint.Protocol == protocol {
			return endpoint, true
		}
	}
	return TargetEndpoint{}, false
}

// EngineEvent reports an interaction-session lifecycle or health transition.
type EngineEvent struct {
	Type       string
	Status     string
	Message    string
	OccurredAt time.Time
}

type EngineEventSource interface {
	EngineEvents() <-chan EngineEvent
}

type EngineHealth struct {
	Status     string
	Message    string
	ObservedAt time.Time
}

const (
	EngineHealthStarting  = "connecting"
	EngineHealthStopping  = "disconnecting"
	EngineHealthHealthy   = "connected"
	EngineHealthUnhealthy = "unhealthy"
	EngineHealthStopped   = "disconnected"
	EngineHealthUnknown   = "unknown"
)

type EngineHealthProvider interface {
	EngineHealth(context.Context) EngineHealth
}

type EngineCapabilityProvider interface {
	EngineCapabilities() []string
}

type EngineInspection struct {
	Available    bool
	Capabilities []string
	Message      string
}

// EngineInspector reports interaction-adapter availability and capabilities.
type EngineInspector interface {
	InspectEngine(context.Context) EngineInspection
}

// EngineAdapter connects a Jangolova interaction engine to a caller-owned
// target. It does not launch, stop, or otherwise provision that target.
type EngineAdapter interface {
	Connect(context.Context, manifest.EngineSpec, EngineTarget) (EngineInstance, error)
}
