# Jangolova Pacman for Godot

`godot-pacman` is the license-free reference runtime for
`jangolova.pacman/v1alpha1`. It is a small Godot 4 GDScript package that keeps
semantic registration separate from rendering and application lifecycle.

Add `PacmanRegistry` to a scene and populate `registrations` explicitly. Each
registration has a stable kind-prefixed ID, a Node target, and an action
allowlist. Unregistered Nodes are invisible to Pacman; the package never walks
the SceneTree to export the project automatically.

`PacmanWebSocketHost` owns an authenticated caller-facing `pacman-ws` listener.
It accepts one connection, enforces a message-size limit, and dispatches all
semantic work on Godot's main thread. Closing the connection never quits Godot
or frees the target scene.

The package is dependency-free and can run in a headless Godot 4 container. The
fixture at `tests/godot-pacman-fixture` exercises the protocol without requiring
a display or a proprietary engine license.
