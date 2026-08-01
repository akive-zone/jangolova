// Package builtin registers the interaction engines distributed with
// Jangolova. Target runtimes are deliberately absent from this registry.
package builtin

import (
	"fmt"

	"jangolova/adapters/browserautomation"
	"jangolova/internal/orchestrator"
)

func EngineRegistry() (*orchestrator.Registry, error) {
	registry := orchestrator.NewRegistry()
	if err := registry.RegisterEngine("playwright", browserautomation.Playwright()); err != nil {
		return nil, fmt.Errorf("register Playwright interaction engine: %w", err)
	}
	if err := registry.RegisterEngine("puppeteer", browserautomation.Puppeteer()); err != nil {
		return nil, fmt.Errorf("register Puppeteer interaction engine: %w", err)
	}
	return registry, nil
}
