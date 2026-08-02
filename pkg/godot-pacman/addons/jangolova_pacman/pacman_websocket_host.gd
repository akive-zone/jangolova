class_name PacmanWebSocketHost
extends Node

@export var listen_host := "0.0.0.0"
@export var listen_port := 8090
@export var bearer_token := ""
@export var registry_path: NodePath

var _server := TCPServer.new()
var _peer: WebSocketPeer
var _registry: PacmanRegistry

func _ready() -> void:
	_registry = get_node_or_null(registry_path) as PacmanRegistry
	if _registry == null or bearer_token.is_empty():
		push_error("Pacman WebSocket host requires a registry and bearer token.")
		return
	var error := _server.listen(listen_port, listen_host)
	if error != OK:
		push_error("Pacman WebSocket listener failed: %s" % error)

func _process(_delta: float) -> void:
	if _server.is_connection_available():
		var stream := _server.take_connection()
		var candidate := WebSocketPeer.new()
		candidate.accept_stream(stream)
		_peer = candidate
	if _peer == null:
		return
	_peer.poll()
	if _peer.get_ready_state() == WebSocketPeer.STATE_OPEN:
		while _peer.get_available_packet_count() > 0:
			var packet := _peer.get_packet()
			if packet.size() > PacmanProtocol.MAXIMUM_MESSAGE_BYTES:
				_peer.close(1009, "Pacman message is too large")
				return
			_handle_message(packet.get_string_from_utf8())
	elif _peer.get_ready_state() == WebSocketPeer.STATE_CLOSED:
		_peer = null

func _handle_message(text: String) -> void:
	var request = JSON.parse_string(text)
	if not request is Dictionary:
		_send_error(0, "invalid_json", "Pacman request must be an object.")
		return
	var request_id = request.get("id", 0)
	var method := str(request.get("method", ""))
	var params: Dictionary = request.get("params", {})
	if method.is_empty() or not params is Dictionary:
		_send_error(request_id, "invalid_request", "Pacman request requires method and object params.")
		return
	var result: Dictionary = _registry.dispatch(method, params)
	if result.has("__pacman_error"):
		var error: Dictionary = result["__pacman_error"]
		_send_error(request_id, str(error.get("code", "pacman_error")), str(error.get("message", "Pacman error.")))
		return
	_send({"id": request_id, "result": result})

func _send_error(request_id, code: String, message: String) -> void:
	_send({"id": request_id, "error": {"code": code, "message": message}})

func _send(value: Dictionary) -> void:
	if _peer != null and _peer.get_ready_state() == WebSocketPeer.STATE_OPEN:
		_peer.send_text(JSON.stringify(value))

func _exit_tree() -> void:
	if _peer != null:
		_peer.close(1000, "Pacman detached")
	_server.stop()
