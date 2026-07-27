package orchestrator

import (
	"errors"
	"fmt"
	"strings"

	"jangolova/internal/manifest"
)

type Registry struct {
	engines     map[string]EngineAdapter
	surfaces    map[string]SurfaceAdapter
	controllers map[string]ControllerAdapter
	connectors  map[string]ConnectorAdapter
}

func NewRegistry() *Registry {
	return &Registry{
		engines:     make(map[string]EngineAdapter),
		surfaces:    make(map[string]SurfaceAdapter),
		controllers: make(map[string]ControllerAdapter),
		connectors:  make(map[string]ConnectorAdapter),
	}
}

func (r *Registry) RegisterEngine(name string, adapter EngineAdapter) error {
	return register(r.engines, "engine", name, adapter)
}

func (r *Registry) RegisterSurface(name string, adapter SurfaceAdapter) error {
	return register(r.surfaces, "surface", name, adapter)
}

func (r *Registry) RegisterController(name string, adapter ControllerAdapter) error {
	return register(r.controllers, "controller", name, adapter)
}

func (r *Registry) RegisterConnector(name string, adapter ConnectorAdapter) error {
	return register(r.connectors, "connector", name, adapter)
}

func register[T any](adapters map[string]T, kind, name string, adapter T) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%s adapter name is required", kind)
	}
	if any(adapter) == nil {
		return fmt.Errorf("%s adapter %q is nil", kind, name)
	}
	if _, exists := adapters[name]; exists {
		return fmt.Errorf("%s adapter %q is already registered", kind, name)
	}
	adapters[name] = adapter
	return nil
}

type plan struct {
	engine      EngineAdapter
	surfaces    []SurfaceAdapter
	controllers []ControllerAdapter
	connectors  []ConnectorAdapter
}

func (r *Registry) resolve(value manifest.Manifest) (plan, error) {
	if r == nil {
		return plan{}, errors.New("adapter registry is nil")
	}

	var problems []error
	resolved := plan{
		surfaces:    make([]SurfaceAdapter, len(value.Spec.Surfaces)),
		controllers: make([]ControllerAdapter, len(value.Spec.Controllers)),
		connectors:  make([]ConnectorAdapter, len(value.Spec.Connectors)),
	}

	var exists bool
	resolved.engine, exists = r.engines[value.Spec.Engine.Adapter]
	if !exists {
		problems = append(problems, fmt.Errorf(
			"engine adapter %q is not registered",
			value.Spec.Engine.Adapter,
		))
	}

	for index, spec := range value.Spec.Surfaces {
		resolved.surfaces[index], exists = r.surfaces[spec.Adapter]
		if !exists {
			problems = append(problems, fmt.Errorf(
				"surface adapter %q for %q is not registered",
				spec.Adapter,
				spec.Name,
			))
		}
	}
	for index, spec := range value.Spec.Controllers {
		resolved.controllers[index], exists = r.controllers[spec.Adapter]
		if !exists {
			problems = append(problems, fmt.Errorf(
				"controller adapter %q for %q is not registered",
				spec.Adapter,
				spec.Name,
			))
		}
	}
	for index, spec := range value.Spec.Connectors {
		resolved.connectors[index], exists = r.connectors[spec.Adapter]
		if !exists {
			problems = append(problems, fmt.Errorf(
				"connector adapter %q for %q is not registered",
				spec.Adapter,
				spec.Name,
			))
		}
	}

	return resolved, errors.Join(problems...)
}
