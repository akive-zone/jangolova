package orchestrator

import (
	"context"
	"time"

	"jangolova/internal/manifest"
)

type EngineInstance interface {
	Stop(context.Context) error
}

// EngineEnvironment contains placement-resolved values supplied by the caller.
type EngineEnvironment map[string]string

// EngineHandles contains named opaque runtime values supplied and owned by the
// caller. An adapter may interpret a handle it understands, but must not assume
// ownership of the resource identified by that value.
type EngineHandles map[string]string

// EngineRuntime contains the caller-resolved inputs for one engine launch.
// Jangolova consumes these inputs without creating the display or placement
// resources that produced them.
type EngineRuntime struct {
	Environment EngineEnvironment
	Handles     EngineHandles
}

// EngineEvent reports an engine lifecycle or health transition. Status, when
// present, becomes the provider-visible instance status.
type EngineEvent struct {
	Type       string
	Status     string
	Message    string
	OccurredAt time.Time
}

// EngineEventSource is optionally implemented by instances that can report
// readiness, health, or unexpected termination after Start returns. The
// channel must close when the instance stops producing events.
type EngineEventSource interface {
	EngineEvents() <-chan EngineEvent
}

type EngineHealth struct {
	Status     string
	Message    string
	ObservedAt time.Time
}

const (
	EngineHealthStarting  = "starting"
	EngineHealthStopping  = "stopping"
	EngineHealthHealthy   = "healthy"
	EngineHealthUnhealthy = "unhealthy"
	EngineHealthStopped   = "stopped"
	EngineHealthUnknown   = "unknown"
)

// EngineHealthProvider is optionally implemented by instances that can probe
// their current engine-local health without assuming display ownership.
type EngineHealthProvider interface {
	EngineHealth(context.Context) EngineHealth
}

type EngineInspection struct {
	Available    bool
	Capabilities []string
	Message      string
}

// EngineInspector reports adapter availability and engine-local capabilities.
type EngineInspector interface {
	InspectEngine(context.Context) EngineInspection
}

type EngineAdapter interface {
	Start(context.Context, manifest.EngineSpec, EngineRuntime) (EngineInstance, error)
}
