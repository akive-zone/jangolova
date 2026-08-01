# Vision

## Purpose

Jangolova makes display engines portable across hosts, virtual machines, and
containers. It provides one engine lifecycle and endpoint contract for browser
engines, native runtimes, and cooperative 2D/3D experiences.

The product promise is:

> Given an engine specification and an existing runtime environment, start or
> attach to the engine and return the endpoints needed to use it.

Jangolova deliberately stops at that boundary. It does not decide where a
workload is placed, create its display server, publish its ports, or own the
larger display session.

## Principles

### Standalone first, orchestrator compatible

Every engine adapter must work without Xallet: directly on a native host,
against a caller-provided display, or in an independently configured
container. Xallet uses the same contracts when it manages the workload.

### Engines consume runtime inputs

An engine can inherit a native desktop or receive values such as `DISPLAY` and
`WAYLAND_DISPLAY`. Those values identify runtime resources owned by the
caller. They are not surface objects managed by Jangolova.

### Engines are not forced into one API

Chromium, WebKit, Gecko, SpiderMonkey, Unity, Unreal, and web rendering
libraries have different lifecycle and control models. Jangolova provides a
small common launch/stop/endpoint contract while preserving engine-specific
options and capabilities.

### Cooperative presentation is engine-side

Web libraries and native engine plugins can implement the Jangolova bridge to
describe a scene, expose bounded actions, and emit events. CDP, VNC, capture,
input routing, and access policy remain responsibilities of the display
runtime or caller.

### Placement is external

Docker, Podman, Apple Container, Kubernetes, a VM manager, or a native process
launcher may place Jangolova. The repository can include build artifacts and
test fixtures, but production topology belongs to the operator. When the
operator is Xallet, Xallet owns that configuration.

### Private endpoints by default

Engine endpoints are local capabilities. Jangolova reports them without
assuming they are publicly reachable. The operator decides whether and how to
expose them.

## Initial engine families

1. Chromium launch and attach with CDP endpoint discovery.
2. Local web projects presented through Chromium.
3. Generic native processes with an authenticated cooperative bridge.
4. Unity and Unreal integrations using the same lifecycle and bridge model.
5. Future WebKit, Gecko, SpiderMonkey, and additional browser/runtime adapters.

## Non-goals

- Owning Xvfb, VNC, WebRTC, capture, or desktop input.
- Defining OCI networks, volumes, devices, port mappings, or placement policy.
- Acting as Xallet's scheduler or container-management layer.
- Replacing engine-native rendering and scene APIs.
- Hiding every engine feature behind a lowest-common-denominator interface.
- Providing unrestricted agent access to a desktop or engine process.
