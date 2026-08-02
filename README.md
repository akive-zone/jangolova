# Jangolova

Jangolova is a deployment-neutral interaction and presentation engine toolkit.
It uses Playwright, Puppeteer, Three.js, Unity, Unreal, and cooperative bridge
integrations to observe, operate, and present through caller-owned targets.

Its product goal has two equal parts: operate existing interfaces—including
clicking and typing through semantic or display-level contracts—and create
dynamic 2D/3D interfaces that agents can present and update.

Grimlock is Jangolova's model-powered agent subsystem. It accepts a
caller-supplied model gateway and opaque credentials, then turns approved
Jangolova capabilities into effect-aware agent tools. HTTP, MCP, ACP, and A2A
sit at its northbound boundary. Deterministic callers can continue using the
engine API directly.

Jangolova does not provision Chromium, native applications, displays,
containers, VMs, networking, or credentials. Xallet owns those concerns when
the products run together; a native user or another operator can provide the
same target endpoints and handles without Xallet.

## Included interaction engines

- Playwright attachment to a caller-owned Chromium-compatible CDP target.
- Puppeteer attachment over CDP or WebDriver BiDi, including Firefox.
- Cymonkey transport-neutral augmented browsing over CDP, WebDriver BiDi, or a
  negotiated Safari MCP subset, with an optional Jangolova Browser Extension for
  persistent scripts, storage, and declarative network rules.
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

Cymonkey needs no extension for its CDP or WebDriver BiDi baseline. To add the
optional persistent Jangolova Browser Extension backend, build it with WXT and have
the target owner install the unpacked extension from
`pkg/browser-jangolova/.output/<browser>-mv3` (legacy: `pkg/browser-cymonkey/...`):

```bash
npm install --prefix pkg/browser-jangolova
npm --prefix pkg/browser-jangolova run check
```

For Xallet spoke variants, run `npm --prefix pkg/browser-jangolova run build:spoke` and
load the matching `<browser>-mv3-spoke` directory. Spoke builds still work standalone
when Xallet Hub is absent.

```bash
jangolova connect-engine \
  --adapter cymonkey \
  --target-kind browser \
  --endpoint cdp=http://127.0.0.1:9222 \
  --options '{"backend":"auto","extension":{"mode":"auto","id":"optional-installed-extension-id"}}'
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
Failed adapter attachments are re-created against the same caller-owned target
without restarting that target or replaying semantic actions; see
[attachment recovery](docs/attachment-recovery.md).

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
[Grimlock agent subsystem](docs/grimlock.md),
[Cymonkey augmented browsing](docs/cymonkey.md),
[Pacman](docs/pacman.md),
[interface creation and operation](docs/interface-model.md),
[caller-supplied targets](docs/target-descriptor.md),
[browser target protocols](docs/browser-target-protocols.md), and
[Xallet boundary](docs/xallet-boundary.md).

## Tests

```bash
go test ./...
npm run test:browser-worker
npm run test:cymonkey
npm run test:unity-package
npm run test:unity-pacman-package
npm run test:unreal-pacman-package
npm run test:unreal-pacman-fixture
```

The optional container fixture is documented in
[tests/docker/README.md](tests/docker/README.md). Docker is not required by
Jangolova itself.
