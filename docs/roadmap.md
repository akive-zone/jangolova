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
- [ ] Add WebDriver BiDi interaction adapters for WebKit and Gecko targets.
- [ ] Add richer page observation and accessibility descriptions.

## Phase 3: Presentation engines

- [x] Keep the Three.js dynamic-scene experience and protocol.
- [x] Keep the Unity package and authenticated native bridge.
- [ ] Package Three.js presentations behind a provider-visible adapter.
- [ ] Expose Unity bridge attachment through the interaction provider.
- [ ] Add an Unreal plugin implementing the same semantic contract.

## Phase 4: Hardening

- [ ] Add per-capability policy hooks and audit records.
- [ ] Add orphan recovery and reconnection.
- [ ] Add generated protocol clients and compatibility fixtures.
- [ ] Sign and publish versioned interaction-runtime images.
