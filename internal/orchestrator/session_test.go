package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"jangolova/internal/manifest"
)

func TestSessionLifecycleOrder(t *testing.T) {
	t.Parallel()

	var events []string
	registry := testRegistry(t, &events, nil)
	session := NewSession(testManifest(), registry)

	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if state := session.State(); state != StateRunning {
		t.Fatalf("State() = %q, want %q", state, StateRunning)
	}
	if err := session.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if state := session.State(); state != StateStopped {
		t.Fatalf("State() = %q, want %q", state, StateStopped)
	}

	want := []string{
		"surface:open:desktop",
		"engine:start:browser",
		"controller:attach:automation",
		"connector:connect:remote-view:desktop",
		"connector:close:remote-view",
		"controller:close:automation",
		"engine:stop",
		"surface:close:desktop",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestSessionRollsBackOnPartialFailure(t *testing.T) {
	t.Parallel()

	var events []string
	registry := testRegistry(t, &events, errors.New("connector unavailable"))
	session := NewSession(testManifest(), registry)

	err := session.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connector unavailable") {
		t.Fatalf("Start() error = %v", err)
	}
	if state := session.State(); state != StateFailed {
		t.Fatalf("State() = %q, want %q", state, StateFailed)
	}

	want := []string{
		"surface:open:desktop",
		"engine:start:browser",
		"controller:attach:automation",
		"connector:connect:remote-view:desktop",
		"controller:close:automation",
		"engine:stop",
		"surface:close:desktop",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestSessionResolvesAllAdaptersBeforeOpeningResources(t *testing.T) {
	t.Parallel()

	var events []string
	registry := NewRegistry()
	mustRegister(t, registry.RegisterSurface("xvfb", surfaceAdapterFunc(func(
		_ context.Context,
		spec manifest.SurfaceSpec,
	) (Surface, error) {
		events = append(events, "opened:"+spec.Name)
		return fakeSurface{name: spec.Name}, nil
	})))

	session := NewSession(testManifest(), registry)
	err := session.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Start() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("resources opened during failed preflight: %#v", events)
	}
}

func TestSessionRollbackIsNotCanceledWithStartContext(t *testing.T) {
	t.Parallel()

	var surfaceClosed bool
	registry := NewRegistry()
	mustRegister(t, registry.RegisterSurface("xvfb", surfaceAdapterFunc(func(
		_ context.Context,
		spec manifest.SurfaceSpec,
	) (Surface, error) {
		return fakeSurface{
			name: spec.Name,
			close: func(ctx context.Context) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				surfaceClosed = true
				return nil
			},
		}, nil
	})))
	mustRegister(t, registry.RegisterEngine("browser", engineAdapterFunc(func(
		context.Context,
		manifest.EngineSpec,
		map[string]Surface,
	) (EngineInstance, error) {
		return nil, errors.New("engine start failed")
	})))

	value := testManifest()
	value.Spec.Controllers = nil
	value.Spec.Connectors = nil
	session := NewSession(value, registry)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := session.Start(ctx)
	if err == nil || !strings.Contains(err.Error(), "engine start failed") {
		t.Fatalf("Start() error = %v", err)
	}
	if !surfaceClosed {
		t.Fatal("surface was not closed with an uncanceled rollback context")
	}
}

func TestRegistryRejectsDuplicateAdapters(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	adapter := engineAdapterFunc(func(
		context.Context,
		manifest.EngineSpec,
		map[string]Surface,
	) (EngineInstance, error) {
		return fakeEngine{}, nil
	})

	if err := registry.RegisterEngine("browser", adapter); err != nil {
		t.Fatalf("RegisterEngine() error = %v", err)
	}
	err := registry.RegisterEngine("browser", adapter)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate RegisterEngine() error = %v", err)
	}
}

func testManifest() manifest.Manifest {
	return manifest.Manifest{
		APIVersion: manifest.APIVersion,
		Kind:       manifest.Kind,
		Metadata:   manifest.Metadata{Name: "test-session"},
		Spec: manifest.Spec{
			Engine: manifest.EngineSpec{Adapter: "browser"},
			Surfaces: []manifest.SurfaceSpec{
				{Name: "desktop", Adapter: "xvfb"},
			},
			Controllers: []manifest.ControllerSpec{
				{Name: "automation", Adapter: "puppeteer"},
			},
			Connectors: []manifest.ConnectorSpec{
				{Name: "remote-view", Adapter: "vnc", Surface: "desktop"},
			},
		},
	}
}

func testRegistry(t *testing.T, events *[]string, connectorErr error) *Registry {
	t.Helper()

	registry := NewRegistry()
	mustRegister(t, registry.RegisterSurface("xvfb", surfaceAdapterFunc(func(
		_ context.Context,
		spec manifest.SurfaceSpec,
	) (Surface, error) {
		*events = append(*events, "surface:open:"+spec.Name)
		return fakeSurface{
			name: spec.Name,
			close: func(context.Context) error {
				*events = append(*events, "surface:close:"+spec.Name)
				return nil
			},
		}, nil
	})))
	mustRegister(t, registry.RegisterEngine("browser", engineAdapterFunc(func(
		_ context.Context,
		spec manifest.EngineSpec,
		surfaces map[string]Surface,
	) (EngineInstance, error) {
		if surfaces["desktop"] == nil {
			t.Fatal("engine did not receive desktop surface")
		}
		*events = append(*events, "engine:start:"+spec.Adapter)
		return fakeEngine{stop: func(context.Context) error {
			*events = append(*events, "engine:stop")
			return nil
		}}, nil
	})))
	mustRegister(t, registry.RegisterController("puppeteer", controllerAdapterFunc(func(
		_ context.Context,
		spec manifest.ControllerSpec,
		_ EngineInstance,
	) (ControllerHandle, error) {
		*events = append(*events, "controller:attach:"+spec.Name)
		return fakeController{close: func(context.Context) error {
			*events = append(*events, "controller:close:"+spec.Name)
			return nil
		}}, nil
	})))
	mustRegister(t, registry.RegisterConnector("vnc", connectorAdapterFunc(func(
		_ context.Context,
		spec manifest.ConnectorSpec,
		surface Surface,
	) (ConnectorHandle, error) {
		*events = append(*events, "connector:connect:"+spec.Name+":"+surface.Name())
		if connectorErr != nil {
			return nil, connectorErr
		}
		return fakeConnector{close: func(context.Context) error {
			*events = append(*events, "connector:close:"+spec.Name)
			return nil
		}}, nil
	})))
	return registry
}

func mustRegister(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("register adapter: %v", err)
	}
}

type surfaceAdapterFunc func(context.Context, manifest.SurfaceSpec) (Surface, error)

func (fn surfaceAdapterFunc) Open(ctx context.Context, spec manifest.SurfaceSpec) (Surface, error) {
	return fn(ctx, spec)
}

type engineAdapterFunc func(
	context.Context,
	manifest.EngineSpec,
	map[string]Surface,
) (EngineInstance, error)

func (fn engineAdapterFunc) Start(
	ctx context.Context,
	spec manifest.EngineSpec,
	surfaces map[string]Surface,
) (EngineInstance, error) {
	return fn(ctx, spec, surfaces)
}

type controllerAdapterFunc func(
	context.Context,
	manifest.ControllerSpec,
	EngineInstance,
) (ControllerHandle, error)

func (fn controllerAdapterFunc) Attach(
	ctx context.Context,
	spec manifest.ControllerSpec,
	instance EngineInstance,
) (ControllerHandle, error) {
	return fn(ctx, spec, instance)
}

type connectorAdapterFunc func(
	context.Context,
	manifest.ConnectorSpec,
	Surface,
) (ConnectorHandle, error)

func (fn connectorAdapterFunc) Connect(
	ctx context.Context,
	spec manifest.ConnectorSpec,
	surface Surface,
) (ConnectorHandle, error) {
	return fn(ctx, spec, surface)
}

type fakeSurface struct {
	name  string
	close func(context.Context) error
}

func (f fakeSurface) Name() string { return f.name }
func (f fakeSurface) Environment() map[string]string {
	return nil
}
func (f fakeSurface) Close(ctx context.Context) error {
	if f.close != nil {
		return f.close(ctx)
	}
	return nil
}

type fakeEngine struct {
	stop func(context.Context) error
}

func (f fakeEngine) Stop(ctx context.Context) error {
	if f.stop != nil {
		return f.stop(ctx)
	}
	return nil
}

type fakeController struct {
	close func(context.Context) error
}

func (f fakeController) Close(ctx context.Context) error {
	return f.close(ctx)
}

type fakeConnector struct {
	close func(context.Context) error
}

func (f fakeConnector) Close(ctx context.Context) error {
	return f.close(ctx)
}
