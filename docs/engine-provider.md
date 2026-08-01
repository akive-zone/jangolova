# Display Engine Provider

Jangolova exposes display engines through the authenticated
`jangolova.engine/v1alpha1` provider API. Xallet is the primary orchestrator,
but the protocol is independently usable by any authorized client.

Start the provider:

```bash
export JANGOLOVA_PROVIDER_TOKEN="a-random-session-secret"
jangolova serve-engine-provider --bind 127.0.0.1:7391
```

The command can be started directly on a host, inside an independently managed
container, or by Xallet. The health endpoint is unauthenticated; all engine
inventory and lifecycle operations require `Authorization: Bearer`.

## Operations

- `GET /healthz`
- `GET /v1/engines`
- `POST /v1/instances`
- `GET /v1/instances/{instanceId}`
- `GET /v1/instances/{instanceId}/events`
- `DELETE /v1/instances/{instanceId}`

A launch request contains only engine-owned configuration and a
placement-scoped runtime supplied by its caller. Environment entries are
inherited by child processes. Handles are opaque named values interpreted only
by an adapter that understands them:

```json
{
  "apiVersion": "jangolova.engine/v1alpha1",
  "instanceId": "browser-one",
  "engine": {
    "adapter": "chromium",
    "source": "about:blank",
    "options": {
      "address": "http://127.0.0.1:9222"
    }
  },
  "environment": {
    "DISPLAY": ":99"
  },
  "handles": {
    "native.window": "caller-owned-window-1234"
  }
}
```

Passing a handle does not transfer ownership. Jangolova must not create,
publish, close, or destroy the resource identified by it. Handle names are
stable protocol identifiers; handle values are placement-specific and should
not be logged.

The response reports typed engine endpoints. `targetPort` lets Xallet resolve
an engine endpoint through its managed container port mapping instead of
assuming the provider and display runtime share a network namespace:

```json
{
  "apiVersion": "jangolova.engine/v1alpha1",
  "instanceId": "browser-one",
  "adapter": "chromium",
  "status": "running",
  "health": {
    "status": "healthy",
    "observedAt": "2026-08-01T12:00:00Z"
  },
  "endpoints": [
    {
      "name": "cdp",
      "protocol": "cdp",
      "url": "http://127.0.0.1:9222",
      "targetPort": 9222,
      "visibility": "private"
    }
  ]
}
```

`GET /v1/engines` reports whether each adapter is currently usable and lists
only capabilities supported in the current environment. For example, Chromium
always advertises `attach`, while `launch` is advertised only when a Chromium
executable is discoverable. `GET /v1/instances/{instanceId}` performs an active
adapter health probe and records a lifecycle event when health changes.

## Lifecycle events

Each instance retains a bounded, cursor-addressed lifecycle history. Read all
currently retained events, or continue after a previous cursor:

```http
GET /v1/instances/browser-one/events?after=2&limit=100
Authorization: Bearer ...
```

```json
{
  "apiVersion": "jangolova.engine/v1alpha1",
  "instanceId": "browser-one",
  "events": [
    {
      "cursor": "3",
      "type": "engine.failed",
      "status": "failed",
      "message": "process exited",
      "occurredAt": "2026-08-01T12:00:00Z"
    }
  ],
  "cursor": "3"
}
```

The provider itself records `instance.starting`, `instance.ready`, and
`instance.stopping`. Process-owning adapters report `engine.exited` or
`engine.failed`, which also updates the instance status. Event history is
bounded to 256 entries; an expired cursor returns `410 Gone`.

The provider owns engine-local cleanup. Xallet owns the provider workload,
surface lifecycle, endpoint publication, controllers, grants, and display
session state.
