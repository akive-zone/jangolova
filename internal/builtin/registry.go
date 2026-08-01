// Package builtin registers the interaction engines distributed with
// Jangolova. Target runtimes are deliberately absent from this registry.
package builtin

import (
	"fmt"

	"jangolova/adapters/browserautomation"
	"jangolova/adapters/pacman"
	"jangolova/adapters/safarimcp"
	"jangolova/adapters/webdriverclassic"
	"jangolova/adapters/webpresentation"
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
	if err := registry.RegisterEngine("webdriver-classic", webdriverclassic.Generic()); err != nil {
		return nil, fmt.Errorf("register WebDriver Classic interaction engine: %w", err)
	}
	if err := registry.RegisterEngine("webkit-webdriver", webdriverclassic.WebKit()); err != nil {
		return nil, fmt.Errorf("register WebKit WebDriver interaction engine: %w", err)
	}
	if err := registry.RegisterEngine("safari-mcp", safarimcp.Adapter{}); err != nil {
		return nil, fmt.Errorf("register Safari MCP interaction engine: %w", err)
	}
	if err := registry.RegisterEngine("web-presentation", webpresentation.Adapter{}); err != nil {
		return nil, fmt.Errorf("register web presentation interaction engine: %w", err)
	}
	if err := registry.RegisterEngine("pacman", pacman.Adapter{}); err != nil {
		return nil, fmt.Errorf("register Pacman interaction engine: %w", err)
	}
	return registry, nil
}
