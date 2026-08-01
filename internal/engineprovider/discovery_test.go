package engineprovider

import (
	"context"
	"testing"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

type inspectedAdapter struct{}

func (inspectedAdapter) Start(
	context.Context,
	manifest.EngineSpec,
	orchestrator.EngineRuntime,
) (orchestrator.EngineInstance, error) {
	return nil, nil
}

func (inspectedAdapter) InspectEngine(context.Context) orchestrator.EngineInspection {
	return orchestrator.EngineInspection{
		Available:    false,
		Capabilities: []string{"attach", "attach", "health"},
		Message:      "launch binary missing",
	}
}

func TestDiscoverEnginesUsesAdapterInspection(t *testing.T) {
	t.Parallel()

	registry := orchestrator.NewRegistry()
	if err := registry.RegisterEngine("fixture", inspectedAdapter{}); err != nil {
		t.Fatal(err)
	}
	engines := DiscoverEngines(context.Background(), registry)
	if len(engines) != 1 || engines[0].Available ||
		engines[0].Message != "launch binary missing" {
		t.Fatalf("DiscoverEngines() = %#v", engines)
	}
	want := []string{"attach", "health"}
	if len(engines[0].Capabilities) != len(want) {
		t.Fatalf("capabilities = %#v", engines[0].Capabilities)
	}
	for index := range want {
		if engines[0].Capabilities[index] != want[index] {
			t.Fatalf("capabilities = %#v", engines[0].Capabilities)
		}
	}
}
