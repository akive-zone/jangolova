package cymonkey

import "encoding/json"

const ProtocolVersion = "jangolova.cymonkey/v1alpha1"

type BackendName string

const (
	BackendCDP                BackendName = "cdp"
	BackendBiDi               BackendName = "bidi"
	BackendSafariMCP          BackendName = "safari-mcp"
	BackendWebExtension       BackendName = "webextension"
	BackendMacOSAppleEvents   BackendName = "macos-apple-events"
	BackendMacOSAccessibility BackendName = "macos-accessibility"
	BackendMacOSCooperative   BackendName = "macos-cooperative"
)

type SupportMode string

const (
	SupportNative   SupportMode = "native"
	SupportMapped   SupportMode = "mapped"
	SupportEmulated SupportMode = "emulated"
)

type Lifetime string

const (
	LifetimeCall           Lifetime = "call"
	LifetimeDocument       Lifetime = "document"
	LifetimeBrowserSession Lifetime = "browser-session"
	LifetimeProfile        Lifetime = "profile"
)

type Persistence string

const (
	PersistenceEphemeral  Persistence = "ephemeral"
	PersistenceSession    Persistence = "session"
	PersistencePersistent Persistence = "persistent"
)

type Capability struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Backend      BackendName     `json:"backend"`
	Support      SupportMode     `json:"support"`
	Lifetime     Lifetime        `json:"lifetime"`
	Persistence  Persistence     `json:"persistence"`
	Effect       string          `json:"effect"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	Alternatives []BackendName   `json:"alternatives,omitempty"`
}

type Hello struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Implementation  implementation `json:"implementation"`
	Backends        []BackendName  `json:"backends"`
	Features        []string       `json:"features,omitempty"`
}

type implementation struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

func objectSchema(required ...string) json.RawMessage {
	value, _ := json.Marshal(map[string]any{
		"type": "object", "required": required, "additionalProperties": true,
	})
	return value
}
