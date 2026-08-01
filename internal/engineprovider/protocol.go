// Package engineprovider implements the provider-neutral interaction-engine
// contract. Jangolova is one implementation and Xallet is one supported
// target provider.
package engineprovider

import (
	"encoding/json"
	"time"
)

const APIVersion = "interaction.engine/v1alpha1"

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

type TargetEndpoint struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
}

type Target struct {
	Kind      string            `json:"kind"`
	Endpoints []TargetEndpoint  `json:"endpoints,omitempty"`
	Handles   map[string]string `json:"handles,omitempty"`
}

type ConnectRequest struct {
	APIVersion string     `json:"apiVersion"`
	InstanceID string     `json:"instanceId"`
	Engine     EngineSpec `json:"engine"`
	Target     Target     `json:"target"`
}

type Instance struct {
	APIVersion   string   `json:"apiVersion"`
	InstanceID   string   `json:"instanceId"`
	Adapter      string   `json:"adapter"`
	Status       string   `json:"status"`
	Health       Health   `json:"health"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type Health struct {
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

type CallRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type CallResponse struct {
	APIVersion string          `json:"apiVersion"`
	InstanceID string          `json:"instanceId"`
	Result     json.RawMessage `json:"result"`
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
