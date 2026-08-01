// Package manifest contains the engine specification shared by Jangolova
// adapters and provider transports.
package manifest

import "encoding/json"

type EngineSpec struct {
	Adapter string          `json:"adapter"`
	Source  string          `json:"source,omitempty"`
	Options json.RawMessage `json:"options,omitempty"`
}
