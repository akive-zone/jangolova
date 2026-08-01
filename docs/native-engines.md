# Native Engines

## Native-process adapter

The `native-process` adapter launches an executable while preserving the
engine's native semantics. It is the lifecycle foundation for Unity, Unreal,
creative tools, and custom applications.

```bash
go run ./cmd/jangolova launch-engine \
  --adapter native-process \
  --source ./path/to/application \
  --env DISPLAY=:99 \
  --handle native.window=caller-owned-window-1234 \
  --options '{
    "args":["--project","./project"],
    "workingDir":"./workspace",
    "environment":{"APPLICATION_MODE":"presentation"},
    "inheritEnvironment":true,
    "startupGrace":"500ms",
    "stopTimeout":"5s"
  }'
```

Environment values are combined in this order:

1. inherited host environment, unless disabled;
2. caller-supplied runtime environment;
3. native-process adapter options;
4. Jangolova engine metadata and optional bridge credentials.

Opaque handles remain separate from environment values and remain owned by the
caller. An engine-specific adapter or plugin may interpret a handle it knows.

`startupGrace` detects immediate failure; it is not semantic application
readiness. The process emits lifecycle events after launch and on unexpected
exit. Stop sends an interrupt and escalates to a forced kill after
`stopTimeout`.

Run the included fixture:

```bash
go run ./cmd/jangolova launch-engine \
  --adapter native-process \
  --source examples/native-process/fixture.sh
```

## Cooperative native bridge

When `bridge.enabled` is true, the adapter creates an authenticated loopback
WebSocket host before starting the engine and injects:

- `JANGOLOVA_BRIDGE_URL`
- `JANGOLOVA_BRIDGE_TOKEN`
- `JANGOLOVA_BRIDGE_PROTOCOL`

The engine plugin connects outward. Browser-origin requests, invalid bearer
tokens, and second connections are rejected. The bridge implements `hello`,
`capabilities`, `describe`, `act`, and `events`.

`internal/bridge.ValidateConformance` verifies protocol identity, capability
schemas, semantic description, a selected safe action, and cursor-addressed
events. These are engine-integration checks; caller authorization remains
outside Jangolova.

## Unity package

The Unity package lives at `integrations/unity/com.jangolova.bridge`. It reads
the injected bridge environment, connects with `ClientWebSocket`, and
dispatches requests on Unity's main thread.

For a macOS player, launch the executable inside the application bundle:

```bash
go run ./cmd/jangolova launch-engine \
  --adapter native-process \
  --source ./Builds/macOS/JangolovaDemo.app/Contents/MacOS/JangolovaDemo \
  --options '{"bridge":{"enabled":true,"address":"127.0.0.1:0"}}'
```

Windows and Linux use the same lifecycle contract with their native executable
paths and caller-supplied runtime environment.
