# Dynamic Presentation Integrations

Jangolova lets browser and native engines expose cooperative, engine-specific
presentation capabilities. The engine owns its scene model. Jangolova owns the
engine-side protocol and lifecycle; Xallet or another caller owns control,
authorization, input, capture, and display publication.

```text
Xallet or another authorized caller
              |
              | caller-owned control transport
              v
      cooperative engine integration
      (web / Unity / Unreal plugin)
              |
              v
       engine scene and renderer
```

## Experience bridge

Cooperative integrations implement `jangolova.bridge/v1alpha1`:

- `hello` negotiates protocol version and implementation identity;
- `capabilities` describes engine-specific bounded operations;
- `describe` returns serializable engine state;
- `act` performs one advertised operation;
- `events` reads bounded cursor-addressed engine events.

Capabilities and effects are descriptive protocol data. They never grant
authorization. The caller applies policy before invoking an operation.

## Browser experiences

The `web-project` engine serves a local project on loopback and opens it in
Chromium. A cooperative page can expose `window.jangolova` with the five bridge
operations. The included Three.js example implements object creation, updates,
animation, camera changes, scene description, and selection events.

Install the Three.js dependency and launch the example:

```bash
npm install
go run ./cmd/jangolova launch-engine \
  --adapter web-project \
  --source examples/threejs-scene \
  --options '{
    "mounts":[{
      "urlPath":"/vendor/three.module.js",
      "source":"./node_modules/three/build/three.module.js"
    }],
    "chromium":{"headless":false}
  }'
```

Jangolova reports the private CDP endpoint for the browser engine. It does not
attach a CDP client or expose the display. Xallet or another caller may consume
that endpoint and the page bridge under its own policy.

## Unity and Unreal

`integrations/unity/com.jangolova.bridge` is the first native integration. It
connects outward to the authenticated loopback bridge supplied to a
`native-process` engine and dispatches bridge calls on Unity's main thread.

Unreal should implement the same engine-side wire contract without copying
Unity-specific scene semantics. Shared conventions should emerge only when
multiple engines demonstrate genuinely common behavior.

## Trust boundary

- Engine content declares capabilities; callers authorize them.
- Bridge credentials are process-scoped and never appear in endpoint URLs.
- Browser input, desktop input, screenshots, recordings, and remote display
  gateways remain outside Jangolova.
- Engine plugins may expose richer operations without expanding the common
  lifecycle contract.
