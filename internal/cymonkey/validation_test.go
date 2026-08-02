package cymonkey

import (
	"encoding/json"
	"testing"
)

func TestRuntimeAgnosticManifestAcceptsWebAndMacOSTargets(t *testing.T) {
	for name, raw := range map[string]string{
		"web":   `{"apiVersion":"jangolova.cymonkey/v1alpha2","kind":"Augmentation","metadata":{"id":"reading-tools","revision":"1"},"spec":{"targets":[{"profile":"web","match":{"urlPatterns":["https://example.com/*"]}}],"permissions":["dom.query"],"web":{"scripts":[]}}}`,
		"macos": `{"apiVersion":"jangolova.cymonkey/v1alpha2","kind":"Augmentation","metadata":{"id":"music-tools","revision":"1"},"spec":{"targets":[{"profile":"macos","match":{"bundleId":"com.apple.Music"}}],"permissions":["app.command.invoke","ui.query"],"macos":{"commands":[{"id":"play"}]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var manifest Manifest
			if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
				t.Fatal(err)
			}
			if err := ValidateManifest(manifest); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRuntimeAgnosticCapabilityRejectsCrossProfileBackend(t *testing.T) {
	err := ValidateCapabilities([]Capability{{
		Name: "ui.query", Profile: ProfileMacOS, Backend: BackendWebExtension,
		Support: SupportMapped, Lifetime: LifetimeAttachment, Persistence: PersistenceSession,
		Effect: "read", InputSchema: json.RawMessage(`{"type":"object"}`),
	}})
	if err == nil {
		t.Fatal("ValidateCapabilities() error = nil")
	}
}

func TestRuntimeHelloRequiresExactVersionAndKnownProfile(t *testing.T) {
	value := Hello{
		ProtocolVersion: ProtocolVersion, Implementation: Implementation{Name: "fixture"},
		Profiles: []Profile{ProfileWeb, ProfileMacOS},
		Backends: []Backend{BackendWebExtension, BackendMacOSAccessibility},
	}
	if err := ValidateHello(value); err != nil {
		t.Fatal(err)
	}
	value.ProtocolVersion = LegacyWebProtocol
	if err := ValidateHello(value); err == nil {
		t.Fatal("ValidateHello() accepted legacy version as v1alpha2")
	}
}
