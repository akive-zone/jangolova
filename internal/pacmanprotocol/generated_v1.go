// Code generated from protocol/pacman/v1/protocol.schema.json; DO NOT EDIT.
// Schema SHA-256: 869e8ccce2b3504e69ec42d439703f779c55b2ee6e1ed24f3f88768c91a8b10d

package pacmanprotocol

import (
	"context"
	"encoding/json"
	"time"
)

const ProtocolVersion = "jangolova.pacman/v1"

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

type Effect string

const (
	EffectRead     Effect = "read"
	EffectWrite    Effect = "write"
	EffectExternal Effect = "external"
)

type Capability struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Effect      Effect          `json:"effect"`
	TargetKinds []ResourceKind  `json:"targetKinds,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

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

type Health struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
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

type EventBatch struct {
	Cursor string `json:"cursor"`
	Events []Event `json:"events"`
}

type CallRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type CallResponse struct {
	ProtocolVersion string          `json:"protocolVersion"`
	InstanceID      string          `json:"instanceId,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
}

type Transport interface {
	Call(context.Context, CallRequest) (CallResponse, error)
}

type Client struct {
	Transport Transport
}

func (c Client) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	response, err := c.Transport.Call(ctx, CallRequest{Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if len(response.Result) == 0 {
		return json.RawMessage("null"), nil
	}
	return response.Result, nil
}
