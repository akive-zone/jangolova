package orchestrator

import (
	"context"

	"jangolova/internal/manifest"
)

type Surface interface {
	Name() string
	Environment() map[string]string
	Close(context.Context) error
}

type EngineInstance interface {
	Stop(context.Context) error
}

type ControllerHandle interface {
	Close(context.Context) error
}

type ConnectorHandle interface {
	Close(context.Context) error
}

type SurfaceAdapter interface {
	Open(context.Context, manifest.SurfaceSpec) (Surface, error)
}

type EngineAdapter interface {
	Start(context.Context, manifest.EngineSpec, map[string]Surface) (EngineInstance, error)
}

type ControllerAdapter interface {
	Attach(context.Context, manifest.ControllerSpec, EngineInstance) (ControllerHandle, error)
}

type ConnectorAdapter interface {
	Connect(context.Context, manifest.ConnectorSpec, Surface) (ConnectorHandle, error)
}
