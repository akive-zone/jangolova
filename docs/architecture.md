# Architecture

Unity and Unreal semantic presentation use the [Pacman boundary](pacman.md):
Jangolova dials a caller-owned semantic endpoint while the engine keeps
rendering and the supervisor separately owns target and display lifecycle.

Jangolova owns its Grimlock agent subsystem plus interaction and presentation
engines. Xallet, another operator, or the caller’s host owns the target runtime
with which those engines interact.

Interaction includes operating semantic browser/application interfaces and
requesting display-level pointer/keyboard actions. Presentation includes
creating and updating dynamic 2D/3D interfaces. Both use the common semantic
protocol while target runtime and display ownership remain external.

## Jangolova ownership model

Jangolova is the browser runtime and the browser extension product/name.

Jangolova owns:

- transport and backend discovery for browser interaction (`cdp`, `webdriver-bidi`,
  `safarimcp`, optional webextension backend);
- authenticated control-plane and extension control endpoints;
- script-injection infrastructure and policy checks;
- storage service, network rules service, shared event service;
- platform-native workers used by browser adapters;
- worker lifecycle, health, and reconnection policy.

Cymonkey is a Jangolova subsystem that adds augmented browsing:

- DOM query/observe/patch, styles, overlays, script injection orchestration;
- augmentation install/update/enable/disable lifecycle;
- per-augmentation namespacing and ownership.

Cymonkey consumes Jangolova platform services; it does not own raw storage,
network interception, browser permissions, or process lifecycle.

Pacman is the existing engine-neutral semantic presentation system
`jangolova.pacman/v1alpha1`:

- only explicit scene/object/material/camera/animation/timeline/UI/event
  registration is supported;
- no arbitrary Three.js object scanning;
- no implicit control-plane mapping from raw engine IDs.

### Trust and lifecycle boundary

Caller-owned browser/application lifecycle is preserved:

1. Jangolova attaches to caller-owned endpoints.
2. Jangolova disconnects never own/terminate browser/app processes.
3. Reconnect and credential rollover preserve the caller-owned process state.

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
        +-- Cymonkey (subsystem) ---- CDP / BiDi / Safari MCP+
        |       +-- optional Jangolova Browser Extension---+
        +-- WebDriver Classic ------ existing session+--> caller-owned targets
        +-- Safari MCP -------- Streamable HTTP relay+
        +-- Godot / Unity / Unreal bridge WS --------+
        +-- Three.js presentation ------- web -------+
                                                      |
                                    Caller-owned or host-owned lifecycle
```

Endpoint and handle flow is inward: the operator creates a target and gives
Jangolova its connection coordinates. Jangolova never returns a newly created
Chromium endpoint because it does not create Chromium.

## Package direction

```text
cmd/jangolova/              CLI and authenticated provider
internal/boundary/          trust boundary and adapter registration
internal/engineprovider/    target-in / semantic-call protocol
internal/orchestrator/      interaction lifecycle and target contracts
internal/bridge/            engine-neutral semantic methods
internal/grimlock/          ADK agents and caller-supplied model connectors
adapters/browserautomation/  Playwright CDP and Puppeteer CDP/BiDi attachment
adapters/cymonkey/          transport-neutral augmented browsing subsystem
adapters/webdriverclassic/   existing W3C WebDriver session attachment
adapters/safarimcp/          caller-owned Safari MCP relay attachment
pkg/browser-cymonkey/       legacy package path for the Jangolova browser extension
pkg/browser-jangolova/       migration destination (compatibility/migration layer)
pkg/                       distributable Godot, Unity, Unreal, and related packages
tests/godot-pacman-fixture/ license-free Godot conformance project
tests/unreal-pacman-fixture/ caller-owned Unreal conformance project
deploy/engine-runtime/       optional interaction artifact
deploy/godot-pacman-fixture/ optional headless Godot target image
deploy/unreal-pacman-fixture/ optional packaged Unreal target image
tests/docker/               target-owning portability fixture only
```

No package imports Xallet. No product adapter provisions a target runtime.
The complete interface model is documented in
[Interface creation and operation](interface-model.md).
