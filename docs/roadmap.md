# Roadmap

The roadmap is organized around executable milestones rather than a complete
adapter catalog.

## Phase 0: Preserve the vertical slice

Status: complete.

- Headed Chromium on Xvfb.
- Localhost-bound CDP and VNC.
- Persistent browser profiles.
- Go CDP, Go Playwright, and Puppeteer controller modes.
- Local fixture and Docker smoke tests.
- Initial Git history.

## Phase 1: Session foundation

Status: in progress.

- Define the versioned session manifest.
- Validate names, adapter references, and cross-resource references.
- Define engine, surface, controller, and connector contracts.
- Implement ordered startup and reverse-order rollback.
- Add `jangolova validate` and adapter discovery commands.
- Keep the foundation free of concrete engine dependencies.

Exit criterion: a test session using fake adapters proves startup, readiness,
stop, and rollback behavior.

## Phase 2: Extract the browser vertical slice

- Implement Xvfb as a surface adapter.
- Implement Chromium as a browser engine adapter.
- Wrap existing CDP, Go Playwright, and Puppeteer flows as controllers.
- Implement VNC as a connector.
- Express the Xpost prototype as a session manifest.

Exit criterion: the current Docker smoke test runs through the generalized
orchestrator without losing any controller mode.

## Phase 3: Browser rendering engines

- Add a static/web-project engine adapter.
- Add Phaser, Three.js, and Babylon.js examples.
- Add canvas readiness and frame-capture capabilities.
- Add browser input and viewport configuration.

Exit criterion: one manifest can launch each example and expose it locally,
through VNC, and through screenshot capture.

## Phase 4: Native engines

- Define native process and project/build discovery capabilities.
- Add Unity adapter proof of concept.
- Add Unreal Engine adapter proof of concept.
- Record engine-specific capabilities without expanding the common contract
  prematurely.

Exit criterion: launch or attach to one Unity and one Unreal example and
connect each to a supported display surface.

## Phase 5: Interactive remote transport

- Add WebRTC connector.
- Define the authenticated remote-agent protocol.
- Add placement and capability negotiation.
- Forward input, health, logs, and selected artifacts.

Exit criterion: run an engine on one machine and interact with it from another
using a single session description.

## Phase 6: Operations

- Session persistence and recovery.
- Structured events, metrics, and tracing.
- Resource limits and concurrency policy.
- Secret-provider integration.
- Compatibility policy for manifests and adapter APIs.
