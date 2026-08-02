# Cymonkey userscripts

Userscripts are user-installed JavaScript programs that run on matching web
documents. Cymonkey owns their semantic lifecycle as a high-trust augmentation
form. They are not a separate Jangolova subsystem, ordinary packaged scripts,
or a raw extension API.

The manifest payload is `jangolova.cymonkey.userscript/v1alpha1`. Its shared
validation and registration planning live in `pkg/userscript-runtime` and are
consumed by the Jangolova extension manager in WXT and the Safari WebExtension
embedded in `pkg/macos-ext`. Cymonkey augmentation manifests may carry these
payloads under `spec.web.userscripts`.

## User surface

The initial Cymonkey capabilities are:

- `userscript.install`
- `userscript.update`
- `userscript.uninstall`
- `userscript.enable`
- `userscript.disable`
- `userscript.list`
- `userscript.describe`

Callers discover these through Cymonkey `capabilities` and invoke them through
`act`, for example:

```json
{
  "method": "act",
  "params": {
    "name": "userscript.install",
    "input": {"manifest": {}, "approved": true}
  }
}
```

Cymonkey `describe` returns manager availability and source-free installed
script descriptions. Lifecycle notifications use Cymonkey `events`.

Installation accepts a manifest plus source. It never accepts browser API
objects or a request to bypass extension policy. A script has a stable ID,
revision, display name, match/exclude patterns, run timing, execution world,
declared grants, source provenance, and enabled state.

The MVP accepts `@grant none` only. Brokered Greasemonkey/Tampermonkey-style
grants require separate, schema-described Jangolova capabilities in a later
version. Undeclared network, storage, native messaging, and extension API
access are not inferred from source text.

## Installation and consent

Arbitrary userscript source is privileged. Installation therefore requires an
authenticated Cymonkey control-plane caller plus explicit approval, or a direct
user gesture in Jangolova UI. The page-safe projection of
`window.jangolova.cymonkey` does not advertise userscript capabilities.

Before enabling a script, the UI must show:

- name, namespace, version, description, and source origin;
- requested match and exclude patterns;
- execution world and run timing;
- declared grants;
- whether an update URL is configured.

Remote update URLs must use HTTPS. An update is staged and revalidated; it does
not silently widen matches, grants, or execution world. A material permission
increase requires renewed approval.

## Runtime adapters

The shared package parses, validates, normalizes, compares permissions, and
produces a browser-neutral registration plan. It does not call WebExtension
APIs itself.

The Jangolova extension supplies the browser manager/backend. Its preferred
runtime adapter is the MV3
`userScripts` API, which is designed for user-provided arbitrary code. The
extension advertises userscript execution only after probing the API with a
real method call. Browser-level user settings may make the API unavailable
even when the manifest permission exists.

There is deliberately no `eval` or `Function` fallback in an extension
service worker/content script. A browser without a verified arbitrary-code
userscript execution environment may still import a disabled script, inspect
it, store it, or remove it, but reports runtime state `unavailable`; enabling
or installing an enabled script fails safely and no source is executed.

Chrome requires the `userScripts` permission and a user-controlled setting.
Firefox MV3 also provides a dedicated `userScripts` API. Safari support is
capability-probed in the embedded WebExtension; Jangolova does not infer it
from generic `scripting` support.

## Storage and lifecycle

The canonical extension-side record is stored under a Jangolova-owned
namespace in `browser.storage.local`. Source is size-bounded and never copied
into events or descriptions. Descriptions return source metadata and hashes,
not source code, unless an authenticated caller explicitly requests a source
export capability that is not part of the MVP.

On extension startup/update, the manager reconciles enabled stored records with
the browser's registered userscripts. It removes orphan registrations and
restores missing approved registrations. Browser registration IDs are derived
from stable script IDs and never supplied directly by page code.

Events include install/update/enable/disable/uninstall and runtime availability
changes. Event payloads omit source and credentials.

## Ownership boundary

Cymonkey owns the programmatic contract: install, update, uninstall,
enable/disable, list, describe, and lifecycle events. The Jangolova extension
manager implements browser-native approval, persistence, registration,
reconciliation, and the Safari native metadata bridge. Cymonkey never exposes
that manager's raw `chrome.*`, `browser.*`, or Safari APIs.
