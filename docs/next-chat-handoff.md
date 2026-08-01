# Next-chat handoff: Jangolova presentation work

This document is the restart point for the next implementation chat.

## Current state

The repository now has a provider-visible `web-presentation` adapter. It
attaches to a browser that somebody else already started through a CDP
endpoint. The adapter launches only a short-lived Node/Puppeteer interaction
worker; it does not launch, stop, provision, or display a browser.

The reference page is [examples/web-presentation](../examples/web-presentation).
It exposes `window.jangolova` and can render either structured documents or
authored HTML/CSS/JavaScript.

Implemented operations:

- `presentation.create` and `presentation.replace` for structured documents or
  HTML-source documents;
- `presentation.write` for `{html, css, js}` source;
- `presentation.execute` for incremental JavaScript;
- `presentation.patch` with bounded `set`, `remove`, and `append` operations;
- `presentation.describe`, `presentation.activate`, and `presentation.capture`;
- cursor-based `events` for presentation actions and resize events.

The adapter is registered as `web-presentation` and is discoverable through
`jangolova engines --json`. The provider handoff is documented in
[presentation-provider.md](presentation-provider.md).

Validation already run:

```bash
GOCACHE=/tmp/jangolova-go-build-cache \
GOMODCACHE=/tmp/jangolova-go-mod-cache go test -race ./...
npm run test:browser-worker
node --test tests/presentation-worker-contract-test.mjs
```

## Important boundary

Jangolova owns presentation definitions, generated source, semantic actions,
events, and engine integrations. Xallet or the native host owns:

- serving the HTML/JS assets or presentation URL;
- Chromium, Firefox, WebKit, Unity, Unreal, or other runtime processes;
- CDP/WebDriver/MCP endpoints and address translation;
- windows, surfaces, display placement, GPU, VNC/WebRTC, and input injection;
- credentials, network policy, container/VM placement, and cleanup.

The adapter must remain target-preserving: disconnecting Jangolova cannot stop
the caller-owned runtime.

## What is still missing

### 1. Prove the authored path against a real browser

Add a live smoke test that starts a test-owned Chromium, serves
`examples/web-presentation`, connects `web-presentation`, calls
`presentation.write`, verifies the resulting DOM and event, captures a PNG,
then disconnects and verifies Chromium is still alive. Keep this under test
fixtures; do not move browser launch back into the adapter.

### 2. Decide the presentation artifact model

The current `presentation.write` accepts arbitrary source strings. Decide and
document whether production callers will use:

- inline `{html, css, js}`;
- a versioned presentation bundle URL;
- an artifact manifest with separate asset URLs;
- or all three with explicit size and origin limits.

Then add size limits, source origin policy, asset loading rules, and version or
revision identifiers.

### 3. Harden JavaScript execution

`presentation.execute` intentionally runs code inside the target page and is
marked externally effectful. Add provider policy hooks, audit records,
timeouts, cancellation behavior, and a clear distinction between trusted
presentation code and untrusted agent-authored code. Avoid silently treating
arbitrary JavaScript as a safe semantic action.

### 4. Improve document semantics

The reference renderer is deliberately small. Define a stable document schema
for layout, text, media, controls, and accessible names. Add validation and
revision-aware patching so concurrent agents cannot overwrite each other's
changes. Add richer `describe` output containing semantic nodes and action
schemas.

### 5. Connect Three.js as a first-class presentation host

`examples/threejs-scene` already exposes scene actions and pointer events, but
it is not converted to the new `presentation.*` document contract. Decide
whether Three.js remains an engine-specific bridge or also implements the
common presentation artifact lifecycle. Add an adapter/fixture if common
operations are required.

### 6. Add Unity and Unreal provider attachment

Unity has a native bridge package, but provider-level attachment is still
unchecked in the roadmap. Define the endpoint/handle shape Xallet will pass,
add an adapter conformance test, and implement the Unreal plugin against the
same hello/capabilities/describe/act/events contract.

### 7. Build the display-level fallback

The current browser adapters are semantic/runtime adapters. The planned
provider-neutral display adapter still needs `display.describe`,
`display.capture`, pointer, keyboard, focus, coordinate-space, and input-policy
operations. Xallet should own the VNC/WebRTC/OS mechanism; Jangolova should
translate approved agent intent into that contract.

### 8. Harden operations and packaging

Remaining production work includes per-capability authorization, audit logs,
reconnection/orphan recovery, generated protocol clients, compatibility
fixtures, and versioned runtime packaging. The un-published image name should
remain `jangolova/engine-runtime` until a release process exists.

## Recommended next build order

1. Add the real Chromium authored-presentation smoke test.
2. Add source size/origin/revision policy and update the provider contract.
3. Add policy/audit hooks around `presentation.execute` and capture.
4. Normalize the document schema and semantic `describe` response.
5. Decide and implement the Three.js common-host path.
6. Continue with Unity/Unreal attachment, then display-level input.

## Useful files

- Adapter: `adapters/webpresentation/adapter.go`
- Worker: `scripts/presentation-worker.mjs`
- Reference host: `examples/web-presentation/main.js`
- Provider API docs: `docs/engine-provider.md`
- Xallet contract: `docs/presentation-provider.md`
- Product model: `docs/interface-model.md`
- Broad checklist: `docs/roadmap.md`
