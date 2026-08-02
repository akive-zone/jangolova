# Jangolova

Jangolova is a deployment-neutral interaction and presentation engine toolkit.
It uses Playwright, Puppeteer, Three.js, Unity, Unreal, and cooperative bridge
integrations to observe, operate, and present through caller-owned targets.

Its product goal has two equal parts: operate existing interfaces—including
clicking and typing through semantic or display-level contracts—and create
dynamic 2D/3D interfaces that agents can present and update.

Jangolova does not provision Chromium, native applications, displays,
containers, VMs, networking, or credentials. Xallet owns those concerns when
the products run together; a native user or another operator can provide the
same target endpoints and handles without Xallet.

## Included interaction engines

- Playwright attachment to a caller-owned Chromium-compatible CDP target.
- Puppeteer attachment over CDP or WebDriver BiDi, including Firefox.
- WebDriver Classic attachment to an existing caller-owned session, including
  Safari's `safaridriver`.
- Named WebKit WebDriver attachment for WebKitGTK, WPE WebKit, and Safari.
- Safari MCP attachment through a caller-owned Streamable HTTP relay.
- Agent-facing `hello`, `capabilities`, `describe`, `act`, and `events` calls.
- Three.js dynamic presentation example.
- `web-presentation` declarative presentation adapter for caller-owned CDP browsers.
- Authenticated cooperative bridge and Unity Package Manager integration.
- Pacman semantic presentation attachment and an explicitly allowlisted Unity
  package; Unity/Unreal rendering and display transport remain external.
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

Connection references, expiry, private certificate authorities, and output
redaction are documented in
[target connection security](docs/target-connection-security.md).
Credential leases renew HTTP requests and reconnect CDP/BiDi workers without
replacing the interaction instance or caller-owned runtime.

## Ownership boundary

```text
Agent -> Jangolova interaction engine -> caller-owned target
             Playwright --- CDP ---------- Chromium
             Puppeteer ---- CDP/BiDi ----- Chromium/Firefox
             WebDriver ---- existing ----- WebKitGTK/WPE/Safari
             Safari MCP --- MCP relay ---- Safari 27 beta/STP
             Three.js/Unity/Unreal ------- presentation target
```

The repository boundary test prevents Chromium launch, native-process launch,
surfaces, VNC, sessions, container placement, and other target-runtime concerns
from returning to Jangolova product code. Test fixtures may create temporary
targets solely to verify attachment portability.

See [Architecture](docs/architecture.md), [Interaction provider](docs/engine-provider.md),
[Deployment modes](docs/deployment-modes.md), [Bridge protocol](docs/bridge-protocol.md),
[Pacman](docs/pacman.md),
[interface creation and operation](docs/interface-model.md),
[caller-supplied targets](docs/target-descriptor.md),
[browser target protocols](docs/browser-target-protocols.md), and
[Xallet boundary](docs/xallet-boundary.md).

## Tests

```bash
go test ./...
npm run test:browser-worker
npm run test:unity-package
npm run test:unity-pacman-package
npm run test:unreal-pacman-package
```

The optional container fixture is documented in
[tests/docker/README.md](tests/docker/README.md). Docker is not required by
Jangolova itself.
