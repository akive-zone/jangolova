package cymonkey

import (
	"context"
	"encoding/json"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

// Backend is the transport-specific boundary below Cymonkey's stable semantic
// contract. Implementations attach to caller-owned targets and must not own the
// browser lifecycle.
type Backend interface {
	Name() BackendName
	Compatible(orchestrator.EngineTarget) bool
	Connect(context.Context, manifest.EngineSpec, orchestrator.EngineTarget, options) (orchestrator.EngineInstance, error)
}

type extensionMode string

const (
	extensionAuto     extensionMode = "auto"
	extensionDisabled extensionMode = "disabled"
	extensionRequired extensionMode = "required"
)

type extensionOptions struct {
	Mode extensionMode `json:"mode,omitempty"`
	ID   string        `json:"id,omitempty"`
}

type policyOptions struct {
	AllowedCapabilities []string `json:"allowedCapabilities,omitempty"`
	AllowedOrigins      []string `json:"allowedOrigins,omitempty"`
}

type options struct {
	Backend    string           `json:"backend,omitempty"`
	NodePath   string           `json:"nodePath,omitempty"`
	WorkerPath string           `json:"workerPath,omitempty"`
	Extension  extensionOptions `json:"extension,omitempty"`
	Policy     policyOptions    `json:"policy,omitempty"`

	// extensionId is accepted as a compatibility alias for the original
	// extension-only prototype.
	ExtensionID string `json:"extensionId,omitempty"`
}

type semanticAction struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

func decodeAction(raw json.RawMessage) (semanticAction, error) {
	var action semanticAction
	err := json.Unmarshal(raw, &action)
	if action.Input == nil {
		action.Input = map[string]any{}
	}
	return action, err
}
