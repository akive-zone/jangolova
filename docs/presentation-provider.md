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
    "source": "http://presentation-host/session.html"
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
    "html": "<article id=\"card\"><h1>Hello</h1><button id=\"next\">Next</button></article>",
    "css": "#card { padding: 32px; }",
    "js": "root.querySelector('#next').onclick = () => emit('next.clicked', {});"
  }
}
```

The JavaScript runs inside the caller-owned browser page and is therefore an
explicit externally-effectful capability. The calling agent system remains
responsible for authorization and content policy.

## Xallet implementation notes

Xallet should model the CDP endpoint and presentation URL as target resources,
translate container-to-host addresses before creating the Jangolova instance,
and keep the endpoint private to the session network. It can later replace the
reference DOM renderer with a Three.js/Unity/Unreal presentation host without
changing the Jangolova provider contract.
