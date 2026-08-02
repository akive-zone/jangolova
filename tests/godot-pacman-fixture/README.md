# Godot Pacman headless fixture

This Godot 4 project is the license-free reference fixture for
`pkg/godot-pacman`. The container copies the package into the project, launches
the scene with `--headless`, and exposes the explicitly registered
`object:fixture` resource through an authenticated `pacman-ws` listener.

The fixture is intentionally source-only. It can run in CI without a display,
GPU, proprietary editor license, or engine installation on the developer's
machine.
