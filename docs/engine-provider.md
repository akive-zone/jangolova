# Interaction Engine Provider

Jangolova implements the authenticated, provider-neutral
`interaction.engine/v1alpha1` API. Xallet is one target provider, not a
Jangolova runtime dependency, and other providers can implement the same
contract.

```bash
export JANGOLOVA_PROVIDER_TOKEN="a-random-session-secret"
jangolova serve-engine-provider --bind 127.0.0.1:7391
```

## Operations

- `GET /healthz`
- `GET /v1/engines`
- `POST /v1/instances`
- `GET /v1/instances/{instanceId}`
- `POST /v1/instances/{instanceId}/call`
- `GET /v1/instances/{instanceId}/events`
- `DELETE /v1/instances/{instanceId}`

Connect Playwright to Chromium that the caller already owns:

```json
{
  "apiVersion": "interaction.engine/v1alpha1",
  "instanceId": "browser-one",
  "engine": {
    "adapter": "playwright"
  },
  "target": {
    "kind": "browser",
    "endpoints": [
      {
        "name": "cdp",
        "protocol": "cdp",
        "url": "http://127.0.0.1:9222"
      }
    ]
  }
}
```

Puppeteer can instead attach to a caller-owned WebDriver BiDi endpoint:

```json
{
  "apiVersion": "interaction.engine/v1alpha1",
  "instanceId": "firefox-one",
  "engine": {"adapter": "puppeteer"},
  "target": {
    "kind": "browser",
    "endpoints": [{
      "name": "webdriver-bidi",
      "protocol": "webdriver-bidi",
      "url": "ws://127.0.0.1:9223/session"
    }]
  }
}
```

Attach Jangolova's declarative web presentation bridge to a caller-owned
Chromium page. The target provider is responsible for starting Chromium,
serving the presentation URL, and supplying the CDP endpoint:

```json
{
  "apiVersion": "interaction.engine/v1alpha1",
  "instanceId": "presentation-one",
  "engine": {
    "adapter": "web-presentation",
    "source": "http://127.0.0.1:8080/examples/web-presentation/"
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

The presentation adapter does not launch Chromium, allocate a display, or
serve files. Those responsibilities remain with Xallet or a native host.
Once connected, call `act` with `presentation.create` or
`presentation.replace` for structured documents, or `presentation.write` with
HTML/CSS/JavaScript source for a complete authored surface. Then use
`presentation.patch`, `presentation.execute`, and `presentation.activate` for
incremental updates. `presentation.capture` returns a PNG captured by the
attached browser.

WebDriver Classic attaches to an existing caller-owned driver session. The
target provider creates and later deletes that session:

```json
{
  "apiVersion": "interaction.engine/v1alpha1",
  "instanceId": "safari-one",
  "engine": {"adapter": "webdriver-classic"},
  "target": {
    "kind": "browser",
    "endpoints": [{
      "name": "webdriver",
      "protocol": "webdriver",
      "url": "http://127.0.0.1:4444"
    }],
    "handles": {"webdriver.sessionId": "caller-created-session"}
  }
}
```

Use `webkit-webdriver` with the same target shape for WebKitGTK, WPE WebKit,
or a WebKit-specific Safari attachment. For Safari MCP, select `safari-mcp` and
provide an endpoint with protocol `mcp-streamable-http`; Jangolova discovers
the relay's MCP tools and exposes their input schemas as interaction
capabilities.

The response describes the interaction instance, not the target:

```json
{
  "apiVersion": "interaction.engine/v1alpha1",
  "instanceId": "browser-one",
  "adapter": "playwright",
  "status": "connected",
  "health": {
    "status": "connected",
    "observedAt": "2026-08-01T12:00:00Z"
  },
  "capabilities": ["browser.click", "browser.evaluate", "browser.fill"]
}
```

Call the common semantic protocol:

```http
POST /v1/instances/browser-one/call
Authorization: Bearer ...
Content-Type: application/json

{
  "method": "act",
  "params": {
    "name": "browser.fill",
    "input": {"selector": "#email", "value": "agent@example.com"}
  }
}
```

Supported browser actions currently include navigation, click, fill, press,
JavaScript evaluation, and screenshots. Capability metadata reports effect and
input schema; authorization remains the caller's responsibility.

Deleting an instance disconnects Jangolova. It must not terminate the browser
or native target. Lifecycle events are bounded to 256 entries and addressed by
cursor through the `/events` operation.

See [Browser target protocols and Xallet handoff](browser-target-protocols.md)
for protocol selection, address translation, security, and cleanup ownership.
