package builtin

import (
	"context"
	"testing"

	"jangolova/internal/orchestrator"
)

func TestRegistryIncludesProviderVisiblePacman(t *testing.T) {
	registry, err := EngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	adapter, ok := registry.Engine("pacman")
	if !ok {
		t.Fatal("Pacman adapter is not registered")
	}
	inspection := adapter.(orchestrator.EngineInspector).InspectEngine(context.Background())
	if !inspection.Available || !hasCapability(inspection.Capabilities, "target.pacman-ws") {
		t.Fatalf("Pacman inspection = %#v", inspection)
	}
}

func TestRegistryIncludesProviderVisibleCymonkey(t *testing.T) {
	registry, err := EngineRegistry()
	if err != nil {
		t.Fatal(err)
	}
	adapter, ok := registry.Engine("cymonkey")
	if !ok {
		t.Fatal("Cymonkey adapter is not registered")
	}
	inspection := adapter.(orchestrator.EngineInspector).InspectEngine(context.Background())
	if !hasCapability(inspection.Capabilities, "script.register") {
		t.Fatalf("Cymonkey inspection = %#v", inspection)
	}
}

func hasCapability(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
