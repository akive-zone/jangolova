# Architecture

Jangolova is the display-engine side of the system. It starts or attaches to
rendering engines and reports engine-local endpoints. It does not create the
display runtime in which those engines execute.

## Active system boundary

```text
Caller or operator
  (shell, test harness, Xallet, or another orchestrator)
              |
              | engine spec + resolved runtime environment
              v
       Jangolova engine provider
              |
              v
         engine adapter
  (Chromium, web project, native process,
       future Unity / Unreal adapters)
              |
              v
   engine instance + typed endpoints
              |
              v
Caller-owned control, display, and publication
```

The caller decides placement and constructs the display environment before it
asks Jangolova to launch an engine. The caller may be a local user, a VM or
container wrapper, a test harness, Xallet, or another compatible system.

Jangolova has no dependency on Xallet. Xallet can nevertheless use exactly the
same engine-provider contract as standalone callers.

## Ownership

Jangolova owns:

- display-engine discovery and lifecycle;
- engine-specific launch and attach behavior;
- engine-local profiles and child processes when requested;
- cooperative web and native engine bridges;
- typed endpoint discovery, such as CDP or a semantic bridge;
- cleanup of resources created inside an engine adapter.

The operator owns:

- physical, virtual-machine, and OCI placement;
- X11, Wayland, native desktop, framebuffer, and other display runtimes;
- container networks, mounts, devices, secrets, and port publication;
- VNC, WebRTC, capture, input routing, access policy, and session state;
- translation of private engine endpoints into client-reachable endpoints.

When Xallet is the operator, all operator responsibilities above belong to
Xallet. A standalone user can provide the same inputs directly.

## Engine launch contract

An engine adapter receives:

1. an adapter name;
2. an engine source, such as a URL, project directory, or executable;
3. engine-specific options;
4. caller-resolved environment values;
5. caller-owned opaque handles.

The environment is ordinary launch input. For example, `DISPLAY=:99` tells an
engine where an existing X11 server is; it does not authorize the adapter to
create or destroy that server. Named opaque handles travel beside environment
values for adapters that understand native windows, views, layers, devices, or
other runtime objects. A handle remains owned by its caller.

An engine instance supports stop and may report typed endpoints, active health,
and lifecycle events. Adapter discovery reports actual availability and
engine-local capabilities. Endpoint metadata describes the engine-side address
and target port. It does not imply that Jangolova publishes that address outside
its current network namespace.

## Execution modes

The engine code is identical in every mode:

- **Native host:** inherit the host environment and native display.
- **External display:** receive `DISPLAY`, `WAYLAND_DISPLAY`, or equivalent
  values from a caller.
- **Independent container:** receive environment, mounts, devices, and network
  configuration from any OCI operator.
- **Xallet-managed:** receive placement-resolved values from Xallet and return
  private engine endpoints for Xallet to expose or control.

See [Deployment modes](deployment-modes.md) for runnable examples.

## Provider protocol

The authenticated `jangolova.engine/v1alpha1` HTTP provider is the primary
process boundary. It exposes engine inventory and launch/get/stop operations.
It may run on loopback as a standalone daemon or on a private application
network managed by Xallet or another operator.

The direct `launch-engine` command uses the same adapter contract without an
HTTP daemon. This keeps native development and portability tests independent
of an orchestrator.

## Package direction

```text
cmd/jangolova/          direct CLI and engine-provider daemon
internal/engineprovider/ versioned provider protocol and service
internal/orchestrator/  engine lifecycle and caller-supplied runtime contract
internal/bridge/        engine-cooperative semantic protocol
adapters/               concrete display-engine adapters
integrations/           engine-side web and native integrations
deploy/engine-runtime/  optional engine artifact, not runtime topology
tests/docker/           reproducible test fixture only
```

Concrete adapters depend on the core engine contract. The provider depends on
the registry, but neither the contract nor adapters import Xallet.

## Removed compatibility layer

The original combined `Session` manifest, surface lifecycle, controllers,
connectors, JSONL runner, and agent runtime are no longer part of Jangolova.
Existing display-session behavior belongs in Xallet or another caller. The
repository boundary test prevents those product responsibilities from being
reintroduced outside test fixtures.

## Security boundaries

- Provider and engine-control endpoints bind to loopback unless an operator
  explicitly places them on a protected private network.
- Provider lifecycle operations require bearer authentication.
- Jangolova returns private endpoint metadata; the operator controls external
  publication and authorization.
- Secrets and persistent credentials remain external launch inputs rather than
  source-controlled engine specifications.
- Cooperative engine capabilities are descriptive data, not authorization.
