# Architecture

Unity and Unreal semantic presentation use the [Pacman boundary](pacman.md):
Jangolova dials a caller-owned semantic endpoint while the engine keeps
rendering and the supervisor separately owns target and display lifecycle.

Jangolova owns its Grimlock agent subsystem plus interaction and presentation
engines. Xallet, a native host, or another operator owns the target runtimes
with which those engines interact.

Interaction includes operating semantic browser/application interfaces and
requesting display-level pointer/keyboard actions. Presentation includes
creating and updating dynamic 2D/3D interfaces. Both use the common semantic
protocol while target runtime and display ownership remain external.

## Jangolova Browser Extension System

Jangolova is the browser runtime and extension product. It owns backend
discovery across CDP, WebDriver BiDi, Safari MCP, and WebExtension, plus the
authenticated control plane, browser policy, packaged injection, scoped
storage, network rules, and shared event service.

Cymonkey is its augmented-browsing subsystem. It owns augmentation lifecycle
and the DOM/style/overlay vocabulary, while consuming Jangolova platform
services. Pacman remains the engine-neutral explicit-registration presentation
contract. In browsers, `@jangolova/threejs-pacman` maps allowlisted stable IDs
to Three.js scenes, objects, cameras, materials, and animation actions. It never
scans arbitrary page or scene objects.

The public page bridge contains only page-safe Cymonkey operations. Platform
services and Pacman control are reachable only through the authenticated
extension control plane or a caller-owned CDP/BiDi/MCP connection.

## System boundary

```text
User, agent, IDE, or application
        |
        +-- deterministic engine API -----------------+
        |                                             |
        +-- HTTP / MCP / ACP / A2A --> Grimlock ------+
                                      model + policy  |
                                                    v
                                      Jangolova interaction core
        |
        +-- Playwright ------------------ CDP -------+
        +-- Puppeteer ---------------- CDP / BiDi ---+
        +-- Jangolova browser runtime ----------------+
        |       +-- CDP / BiDi / Safari MCP ----------+
        |       +-- optional Jangolova WebExtension --+
        |               +-- Cymonkey subsystem -------+
        |               +-- Pacman / Three.js --------+
        +-- WebDriver Classic ------ existing session+--> caller-owned targets
        +-- Safari MCP -------- Streamable HTTP relay+
        +-- Godot / Unity / Unreal bridge WS --------+
        +-- Three.js presentation ------- web -------+
                                                      |
                                    Xallet or native host owns lifecycle
```

Endpoint and handle flow is inward: the operator creates a target and gives
Jangolova its connection coordinates. Jangolova never returns a newly created
Chromium endpoint because it does not create Chromium.

## Ownership

Jangolova owns:

- Playwright, Puppeteer, and future browser-interaction libraries;
- WebDriver and MCP clients that attach to caller-owned WebKit/Safari targets;
- Three.js presentation logic and cooperative web experiences;
- Unity and Unreal interaction plugins and bridge protocol;
- semantic capability discovery, description, actions, observations, events,
  and interaction-session health;
- worker processes used internally by an interaction adapter.
- provider-neutral adapters that translate display observations and
  pointer/keyboard intent into a caller-supplied surface/input contract.

The target provider owns:

- Chromium, WebKit, Gecko, SpiderMonkey, Unity, Unreal, and other executable
  runtime processes;
- physical machines, VMs, OCI workloads, and their lifecycle;
- displays, windows, surfaces, profiles, devices, networks, ports, and secrets;
- VNC, WebRTC, CDP exposure, capture, access policy, and session state.
- browser-driver processes and any stdio/network relay required to expose a
  caller-owned Safari MCP server.

When Xallet is present, it is the target provider. In standalone use, a native
user or another system supplies the same target contract.

## Connection contract

An interaction adapter receives an adapter name, optional interaction-specific
options, and a caller-owned target containing:

- a target kind such as `browser`;
- typed endpoints such as `cdp`, `webdriver-bidi`, `webdriver`,
  `mcp-streamable-http`, or `websocket`;
- optional opaque native handles.

Connecting creates only a Jangolova interaction session. Disconnecting releases
the adapter and its Playwright/Puppeteer worker without stopping the target.

## Semantic protocol

Every callable interaction engine uses the common bridge methods:

- `hello`
- `capabilities`
- `describe`
- `act`
- `events`

The authenticated, provider-neutral `interaction.engine/v1alpha1` HTTP API transports those
calls. The cooperative Unity bridge implements the same vocabulary.

## Package direction

```text
cmd/jangolova/              CLI and authenticated provider
internal/engineprovider/    target-in / semantic-call protocol
internal/orchestrator/      interaction lifecycle and target contracts
internal/bridge/            engine-neutral semantic methods
internal/grimlock/          ADK agents and caller-supplied model connectors
adapters/browserautomation/ Playwright CDP and Puppeteer CDP/BiDi attachment
adapters/cymonkey/          transport-neutral augmented browsing and backend policy
adapters/webdriverclassic/  existing W3C WebDriver session attachment
adapters/safarimcp/         caller-owned Safari MCP relay attachment
pkg/browser-ext/      canonical WXT runtime and Xallet spoke
pkg/threejs-pacman/         explicit-registration Three.js Pacman runtime
pkg/                        distributable Godot, Unity, and Unreal Pacman packages
tests/godot-pacman-fixture/ license-free Godot conformance project
tests/unreal-pacman-fixture/ caller-owned Unreal conformance project
deploy/engine-runtime/      optional interaction artifact
deploy/godot-pacman-fixture/ optional headless Godot target image
deploy/unreal-pacman-fixture/ optional packaged Unreal target image
tests/docker/               target-owning portability fixture only
```

No package imports Xallet. No product adapter provisions a target runtime.
The complete interface model is documented in
[Interface creation and operation](interface-model.md).
