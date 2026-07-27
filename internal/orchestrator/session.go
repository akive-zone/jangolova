package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"jangolova/internal/manifest"
)

type State string

const (
	StateNew      State = "new"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)

type Session struct {
	mu       sync.Mutex
	manifest manifest.Manifest
	registry *Registry
	state    State
	cleanup  []cleanupFunc
}

type cleanupFunc func(context.Context) error

func NewSession(value manifest.Manifest, registry *Registry) *Session {
	return &Session{
		manifest: value,
		registry: registry,
		state:    StateNew,
	}
}

func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateNew {
		return fmt.Errorf("start session: state is %q, expected %q", s.state, StateNew)
	}
	s.state = StateStarting

	if err := s.manifest.Validate(); err != nil {
		s.state = StateFailed
		return fmt.Errorf("start session: validate manifest: %w", err)
	}
	resolved, err := s.registry.resolve(s.manifest)
	if err != nil {
		s.state = StateFailed
		return fmt.Errorf("start session: resolve adapters: %w", err)
	}

	surfaces := make(map[string]Surface, len(s.manifest.Spec.Surfaces))
	for index, spec := range s.manifest.Spec.Surfaces {
		surface, openErr := resolved.surfaces[index].Open(ctx, spec)
		if openErr != nil {
			return s.fail(ctx, fmt.Errorf("open surface %q: %w", spec.Name, openErr))
		}
		if surface == nil {
			return s.fail(ctx, fmt.Errorf("open surface %q: adapter returned nil", spec.Name))
		}
		surfaces[spec.Name] = surface
		s.cleanup = append(s.cleanup, func(ctx context.Context) error {
			if err := surface.Close(ctx); err != nil {
				return fmt.Errorf("close surface %q: %w", spec.Name, err)
			}
			return nil
		})
	}

	instance, err := resolved.engine.Start(ctx, s.manifest.Spec.Engine, surfaces)
	if err != nil {
		return s.fail(ctx, fmt.Errorf("start engine %q: %w", s.manifest.Spec.Engine.Adapter, err))
	}
	if instance == nil {
		return s.fail(ctx, fmt.Errorf(
			"start engine %q: adapter returned nil",
			s.manifest.Spec.Engine.Adapter,
		))
	}
	s.cleanup = append(s.cleanup, func(ctx context.Context) error {
		if err := instance.Stop(ctx); err != nil {
			return fmt.Errorf("stop engine %q: %w", s.manifest.Spec.Engine.Adapter, err)
		}
		return nil
	})

	for index, spec := range s.manifest.Spec.Controllers {
		handle, attachErr := resolved.controllers[index].Attach(ctx, spec, instance)
		if attachErr != nil {
			return s.fail(ctx, fmt.Errorf("attach controller %q: %w", spec.Name, attachErr))
		}
		if handle == nil {
			return s.fail(ctx, fmt.Errorf("attach controller %q: adapter returned nil", spec.Name))
		}
		s.cleanup = append(s.cleanup, func(ctx context.Context) error {
			if err := handle.Close(ctx); err != nil {
				return fmt.Errorf("close controller %q: %w", spec.Name, err)
			}
			return nil
		})
	}

	for index, spec := range s.manifest.Spec.Connectors {
		surface := surfaces[spec.Surface]
		handle, connectErr := resolved.connectors[index].Connect(ctx, spec, surface)
		if connectErr != nil {
			return s.fail(ctx, fmt.Errorf("connect connector %q: %w", spec.Name, connectErr))
		}
		if handle == nil {
			return s.fail(ctx, fmt.Errorf("connect connector %q: adapter returned nil", spec.Name))
		}
		s.cleanup = append(s.cleanup, func(ctx context.Context) error {
			if err := handle.Close(ctx); err != nil {
				return fmt.Errorf("close connector %q: %w", spec.Name, err)
			}
			return nil
		})
	}

	s.state = StateRunning
	return nil
}

func (s *Session) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateRunning {
		return fmt.Errorf("stop session: state is %q, expected %q", s.state, StateRunning)
	}
	s.state = StateStopping
	err := runCleanup(ctx, s.cleanup)
	s.cleanup = nil
	s.state = StateStopped
	if err != nil {
		return fmt.Errorf("stop session: %w", err)
	}
	return nil
}

func (s *Session) fail(ctx context.Context, cause error) error {
	cleanupErr := runCleanup(context.WithoutCancel(ctx), s.cleanup)
	s.cleanup = nil
	s.state = StateFailed
	if cleanupErr != nil {
		return errors.Join(cause, fmt.Errorf("rollback session: %w", cleanupErr))
	}
	return cause
}

func runCleanup(ctx context.Context, cleanup []cleanupFunc) error {
	var problems []error
	for index := len(cleanup) - 1; index >= 0; index-- {
		if err := cleanup[index](ctx); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}
