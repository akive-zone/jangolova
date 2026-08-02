# Cymonkey augmented browsing subsystem

Cymonkey is Jangolova’s transport-neutral augmented-browsing subsystem. Jangolova
owns backend discovery/selection, authentication, storage/network/event services,
and control-plane routing. Cymonkey contributes the augmentation and page-level
semantics, then consumes those platform services.

It is a semantic contract over a caller-owned browser, not an owner of browser
processes, profiles, credentials, or lifecycle. CDP, WebDriver BiDi, Safari MCP,
and an optional Jangolova Browser Extension backend can implement the same
contract.

The protocol version is `jangolova.cymonkey/v1alpha1`. Its five operations are
also exposed to cooperating page code as:

```js
window.jangolova.cymonkey = {
  hello,
  capabilities,
  describe,
  act,
  events,
};
```

The page global is deliberately less capable than the engine control plane. It
contains page-safe DOM, overlay, description, and event operations only. It never
exposes raw `chrome.*`, raw `browser.*`, arbitrary protocol commands, extension
storage, request interception, or privileged Passthrough.

## Ownership model at a glance

- **Jangolova owns** browser backend discovery and selection, authenticated control
  plane, storage/naming/network/event service infrastructure, browser permissions
  strategy, and lifecycle.
- **Cymonkey owns** augmentation lifecycle, DOM/script/overlay/style operations,
  and page-safe semantic descriptions/events.
- **Caller owns** browser process/profile/install state/process lifecycle.

## Ownership and lifecycle

The target owner or provider owns the browser process, profile, credentials,
extension installation, display, network placement, CDP/BiDi/MCP endpoints,
and endpoint authentication. Jangolova only attaches to those supplied
resources. Disconnecting Cymonkey must not close the browser, remove an
extension, or destroy its profile.

An extension is optional. When extension mode is `auto`, Jangolova attaches to
CDP, BiDi, or Safari MCP first and then probes for an extension backend where
that transport supports a safe probe. A missing extension reduces the
negotiated capability set. It is an error only when extension mode is
`required`. Installation is never attempted by Jangolova.

## Architecture

```text
application or agent
        |
        | jangolova.cymonkey/v1alpha1
        | hello / capabilities / describe / act / events
        v
Go Cymonkey adapter
        |
        +-- selection policy: auto | cdp | bidi | safari-mcp
        +-- capability/origin policy
        +-- capability merger and event cursor
        |
        +-- CDP backend ---------------- Runtime / Page / DOM / CSS / Network / Fetch
        +-- BiDi backend --------------- script / browsingContext / network
        +-- Safari MCP mapper ---------- dynamically discovered safe tool mappings
        +-- optional WebExtension ------ scripting / storage / DNR / persistent state
                                                    |
                                                    +-- isolated content script
                                                    +-- page bootstrap bridge
```

Hybrid operation merges capabilities from a base transport and the optional
extension. Capability names remain stable; `capabilities` reports which backend
will handle each capability and whether another backend is available as a
fallback.

## Backend selection policy

The default backend is `auto`:

1. Prefer a supplied CDP endpoint as the no-install baseline.
2. Otherwise use a supplied WebDriver BiDi endpoint as a first-class backend.
3. Otherwise use a supplied Safari MCP Streamable HTTP endpoint and negotiate
   the safely mapped subset from its discovered tools.
4. If extension mode is `auto` or `required`, probe the optional extension on a
   compatible base transport and merge its persistent/privileged capabilities.
5. Reject the connection when required capabilities cannot be satisfied after
   probing and policy filtering.

An explicit backend option restricts selection rather than changing the
semantic API. An explicit extension policy is one of:

| Value | Meaning |
| --- | --- |
| `auto` | Use the extension if detected; otherwise continue with reduced capabilities. |
| `disabled` | Do not probe or use an extension. |
| `required` | Fail unless the expected extension handshakes successfully. |

Example:

```json
{
  "backend": "auto",
  "extension": {
    "mode": "auto",
    "id": "optional-provider-supplied-id"
  },
  "policy": {
    "allowedCapabilities": ["dom.query", "overlay.mount", "script.execute"],
    "allowedOrigins": ["https://*.wikipedia.org"]
  }
}
```

## Protocol operations

| Operation | Result |
| --- | --- |
| `hello` | Protocol version, implementation, selected backends, and features. |
| `capabilities` | Negotiated, policy-filtered semantic capability descriptors. |
| `describe` | Current backend state, target contexts, augmentations, and extension presence. |
| `act` | Execute one advertised semantic capability. |
| `events` | Non-destructively read events after an opaque cursor. |

Every advertised capability contains:

```json
{
  "name": "script.register",
  "description": "Register a script for matching future documents.",
  "backend": "webextension",
  "support": "native",
  "lifetime": "profile",
  "persistence": "persistent",
  "effect": "external",
  "inputSchema": {"type": "object", "required": ["augmentationId", "script"]},
  "alternatives": ["cdp"]
}
```

`support` is `native`, `mapped`, or `emulated`. `lifetime` is `call`,
`document`, `browser-session`, or `profile`. `persistence` is `ephemeral`,
`session`, or `persistent`. These fields describe behavior; they do not grant
authorization.

## Semantic capability set

The `v1alpha1` vocabulary includes:

- `augmentation.install`, `augmentation.update`, `augmentation.uninstall`
- `augmentation.enable`, `augmentation.disable`, `augmentation.list`,
  `augmentation.describe`
- `script.execute`, `script.register`, `script.unregister`
- `style.insert`, `style.remove`
- `dom.query`, `dom.observe`, `dom.patch`
- `overlay.mount`, `overlay.patch`, `overlay.unmount`
- `network.observe`, `network.rules.install`, `network.rules.remove`
- `storage.get`, `storage.set`

A backend advertises only operations it actually supports after runtime
probing. For example, a Safari MCP endpoint with click, type, and screenshot
tools does not thereby advertise augmentation support.

## Backend mapping matrix

| Semantic area | CDP | WebDriver BiDi | Safari MCP | WebExtension |
| --- | --- | --- | --- | --- |
| script execute | `Runtime.evaluate` | `script.evaluate` / `script.callFunction` | mapped evaluate tool | `scripting.executeScript` |
| script register | `Page.addScriptToEvaluateOnNewDocument` | `script.addPreloadScript` | only a discovered preload tool | `scripting.registerContentScripts` |
| script unregister | `Page.removeScriptToEvaluateOnNewDocument` | `script.removePreloadScript` | only a matching removal tool | `scripting.unregisterContentScripts` |
| DOM query/patch | DOM and Runtime domains | `browsingContext.locateNodes` or script | mapped DOM/evaluate tools | isolated content script |
| styles | CSS/Runtime domains | script mapping when supported | mapped style/evaluate tool | `scripting.insertCSS/removeCSS` |
| network observe | Network events | network events | discovered network observation tool | browser events when permitted |
| network rules | Fetch interception, session lifetime | `network.addIntercept/removeIntercept` and actions | only explicit intercept tools | declarativeNetRequest, persistent |
| storage | page/origin storage, limited lifetime | page/origin storage when script is supported | only explicit storage tools | extension-scoped persistent storage |

CDP and BiDi implementations must probe actual browser support. A protocol
method appearing in a specification is not sufficient reason to advertise it.

## Augmentation manifest and persistence

An augmentation is a versioned semantic declaration. The schema is
`protocol/cymonkey/v1alpha1/augmentation.schema.json`.

```json
{
  "apiVersion": "jangolova.cymonkey/v1alpha1",
  "kind": "Augmentation",
  "metadata": {
    "id": "wikipedia-reading-tools",
    "revision": "sha256:abc123"
  },
  "spec": {
    "matches": ["https://*.wikipedia.org/wiki/*"],
    "permissions": ["dom.query", "overlay.mount", "script.register"],
    "scripts": [{
      "id": "main",
      "source": "globalThis.wikipediaReadingTools = true;",
      "world": "ISOLATED",
      "runAt": "document_start"
    }]
  }
}
```

The same document can be submitted to CDP and BiDi. Its achieved persistence
depends on the negotiated backend:

- CDP/BiDi registration survives navigation while the attachment remains
  active, but does not promise survival after browser restart.
- Safari MCP persistence is whatever the explicitly mapped tool reports.
- WebExtension registration and extension storage can survive attachment and
  browser restarts, subject to browser/profile policy.

Callers that require a persistence level must request it as a required
capability constraint and verify the returned capability descriptor.

## Trust boundary and control planes

Website content is hostile. Page messages, DOM state, URLs, and page-provided
objects are untrusted. The page global is never an authentication mechanism.

Privileged extension commands use a control plane independent of the page. The
preferred production shape is an extension-initiated authenticated WebSocket
using a caller-supplied endpoint and short-lived token. Backends may instead
use provider-controlled native messaging or CDP evaluation in an extension
service-worker/extension-origin target. The current development backend uses
the latter and verifies the expected extension origin and implementation
handshake. It never dispatches privileged commands through `window.postMessage`.

The extension consists of:

- a Manifest V3 service worker owning scripting, storage, and declarative
  network request operations;
- an isolated content script implementing bounded page operations;
- a main-world bootstrap that creates `window.jangolova.cymonkey`;
- an extension-origin control entry point for the backend handshake.

The Xallet spoke build additionally registers with the provider-installed
`Xallet Hub`; it accepts external privileged calls only from the discovered,
enabled hub ID. It still operates standalone when the hub is absent.

Jangolova controls extension ownership and backend identity separately from page-safe
operations. Pacman can be used by augmentations for 3D presentation control, but
Pacman receives only explicitly registered semantic resources and action allowlists.
Cymonkey never maps or drives arbitrary scene objects directly.

## Migration to the Jangolova Browser Extension package

`pkg/browser-cymonkey` remains supported as the legacy path. A compatibility
bridge package is provided at `pkg/browser-jangolova` with the same source and an
explicit migration note for external operators:

- New references should use `pkg/browser-jangolova`.
- Existing scripts that import `pkg/browser-cymonkey` continue to work unchanged.
- Extension control channels and protocol names remain stable (`jangolova.cymonkey/*`).

## Policy requirements

1. Filter capabilities before advertising them and again before dispatch.
2. Apply origin policy to every target context, navigation, augmentation match,
   and network rule.
3. Never expose raw protocol, MCP, or browser-extension API evaluation through
   the public semantic contract.
4. Namespace scripts, styles, rules, storage, and overlays by augmentation ID.
5. Prevent one augmentation from replacing another augmentation's resources.
6. Do not return page contents, source code, credentials, request bodies, or
   extension storage in descriptions/events unless an explicit capability and
   policy allow it.
7. Preserve the caller-owned browser lifecycle on success, error, reconnect,
   and disconnect.

## Browser extension package

The optional backend source currently lives in `pkg/browser-cymonkey` and
`pkg/browser-jangolova` (compatibility bridge), and uses WXT. Build it with:

```sh
npm install --prefix pkg/browser-jangolova
npm --prefix pkg/browser-jangolova run check
```

If you are continuing an existing integration, `pkg/browser-cymonkey` remains
supported and runs the same build outputs. A migration path can use the same
generated artifacts.

## Connection examples

No-install CDP baseline:

```sh
jangolova connect-engine \
  --adapter cymonkey \
  --target-kind browser \
  --endpoint cdp=http://127.0.0.1:9222 \
  --options '{"backend":"auto","extension":{"mode":"auto"}}'
```

First-class BiDi baseline:

```sh
jangolova connect-engine \
  --adapter cymonkey \
  --target-kind browser \
  --endpoint webdriver-bidi=ws://127.0.0.1:9222/session \
  --options '{"backend":"bidi","extension":{"mode":"disabled"}}'
```

Require a provider-installed extension on CDP:

```sh
jangolova connect-engine \
  --adapter cymonkey \
  --target-kind browser \
  --endpoint cdp=http://127.0.0.1:9222 \
  --options '{"extension":{"mode":"required","id":"replace-with-installed-extension-id"}}'
```

Safari MCP uses the same adapter with a caller-owned
`mcp-streamable-http` endpoint. It advertises only the semantic subset derived
from the server's discovered tools.

## Live conformance fixtures

`tests/cymonkey-live-client.mjs` runs one reversible augmentation lifecycle
against either backend. It validates the handshake and capability metadata,
installs/lists/disables/enables/uninstalls the same augmentation document,
queries the DOM, mounts and removes an overlay, checks session storage and
events, and cleans up in a `finally` block. The CDP run additionally installs
and removes a deliberately non-matching owned interception rule.

The target-owning Docker fixtures execute it against Chromium CDP and Firefox
WebDriver BiDi:

```sh
npm run test:cymonkey:live:cdp
npm run test:cymonkey:live:bidi
```

Both fixtures verify that deleting the Cymonkey interaction instance leaves
the independently owned browser process alive.

## Delivery sequence

1. Protocol, trust boundary, ownership, backend matrix, persistence, and policy.
2. Versioned protocol and augmentation schemas.
3. Go backend interface and backend-selection policy.
4. CDP backend and nested page bridge.
5. WebDriver BiDi backend.
6. Safari MCP capability mapper.
7. Optional WebExtension backend and authenticated control plane.
8. Shared conformance and contract tests.
