# Roadmap

The roadmap is organized around a standalone engine toolkit that remains
compatible with Xallet and other operators.

## Phase 0: Preserve portability tests

Status: complete.

- Keep a headed-browser/Xvfb vertical slice as a reproducible test fixture.
- Keep all container topology under `tests/docker/`.
- Treat Xvfb as an externally supplied test dependency, not Jangolova
  product components.

## Phase 1: Establish the engine boundary

Status: in progress.

- [x] Define a registry of display-engine adapters.
- [x] Pass caller-resolved environment values directly to engine adapters.
- [x] Implement direct engine discovery and launch commands.
- [x] Implement an authenticated, versioned engine-provider API.
- [x] Return typed private endpoint metadata.
- [x] Document native-host, external-display, independent-container, and
  Xallet-managed modes.
- [x] Add readiness and unexpected-exit events to engine instances.
- [x] Define opaque native-handle launch inputs without giving Jangolova
  ownership of those handles.

Exit criterion: one adapter binary launches unchanged in every deployment mode
and never creates or publishes a display runtime.

## Phase 2: Complete browser engine coverage

- [x] Launch or attach to Chromium and report CDP.
- [x] Serve a local web project through Chromium.
- [ ] Add WebDriver BiDi endpoint discovery where supported.
- [ ] Add WebKit engine support.
- [ ] Add Gecko engine support.
- [ ] Define the role of SpiderMonkey as an embeddable runtime rather than a
  windowed browser.
- [x] Add adapter capability and executable-availability probes.

Exit criterion: callers can discover what is truly installed and launch each
supported browser family through one provider contract.

## Phase 3: Native 2D/3D engines

- [x] Define an engine-neutral cooperative bridge protocol.
- [x] Add a generic native-process lifecycle adapter.
- [x] Add an authenticated outbound WebSocket bridge host.
- [x] Add a Unity package proof of concept.
- [ ] Validate the Unity integration in Editor and built players on supported
  native and external displays.
- [ ] Add an Unreal Engine plugin proof of concept.
- [x] Add engine-specific capability and active health reporting for current
  adapters.

Exit criterion: Unity and Unreal examples run natively, in an independently
configured environment, and when launched by Xallet without adapter forks.

## Phase 4: Provider hardening

- [x] Add a bounded cursor-addressed lifecycle event journal and status
  transitions.
- [x] Add adapter-specific active health probes.
- [ ] Add graceful recovery and orphan detection.
- [ ] Add provider protocol compatibility tests and generated client fixtures.
- [ ] Add resource limits and concurrency policy hooks supplied by the
  operator.
- [ ] Add signed release artifacts and versioned engine-runtime images.

## Phase 5: Retire combined-session ownership

- [x] Remove the legacy session/surface/controller/connector APIs.
- [x] Remove display-runtime examples and production configuration.
- [x] Replace Jangolova controller calls with endpoint metadata consumed by the
  caller.
- [x] Remove the legacy agent runtime and controller smoke harnesses.
- [x] Add a repository boundary test preventing display-runtime ownership from
  returning to product packages.

Exit criterion: the Jangolova product surface contains only engine and
engine-integration concerns; display-runtime orchestration lives entirely with
its operator.
