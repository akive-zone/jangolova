# Architecture

## System model

```text
Session manifest
      |
      v
Jangolova orchestrator
      |
      +-- Surface providers ---- local window / X11 / Wayland / Xvfb / canvas
      |
      +-- Engine adapters ------ Unity / Unreal / web engine / native process
      |
      +-- Controllers ---------- CDP / Playwright / Puppeteer / input bridge
      |
      +-- Connectors ----------- VNC / WebRTC / capture / remote agent
```

A session is the unit of ownership. It records desired configuration, runtime
state, resources opened by adapters, and cleanup order.

## Core concepts

### Engine

The interactive workload. An engine adapter knows how to start or attach to a
specific runtime and reports the capabilities of the resulting instance.

Examples include Unity, Unreal Engine, a native executable, or a browser
hosting Phaser, Three.js, or Babylon.js.

### Surface

The render destination made available to an engine. A surface provider may
create a native window target, an X11/Wayland display, an Xvfb display, a
framebuffer, or a browser canvas context.

Surfaces expose connection metadata and environment values rather than leaking
provider-specific process management into engines.

### Controller

A control plane attached to a running engine. CDP, Playwright, and Puppeteer
are browser controllers: they do not render the application themselves, but
they can navigate, inspect, automate, and inject input into a browser-hosted
engine.

### Connector

A connector makes a surface or engine capability available to another
consumer. VNC, WebRTC, capture/recording, and a remote-agent tunnel are
connectors.

### Session

A session composes one engine with surfaces, controllers, and connectors. Its
lifecycle is transactional:

1. Validate the manifest and resolve adapters.
2. Open required surfaces.
3. Start or attach to the engine.
4. Attach controllers.
5. Start connectors.
6. Report readiness and health.
7. On stop or failure, close resources in reverse order.

## Manifest direction

The first manifest format is intentionally declarative and versioned:

```yaml
apiVersion: jangolova.dev/v1alpha1
kind: Session
metadata:
  name: rotating-cube
spec:
  engine:
    adapter: browser
    source: ./examples/threejs
  surfaces:
    - name: desktop
      adapter: xvfb
  controllers:
    - name: automation
      adapter: puppeteer
  connectors:
    - name: remote-view
      adapter: vnc
      surface: desktop
```

JSON support comes first to keep the Go foundation dependency-light. YAML can
be added at the configuration boundary without changing the domain model.

## Package direction

```text
cmd/jangolova/          CLI and future daemon entry point
internal/manifest/      versioned configuration and validation
internal/orchestrator/  session lifecycle and rollback
internal/registry/      adapter registration and capability discovery
adapters/               engine, surface, controller, and connector adapters
examples/               runnable engine/session examples
docs/                   product and operator documentation
```

Adapters depend on core contracts. Core packages do not import concrete
adapters.

## Remote architecture

Remote operation will use a Jangolova agent on the engine host. The control
plane sends a validated session plan to the agent; the agent owns local
processes and surfaces and returns capability endpoints and health events.

Remote transport is a later phase. The local lifecycle contracts are being
designed so the same adapter boundary can sit behind an agent without being
rewritten.

## Security boundaries

- Display and control endpoints bind to loopback unless explicitly exposed.
- Remote connections require authenticated, encrypted transport.
- Secrets are referenced from an external provider, never embedded in a
  manifest.
- Session logs redact adapter-declared sensitive fields.
- Destructive or externally visible controller actions require explicit
  configuration.
