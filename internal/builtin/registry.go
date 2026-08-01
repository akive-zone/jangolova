// Package builtin registers the engine adapters distributed with Jangolova.
package builtin

import (
	"fmt"

	"jangolova/adapters/chromium"
	"jangolova/adapters/nativeprocess"
	"jangolova/adapters/webproject"
	"jangolova/internal/orchestrator"
)

func EngineRegistry() (*orchestrator.Registry, error) {
	registry := orchestrator.NewRegistry()
	if err := registry.RegisterEngine("chromium", chromium.Adapter{}); err != nil {
		return nil, fmt.Errorf("register Chromium engine: %w", err)
	}
	if err := registry.RegisterEngine("web-project", webproject.Adapter{}); err != nil {
		return nil, fmt.Errorf("register web-project engine: %w", err)
	}
	if err := registry.RegisterEngine("native-process", nativeprocess.Adapter{}); err != nil {
		return nil, fmt.Errorf("register native-process engine: %w", err)
	}
	return registry, nil
}
