# Jangolova

Jangolova is an engine and display orchestration system. It launches or
attaches to interactive engines, connects their output to local or remote
display systems, forwards control and input, and manages the resulting
session.

The long-term target includes:

- Native engines such as Unity and Unreal Engine.
- Browser engines such as Phaser, Three.js, and Babylon.js.
- Browser controllers such as CDP, Playwright, and Puppeteer.
- Local and virtual surfaces such as native windows, X11, Wayland, Xvfb,
  framebuffers, and browser canvases.
- Remote connectors such as VNC, WebRTC, and remote Jangolova agents.

The repository currently contains a working browser-automation vertical slice.
It runs Chromium on a virtual display, exposes it over VNC, and controls it
through native Go CDP, Go Playwright, or Puppeteer. That prototype is being
retained as the first integration test and reference application while the
general orchestration core is built.

## Project direction

- [Vision](docs/vision.md)
- [Architecture](docs/architecture.md)
- [Roadmap](docs/roadmap.md)
- [Xpost prototype](docs/xpost-prototype.md)

## Current prototype

The prototype can be exercised safely against its local fixture:

```bash
docker compose build
docker compose run --rm --entrypoint scripts/docker-smoke-test.sh xpost
```

For VPS setup, persistent login, and individual controller modes, see the
[Xpost prototype guide](docs/xpost-prototype.md).

## Status

Jangolova is in its foundation phase. Public APIs and manifests are expected to
change until the first end-to-end generalized session is running.

The first `v1alpha1` session manifest and lifecycle contracts are available.
Validate the example manifest with:

```bash
go run ./cmd/jangolova validate --file examples/browser-session.json
```

This validates configuration and references only. Concrete adapters will be
connected to the orchestrator during the browser vertical-slice extraction.
