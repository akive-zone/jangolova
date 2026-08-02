# Roadmap

## Phase 0: Grimlock agent interface

- [x] Define Grimlock as Jangolova's internal model-powered agent subsystem,
  distinct from deterministic engine APIs and target lifecycle ownership.
- [x] Define a separate caller-supplied model profile with opaque credential
  and TLS references.
- [x] Add the ADK Go agent factory, model-connector registry, and initial
  OpenAI-compatible gateway connector.
- [x] Adapt Jangolova capabilities into effect-classified ADK tools with
  approval checks immediately before execution.
- [ ] Add the native Grimlock HTTP session/run/event API.
- [ ] Add MCP, ACP, and A2A adapters over the same Grimlock application service.
- [ ] Add persistent agent sessions, budgets, tracing, and multi-agent
  workflows.

## Phase 1: Correct interaction boundary

- [x] Accept caller-owned target endpoints and opaque handles.
- [x] Remove Chromium, native-process, and display-runtime launch adapters.
- [x] Make disconnect target-preserving.
- [x] Add a repository boundary test against target provisioning.
- [x] Keep native-host, independent-container, and Xallet-managed operation.
- [x] Define a provider-neutral caller-supplied target descriptor.
- [x] Select interaction engines automatically from protocols and required
  capabilities without using target location.
- [x] Resolve opaque credential and TLS references into expiring,
  adapter-private connection material.
- [x] Verify authenticated remote CDP attachment, release, TLS trust, and
  provider-output redaction.

## Phase 2: Browser interaction engines

- [x] Add Playwright Core attachment over CDP.
- [x] Add Puppeteer Core attachment over CDP.
- [x] Expose `hello`, `capabilities`, `describe`, `act`, and `events` through
  the authenticated provider.
- [x] Add active connection health and lifecycle events.
- [x] Verify disconnect does not terminate caller-owned Chromium.
- [x] Add Puppeteer attachment over WebDriver BiDi and verify it against
  caller-owned Firefox.
- [x] Add target-preserving W3C WebDriver Classic attachment for existing
  Safari and other browser sessions.
- [x] Add a named WebKit WebDriver adapter and verify it against a caller-owned
  WebKitGTK session.
- [x] Add a Safari MCP client over a caller-owned Streamable HTTP relay with
  dynamic tool discovery.
- [ ] Add a Safari 27/STP live fixture when that runtime is available in CI.
- [ ] Add richer page observation and accessibility descriptions.

## Phase 2b: Augmented browsing

- [x] Define the Cymonkey page-safe and privileged-extension trust boundary.
- [x] Add the nested `window.jangolova.cymonkey` page bridge.
- [x] Define `jangolova.cymonkey/v1alpha1`, the augmentation schema, backend
  interface, auto-selection policy, and capability persistence metadata.
- [x] Add no-install CDP and first-class WebDriver BiDi mappings for the same
  augmentation contract.
- [x] Run the same reversible augmentation lifecycle against test-owned
  Chromium CDP and Firefox WebDriver BiDi targets while proving target-preserving disconnect.
- [x] Add dynamic Safari MCP mapping that advertises only explicitly supported
  script, preload, DOM, and network semantics.
- [x] Add packaged script registration/execution, CSS injection, extension
  storage, Shadow DOM overlays, and declarative network rules.
- [x] Detect an optional caller-installed Chromium extension through a
  caller-owned CDP target without launching or stopping the browser.
- [x] Add WXT Manifest V3 packaging for Chrome, Edge, and Firefox, including
  standalone and Xallet spoke modes.
- [ ] Add live cross-browser extension fixtures.
- [x] Add provider-level capability and origin filtering.
- [ ] Add signed augmentation bundles and an extension-initiated authenticated
  WebSocket control option.

## Phase 3: Presentation engines

- [x] Keep the Three.js dynamic-scene experience and protocol.
- [x] Keep the Unity package and authenticated native bridge.
- [x] Package a web presentation host behind the provider-visible
  `web-presentation` adapter (HTML/CSS/JS write plus
  create/replace/patch/describe/act/capture/events).
- [x] Add a live Chromium authored-presentation smoke test and verify
  target-preserving disconnect.
- [x] Define artifact size, origin, revision, and asset-loading policy.
- [x] Add provider-neutral artifact references and `presentation.mount`.
- [x] Verify artifact mounting in a direct-container target without Xallet.
- [x] Add authorization, audit, timeout, and cancellation hooks for authored
  JavaScript execution.
- [x] Define the Pacman Godot/Unity/Unreal semantic protocol and lifecycle boundary.
- [x] Add Godot as the license-free Pacman reference runtime.
- [x] Add a headless Godot 4 fixture and authenticated `pacman-ws` container.
- [x] Expose authenticated caller-owned `pacman-ws` attachment through the
  interaction provider.
- [x] Add a minimal Unity Pacman package with explicit resource/action
  allowlisting and target-preserving disconnect verification.
- [x] Scaffold a distributable Unreal plugin with explicit resource/action
  registration and game-thread semantic dispatch.
- [x] Add the authenticated Unreal WebSocket host and game-thread request
  router boundary.
- [x] Add a separate source-only Unreal fixture project and packaged-runtime
  image definition.
- [ ] Add a platform-specific Unreal listener/upgrade binding and live
  packaged-game conformance fixture.
- [ ] Harden and platform-test the Unity WebSocket listener beyond the .NET 4.x
  MVP transport.

## Phase 4: General display interaction

- [ ] Define provider-neutral capture, coordinate-space, focus, pointer, and
  keyboard target contracts.
- [ ] Add display observation and pointer/keyboard semantic capabilities.
- [ ] Add policy metadata for coordinate clicks, typing, and sensitive input.
- [ ] Verify native-host and Xallet-provided display contracts with the same
  adapter conformance suite.

## Phase 5: Hardening

- [x] Add renewable credential leases for HTTP and long-lived CDP/BiDi
  interactions.
- [x] Add live CA and client-certificate transport rotation, with atomic HTTP
  transport promotion and process-safe CDP worker replacement.
- [ ] Extend per-capability policy hooks and audit records beyond the current
  presentation execute/capture path.
- [x] Reattach failed engine instances to the same caller-owned target with
  bounded backoff and action-safe lifecycle events.
- [ ] Add caller reconciliation fixtures for rebuilding desired interaction
  instances after the Jangolova provider process restarts.
- [ ] Add generated protocol clients and compatibility fixtures.
- [ ] Sign and publish versioned interaction-runtime images.
