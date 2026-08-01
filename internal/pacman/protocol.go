// Package pacman defines the transport-neutral semantic contract shared by
// Jangolova, Unity, and future Unreal implementations.
package pacman

import (
	"encoding/json"
	"time"
)

const ProtocolVersion = "jangolova.pacman/v1alpha1"

const (
	MethodHello        = "hello"
	MethodCapabilities = "capabilities"
	MethodDescribe     = "describe"
	MethodAct          = "act"
	MethodEvents       = "events"
	MethodHealth       = "health"
)

type ResourceKind string

const (
	KindScene     ResourceKind = "scene"
	KindObject    ResourceKind = "object"
	KindUI        ResourceKind = "ui"
	KindCamera    ResourceKind = "camera"
	KindMaterial  ResourceKind = "material"
	KindAnimation ResourceKind = "animation"
	KindTimeline  ResourceKind = "timeline"
	KindArtifact  ResourceKind = "artifact"
	KindEvent     ResourceKind = "event"
)

type Implementation struct {
	Engine  string `json:"engine"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Hello struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Implementation  Implementation `json:"implementation"`
	Features        []string       `json:"features,omitempty"`
}

type Capability struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Effect      string          `json:"effect"`
	TargetKinds []ResourceKind  `json:"targetKinds,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Resource is one explicitly registered semantic presentation resource. ID is
// stable across observations and has the form "kind:project-defined-name".
type Resource struct {
	ID         string          `json:"id"`
	Kind       ResourceKind    `json:"kind"`
	Label      string          `json:"label,omitempty"`
	Properties json.RawMessage `json:"properties,omitempty"`
}

type Description struct {
	Revision  string     `json:"revision"`
	Resources []Resource `json:"resources"`
}

type ActionRequest struct {
	Name     string          `json:"name"`
	TargetID string          `json:"targetId,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	SourceID   string          `json:"sourceId,omitempty"`
	OccurredAt time.Time       `json:"occurredAt"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type EventQuery struct {
	After string   `json:"after,omitempty"`
	Types []string `json:"types,omitempty"`
	Limit int      `json:"limit,omitempty"`
}

type EventBatch struct {
	Events []Event `json:"events"`
	Cursor string  `json:"cursor"`
}

type Health struct {
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}
