# Web presentation provider handoff

`web-presentation` is the first provider-visible presentation adapter in
Jangolova. It lets an agent create and update an interface without making
Jangolova responsible for the browser or display process.

## Ownership

Jangolova owns the declarative document model, semantic actions, presentation
events, and the adapter worker. Xallet or a native host owns the static asset
server, Chromium process, CDP endpoint, display/window placement, GPU, and
cleanup. Disconnecting the Jangolova instance must leave that target running.

## Target contract

The target provider starts a Chromium-compatible browser and supplies a CDP
endpoint. The provider may host the reference page in
`examples/web-presentation` or provide another page implementing the same
`window.jangolova` bridge:

```json
{
  "engine": {
    "adapter": "web-presentation",
    "source": "https://presentation.example/session.html",
    "options": {
      "policy": {
        "maxHTMLBytes": 1048576,
        "maxCSSBytes": 262144,
        "maxJavaScriptBytes": 262144,
        "maxTotalBytes": 1572864,
        "allowedSourceOrigins": ["https://presentation.example"],
        "allowedAssetOrigins": ["self", "https://assets.example"],
        "authorizedActions": ["presentation.capture", "presentation.execute"],
        "executeTimeoutMillis": 5000,
        "captureTimeoutMillis": 10000
      }
    }
  },
  "target": {
    "kind": "browser",
    "endpoints": [{
      "name": "cdp",
      "protocol": "cdp",
      "url": "http://127.0.0.1:9222"
    }]
  }
}
```

`source` is optional when the target provider has already opened a compatible
page. Jangolova attaches with Puppeteer Core and never calls a browser launch
or shutdown API.

## Artifact policy

The current artifact model supports inline `{html, css, js}` and structured
presentation documents. A caller may use the independently hosted `source`
page as a versioned bundle host, but Jangolova does not yet ingest a separate
bundle-manifest format.

Every `web-presentation` connection has an enforced policy. Defaults are:

- HTML: 1 MiB;
- CSS: 256 KiB;
- authored or executed JavaScript: 256 KiB;
- total inline source or structured document: 1.5 MiB;
- source page: unrestricted unless `allowedSourceOrigins` is configured;
- loaded assets: `self`, `data:`, and `blob:`;
- sensitive actions authorized by policy: `presentation.capture` and
  `presentation.execute`;
- `presentation.execute` timeout: five seconds;
- `presentation.capture` timeout: ten seconds.

Limits count UTF-8 bytes. Each configured limit must be between one byte and
32 MiB. Origins must be exact HTTP(S) origins without paths, queries,
fragments, or credentials. When `allowedSourceOrigins` is present, the
configured `source` must match it. `allowedAssetOrigins` accepts exact HTTP(S)
origins plus `self`, `data:`, and `blob:`. Supplying an asset list replaces the
default list.

`authorizedActions` is the provider authorization gate for sensitive
presentation operations. It currently accepts only `presentation.execute` and
`presentation.capture`; supplying an empty list denies both. These actions also
emit provider instance audit events named
`presentation.execute.requested`, `.succeeded`, `.failed`, `.cancelled`, or
`.denied`, and the equivalent `presentation.capture.*` names. Audit events do
not include script source, screenshots, or result payloads.

`executeTimeoutMillis` and `captureTimeoutMillis` must be between 1 and
120000. The worker enforces these deadlines inside the CDP attachment. On an
execute timeout it sends Chrome `Runtime.terminateExecution` so a stuck page
script is interrupted; the adapter also keeps a slightly longer cancellation
deadline around the worker request.

The worker enforces asset origins through Chromium request interception. This
is defense in depth for the authored surface, not a replacement for Xallet's
container or host network policy.

## Semantic operations

Call the normal bridge `act` method with one of these names:

- `presentation.create` — initialize a document.
- `presentation.replace` — replace the complete document.
- `presentation.write` — write an HTML/CSS/JavaScript presentation artifact.
- `presentation.execute` — run presentation JavaScript against the mounted
  surface when an incremental behavior update is needed.
- `presentation.patch` — apply `set`, `remove`, or `append` operations.
- `presentation.describe` — return the document and viewport.
- `presentation.activate` — activate a semantic button by id.
- `presentation.capture` — return a PNG captured by the attached browser.

Use the bridge `events` method with a cursor to consume activation, update, and
resize events. A richer Three.js, Unity, or Unreal host can implement the same
bridge and expose engine-specific capabilities alongside these operations.

For example, an agent can write a complete surface with:

```json
{
  "name": "presentation.write",
  "input": {
    "expectedRevision": "0",
    "html": "<article id=\"card\"><h1>Hello</h1><button id=\"next\">Next</button></article>",
    "css": "#card { padding: 32px; }",
    "js": "root.querySelector('#next').onclick = () => emit('next.clicked', {});"
  }
}
```

`presentation.create`, `presentation.replace`, `presentation.write`, and
`presentation.patch` require `expectedRevision`. A new page begins at revision
`"0"`; every successful mutation returns and emits the next revision.
`presentation.describe` returns the current revision. A stale mutation fails
with a revision conflict and leaves the current document unchanged.

`presentation.create` is valid only at the empty revision. Use
`presentation.replace` or `presentation.patch` for subsequent updates.

The JavaScript runs inside the caller-owned browser page and is therefore an
explicit externally-effectful capability. The calling agent system remains
responsible for deciding which sessions receive `presentation.execute`
authorization. Jangolova enforces the provider-supplied authorization policy,
records audit events, and bounds execution time.

## Xallet implementation notes

Xallet should model the CDP endpoint and presentation URL as target resources,
translate container-to-host addresses before creating the Jangolova instance,
and keep the endpoint private to the session network. It can later replace the
reference DOM renderer with a Three.js/Unity/Unreal presentation host without
changing the Jangolova provider contract.
