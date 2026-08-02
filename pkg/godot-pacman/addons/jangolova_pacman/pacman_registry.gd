class_name PacmanRegistry
extends Node

signal event_published(event: Dictionary)

@export var registrations: Array[Dictionary] = []

var _allowlist: Dictionary = {}
var _events: Array[Dictionary] = []
var _revision := 1
var _event_sequence := 0

func _ready() -> void:
	_build_allowlist()

func dispatch(method: String, params: Dictionary) -> Dictionary:
	match method:
		PacmanProtocol.METHOD_HELLO:
			return _hello()
		PacmanProtocol.METHOD_CAPABILITIES:
			return _capabilities()
		PacmanProtocol.METHOD_DESCRIBE:
			return _describe()
		PacmanProtocol.METHOD_ACT:
			return _act(params)
		PacmanProtocol.METHOD_EVENTS:
			return _events(params)
		PacmanProtocol.METHOD_HEALTH:
			return _health()
		_:
			return _error("method_not_found", "Unsupported Pacman method.")

func _build_allowlist() -> void:
	_allowlist.clear()
	for registration in registrations:
		if not registration is Dictionary:
			push_error("Pacman registrations must be dictionaries.")
			continue
		var stable_id: String = str(registration.get("id", ""))
		var kind: String = str(registration.get("kind", ""))
		var target: Node = registration.get("target") as Node
		if target == null or kind not in PacmanProtocol.RESOURCE_KINDS or not PacmanProtocol.is_stable_id(stable_id, kind):
			push_error("Pacman registration requires a target and stable kind-prefixed ID.")
			continue
		if _allowlist.has(stable_id):
			push_error("Duplicate Pacman resource ID: %s" % stable_id)
			continue
		_allowlist[stable_id] = registration

func _hello() -> Dictionary:
	return {
		"protocolVersion": PacmanProtocol.VERSION,
		"implementation": {"engine": "godot", "name": "jangolova-godot-pacman", "version": "0.1.0"},
		"features": ["events.cursor", "resources.explicit-allowlist"]
	}

func _capabilities() -> Array:
	return [
		{"name": "resource.describe", "effect": "read", "targetKinds": PacmanProtocol.RESOURCE_KINDS,
			"inputSchema": {"type": "object", "additionalProperties": false}},
		{"name": "object.visible.set", "effect": "write", "targetKinds": ["object", "ui", "camera"],
			"inputSchema": {"type": "object", "properties": {"visible": {"type": "boolean"}}, "required": ["visible"], "additionalProperties": false}}
	]

func _describe() -> Dictionary:
	var resources: Array = []
	var ids := _allowlist.keys()
	ids.sort()
	for stable_id in ids:
		resources.append(_describe_resource(_allowlist[stable_id]))
	return {"revision": str(_revision), "resources": resources}

func _act(params: Dictionary) -> Dictionary:
	var name := str(params.get("name", ""))
	var target_id := str(params.get("targetId", ""))
	if not _allowlist.has(target_id):
		return _error("target_not_allowlisted", "Pacman target is not allowlisted.")
	var registration: Dictionary = _allowlist[target_id]
	var actions: Array = registration.get("actions", [])
	if name not in actions:
		return _error("action_not_allowlisted", "Pacman action is not allowlisted for this target.")
	if name == "resource.describe":
		return _describe_resource(registration)
	if name == "object.visible.set":
		var input: Dictionary = params.get("input", {})
		if not input.has("visible") or not (input["visible"] is bool):
			return _error("invalid_input", "visible is required.")
		var target: Node = registration["target"] as Node
		target.visible = input["visible"]
		_revision += 1
		_publish("event:resource-changed", target_id, {"visible": target.visible})
		return {"ok": true, "revision": str(_revision)}
	return _error("action_not_implemented", "Allowlisted action has no Godot handler.")

func _events(query: Dictionary) -> Dictionary:
	var after := int(str(query.get("after", "0")))
	var limit := clampi(int(query.get("limit", 100)), 1, 1000)
	var selected: Array = []
	var cursor := str(after)
	for event in _events:
		if int(str(event["id"])) <= after:
			continue
		selected.append(event.duplicate(true))
		cursor = str(event["id"])
		if selected.size() >= limit:
			break
	return {"events": selected, "cursor": cursor}

func _health() -> Dictionary:
	return {"status": "ready", "observedAt": Time.get_datetime_string_from_system(true)}

func _describe_resource(registration: Dictionary) -> Dictionary:
	var target: Node = registration["target"] as Node
	return {"id": registration["id"], "kind": registration["kind"], "label": registration.get("label", ""),
		"properties": {"visible": target.visible}}

func _publish(kind: String, source_id: String, data: Dictionary) -> void:
	_event_sequence += 1
	var event := {"id": str(_event_sequence), "type": kind, "sourceId": source_id,
		"occurredAt": Time.get_datetime_string_from_system(true), "data": data}
	_events.append(event)
	if _events.size() > 256:
		_events.pop_front()
	event_published.emit(event)

func _error(code: String, message: String) -> Dictionary:
	return {"__pacman_error": {"code": code, "message": message}}
