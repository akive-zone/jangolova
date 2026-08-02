extends Node

func _ready() -> void:
	var registry: PacmanRegistry = $PacmanRegistry
	var host: PacmanWebSocketHost = $PacmanWebSocketHost
	registry.registrations = [{
		"id": "object:fixture",
		"kind": "object",
		"label": "Godot headless fixture target",
		"target": $FixtureTarget,
		"actions": ["resource.describe", "object.visible.set"]
	}]
	registry._build_allowlist()
	host.registry_path = registry.get_path()
