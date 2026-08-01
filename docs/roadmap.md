# Roadmap

## Phase 1: Correct interaction boundary

- [x] Accept caller-owned target endpoints and opaque handles.
- [x] Remove Chromium, native-process, and display-runtime launch adapters.
- [x] Make disconnect target-preserving.
- [x] Add a repository boundary test against target provisioning.
- [x] Keep native-host, independent-container, and Xallet-managed operation.

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

## Phase 3: Presentation engines

- [x] Keep the Three.js dynamic-scene experience and protocol.
- [x] Keep the Unity package and authenticated native bridge.
- [x] Package a web presentation host behind the provider-visible
  `web-presentation` adapter (HTML/CSS/JS write plus
  create/replace/patch/describe/act/capture/events).
- [x] Add a live Chromium authored-presentation smoke test and verify
  target-preserving disconnect.
- [x] Define artifact size, origin, revision, and asset-loading policy.
- [ ] Add authorization, audit, timeout, and cancellation hooks for authored
  JavaScript execution.
- [ ] Expose Unity bridge attachment through the interaction provider.
- [ ] Add an Unreal plugin implementing the same semantic contract.

## Phase 4: General display interaction

- [ ] Define provider-neutral capture, coordinate-space, focus, pointer, and
  keyboard target contracts.
- [ ] Add display observation and pointer/keyboard semantic capabilities.
- [ ] Add policy metadata for coordinate clicks, typing, and sensitive input.
- [ ] Verify native-host and Xallet-provided display contracts with the same
  adapter conformance suite.

## Phase 5: Hardening

- [ ] Add per-capability policy hooks and audit records.
- [ ] Add orphan recovery and reconnection.
- [ ] Add generated protocol clients and compatibility fixtures.
- [ ] Sign and publish versioned interaction-runtime images.
