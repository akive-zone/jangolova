# Jangolova Pacman for Unreal

`JangolovaPacman` is the Unreal Engine implementation of
`jangolova.pacman/v1alpha1`. It exposes semantic control of explicitly
registered Unreal objects while Unreal continues to own rendering and
application lifecycle.

Add a `UPacmanRegistryComponent` to an Actor and populate `Registrations` with
stable kind-prefixed IDs, explicit UObject targets, and per-resource action
allowlists. The component never scans the World, Actor registry, UObject heap,
or UMG tree.

The initial semantic handlers provide `resource.describe` and
`object.visibility.set`. `FPacmanWebSocketHost` authenticates an already
upgraded caller-owned WebSocket with a bearer token, enforces the Pacman message
limit, wraps requests and responses, and invokes the registry on the Unreal
game thread. A platform binding supplies the listen/upgrade implementation via
`IPacmanWebSocketConnection`. Stopping a transport must only detach Pacman; it
must never quit the game, destroy the World, or terminate the host process.

This first package slice establishes the engine-neutral registry, dispatcher,
and authenticated WebSocket connection host. A platform-specific listen/upgrade
binding and live packaged-game fixture are the next Unreal milestones.
