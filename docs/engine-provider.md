# Interaction Engine Provider

Jangolova exposes interaction engines through the authenticated
`jangolova.interaction/v1alpha1` API. Xallet is one target provider, not a
Jangolova runtime dependency.

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
  "apiVersion": "jangolova.interaction/v1alpha1",
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

The response describes the interaction instance, not the target:

```json
{
  "apiVersion": "jangolova.interaction/v1alpha1",
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
