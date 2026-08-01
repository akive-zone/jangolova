# Jangolova

Jangolova is a deployment-neutral display-engine toolkit. It discovers,
launches, attaches to, observes the health of, and stops browser and native
engines such as Chromium, Unity, Unreal, and generic executables.

Jangolova consumes runtime inputs supplied by its caller. It does not own
Xvfb, VNC, WebRTC, CDP clients, container placement, networking, volumes, port
publication, or display-session policy. Xallet owns those concerns when the
products are used together.

Jangolova runs directly on a physical machine or VM, inside an independently
configured container, or as a Xallet-managed workload. Xallet integration is a
supported deployment mode, not a runtime requirement.

## Included engines and integrations

- Chromium launch/attach with private CDP endpoint discovery.
- Local web-project serving through Chromium.
- Generic native-process lifecycle with caller-supplied environment and opaque
  handles.
- Engine-side cooperative bridge protocol and conformance checks.
- Unity Package Manager bridge package and Three.js example experience.
- Cursor-addressed engine readiness, health, exit, and failure events.

## Commands

List engine adapters:

```bash
go run ./cmd/jangolova engines
go run ./cmd/jangolova engines --json
```

Launch Chromium directly on the native host:

```bash
go run ./cmd/jangolova launch-engine \
  --adapter chromium \
  --source https://example.com
```

Launch an engine against caller-owned runtime inputs:

```bash
go run ./cmd/jangolova launch-engine \
  --adapter native-process \
  --source ./my-engine \
  --env DISPLAY=:99 \
  --handle native.window=caller-owned-window-1234
```

Run the authenticated provider:

```bash
export JANGOLOVA_PROVIDER_TOKEN="replace-with-a-random-secret"
go run ./cmd/jangolova serve-engine-provider --bind 127.0.0.1:7391
```

## Ownership boundary

The supported Jangolova API contains engines and engine-side integrations only.
The original combined session, surface, controller, connector, agent runtime,
and deployment scripts have been removed. Test fixtures may construct a
temporary external display to verify portability, but they are isolated under
`tests/` and are not product configuration.

See:

- [Architecture](docs/architecture.md)
- [Vision](docs/vision.md)
- [Engine provider](docs/engine-provider.md)
- [Deployment modes](docs/deployment-modes.md)
- [Experience bridge protocol](docs/bridge-protocol.md)
- [Dynamic presentation integrations](docs/dynamic-presentation.md)
- [Native engines](docs/native-engines.md)
- [Xallet boundary](docs/xallet-boundary.md)
- [Roadmap](docs/roadmap.md)

## Tests

```bash
go test ./...
npm run test:unity-package
```

The optional Linux portability fixture is documented in
[tests/docker/README.md](tests/docker/README.md). Docker is not required by
Jangolova itself.
