package grimlock

import (
	"encoding/json"
	"time"

	"jangolova/internal/engineprovider"
)

// APIVersion is the northbound Grimlock application contract. HTTP, MCP,
// ACP, and A2A adapters should all map to this same session model.
const APIVersion = "agent.grimlock/v1alpha1"

// BindingSpec describes one caller-owned interaction target. The target
// descriptor is intentionally provider-neutral; it may point at a native
// process, a VM, a container, or a Xallet-managed surface.
type BindingSpec struct {
	InteractionID       string                    `json:"interactionId"`
	Engine              engineprovider.EngineSpec `json:"engine"`
	Target              engineprovider.Target     `json:"target"`
	AllowedCapabilities []string                  `json:"allowedCapabilities,omitempty"`
	AllowWrites         bool                      `json:"allowWrites,omitempty"`
}

type CreateSessionRequest struct {
	APIVersion string        `json:"apiVersion"`
	UserID     string        `json:"userId"`
	Agent      AgentSpec     `json:"agent"`
	Bindings   []BindingSpec `json:"bindings"`
}

type SessionView struct {
	APIVersion       string            `json:"apiVersion"`
	SessionID        string            `json:"sessionId"`
	UserID           string            `json:"userId"`
	Status           string            `json:"status"`
	CreatedAt        time.Time         `json:"createdAt"`
	Model            ModelProfile      `json:"model"`
	PendingApprovals []PendingApproval `json:"pendingApprovals,omitempty"`
}

type PendingApproval struct {
	ID      string `json:"id"`
	Hint    string `json:"hint"`
	Payload any    `json:"payload,omitempty"`
}

type RunRequest struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Text       string `json:"text"`
	Stream     bool   `json:"stream,omitempty"`
}

type RunResponse struct {
	APIVersion string          `json:"apiVersion"`
	SessionID  string          `json:"sessionId"`
	Cursor     string          `json:"cursor"`
	Events     []EventEnvelope `json:"events"`
}

type EventEnvelope struct {
	Cursor string          `json:"cursor"`
	Event  json.RawMessage `json:"event"`
}

type EventsResponse struct {
	APIVersion string          `json:"apiVersion"`
	SessionID  string          `json:"sessionId"`
	Cursor     string          `json:"cursor"`
	Events     []EventEnvelope `json:"events"`
}

type ConfirmationRequest struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Confirmed  bool   `json:"confirmed"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ConnectorDescriptor struct {
	Protocol string `json:"protocol"`
}
