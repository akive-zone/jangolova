# Jangolova

Jangolova is a deployment-neutral interaction and presentation engine toolkit.
It uses Playwright, Puppeteer, Three.js, Unity, Unreal, and cooperative bridge
integrations to observe, operate, and present through caller-owned targets.

Jangolova does not provision Chromium, native applications, displays,
containers, VMs, networking, or credentials. Xallet owns those concerns when
the products run together; a native user or another operator can provide the
same target endpoints and handles without Xallet.

## Included interaction engines

- Playwright attachment to a caller-owned Chromium-compatible CDP target.
- Puppeteer attachment to the same target contract.
- Agent-facing `hello`, `capabilities`, `describe`, `act`, and `events` calls.
- Three.js dynamic presentation example.
- Authenticated cooperative bridge and Unity Package Manager integration.
- Target-preserving disconnect, active health, and lifecycle events.

## Commands

Discover installed interaction engines:

```bash
jangolova engines
jangolova engines --json
```

Attach directly to a browser already started by the native host or Xallet:

```bash
jangolova connect-engine \
  --adapter playwright \
  --target-kind browser \
  --endpoint cdp=http://127.0.0.1:9222
```

`connect-engine` disconnects Jangolova when interrupted; it does not terminate
the browser.

Run the authenticated provider:

```bash
export JANGOLOVA_PROVIDER_TOKEN="replace-with-a-random-secret"
jangolova serve-engine-provider --bind 127.0.0.1:7391
```

The provider accepts caller-owned targets, creates interaction instances, and
exposes their semantic calls at `POST /v1/instances/{id}/call`.

## Ownership boundary

```text
Agent -> Jangolova interaction engine -> caller-owned target
             Playwright                    Chromium
             Puppeteer                     native application
             Three.js                      display/runtime
             Unity/Unreal bridge
```

The repository boundary test prevents Chromium launch, native-process launch,
surfaces, VNC, sessions, container placement, and other target-runtime concerns
from returning to Jangolova product code. Test fixtures may create temporary
targets solely to verify attachment portability.

See [Architecture](docs/architecture.md), [Interaction provider](docs/engine-provider.md),
[Deployment modes](docs/deployment-modes.md), [Bridge protocol](docs/bridge-protocol.md),
and [Xallet boundary](docs/xallet-boundary.md).

## Tests

```bash
go test ./...
npm run test:browser-worker
npm run test:unity-package
```

The optional container fixture is documented in
[tests/docker/README.md](tests/docker/README.md). Docker is not required by
Jangolova itself.
