// Package engineprovider implements Jangolova's deployment-neutral
// display-engine provider contract. Xallet is one supported client.
package engineprovider

import (
	"encoding/json"
	"time"
)

const APIVersion = "jangolova.engine/v1alpha1"

type EngineDescriptor struct {
	Adapter      string   `json:"adapter"`
	Available    bool     `json:"available"`
	Capabilities []string `json:"capabilities"`
	Message      string   `json:"message,omitempty"`
}

type EngineSpec struct {
	Adapter string          `json:"adapter"`
	Source  string          `json:"source,omitempty"`
	Options json.RawMessage `json:"options,omitempty"`
}

type LaunchRequest struct {
	APIVersion  string            `json:"apiVersion"`
	InstanceID  string            `json:"instanceId"`
	Engine      EngineSpec        `json:"engine"`
	Environment map[string]string `json:"environment,omitempty"`
	Handles     map[string]string `json:"handles,omitempty"`
}

type Endpoint struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	URL        string `json:"url,omitempty"`
	TargetPort int    `json:"targetPort,omitempty"`
	Visibility string `json:"visibility"`
}

type Instance struct {
	APIVersion string     `json:"apiVersion"`
	InstanceID string     `json:"instanceId"`
	Adapter    string     `json:"adapter"`
	Status     string     `json:"status"`
	Health     Health     `json:"health"`
	Endpoints  []Endpoint `json:"endpoints"`
}

type Health struct {
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

type InstanceEvent struct {
	Cursor     string    `json:"cursor,omitempty"`
	Type       string    `json:"type"`
	Status     string    `json:"status,omitempty"`
	Message    string    `json:"message,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}

type InstanceEventBatch struct {
	APIVersion string          `json:"apiVersion"`
	InstanceID string          `json:"instanceId"`
	Events     []InstanceEvent `json:"events"`
	Cursor     string          `json:"cursor"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// EndpointProvider is implemented by engine instances that expose typed
// control endpoints to their caller.
type EndpointProvider interface {
	EngineEndpoints() []Endpoint
}
