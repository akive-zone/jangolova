package cymonkey

import (
	"context"
	"errors"
	"fmt"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

type processBackend struct {
	name             BackendName
	endpointProtocol string
}

func (backend processBackend) Name() BackendName { return backend.name }
func (backend processBackend) Compatible(target orchestrator.EngineTarget) bool {
	_, ok := target.Endpoint(backend.endpointProtocol)
	return target.Kind == "browser" && ok
}

var configuredBackends = []Backend{
	processBackend{name: BackendCDP, endpointProtocol: "cdp"},
	processBackend{name: BackendBiDi, endpointProtocol: "webdriver-bidi"},
	safariMCPBackend{},
}

func (Adapter) Connect(ctx context.Context, spec manifest.EngineSpec, target orchestrator.EngineTarget) (orchestrator.EngineInstance, error) {
	if target.Kind != "browser" {
		return nil, errors.New("cymonkey requires target.kind browser")
	}
	config, err := decodeOptions(spec.Options)
	if err != nil {
		return nil, err
	}
	backend, err := selectBackend(config.Backend, target)
	if err != nil {
		return nil, err
	}
	if backend.Name() != BackendCDP && config.Extension.Mode == extensionRequired {
		return nil, fmt.Errorf("Cymonkey extension mode required is not available with backend %s", backend.Name())
	}
	return backend.Connect(ctx, spec, target, config)
}

func selectBackend(requested string, target orchestrator.EngineTarget) (Backend, error) {
	for _, backend := range configuredBackends {
		if requested != "auto" && requested != string(backend.Name()) {
			continue
		}
		if backend.Compatible(target) {
			return backend, nil
		}
	}
	if requested == "auto" {
		return nil, errors.New("Cymonkey requires a caller-owned CDP, WebDriver BiDi, or Safari MCP endpoint")
	}
	return nil, fmt.Errorf("Cymonkey backend %s has no compatible caller-owned endpoint", requested)
}
