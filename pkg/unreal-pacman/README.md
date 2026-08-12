# Jangolova Pacman for Unreal

Jangolova Pacman is distributed as an Unreal plugin and as a separate packaged
headless fixture image. The plugin is engine code that you add to your own
project; the container is a ready-to-run conformance environment.

## Install the plugin

Download the Unreal 5.8 Linux artifact from the [v0.1.0 GitHub
Release](https://github.com/akive-zone/jangolova/releases/tag/v0.1.0):

```sh
curl -L -o JangolovaPacman.tar.gz \
  https://github.com/akive-zone/jangolova/releases/download/v0.1.0/JangolovaPacman-0.1.0-UE5.8-Linux.tar.gz
mkdir -p MyGame/Plugins
tar -xzf JangolovaPacman.tar.gz -C MyGame/Plugins
```

The extracted directory must be named `JangolovaPacman` and contain
`JangolovaPacman.uplugin`. Enable the plugin in the Unreal project, regenerate
project files, and add `JangolovaPacman` to the module dependencies of any C++
module that includes its headers:

```csharp
PublicDependencyModuleNames.Add("JangolovaPacman");
```

The release artifact is a Linux/UE 5.8 build. Projects targeting another
engine version or platform should build the source plugin with that engine's
`RunUAT.sh BuildPlugin` command instead.

`JangolovaPacman` is the Unreal Engine implementation of
`jangolova.pacman/v1alpha1`. It exposes semantic control of explicitly
registered Unreal objects while Unreal continues to own rendering and
application lifecycle.

Add a `UPacmanRegistryComponent` to an Actor and populate `Registrations` with
stable kind-prefixed IDs, explicit UObject targets, and per-resource action
allowlists. The component never scans the World, Actor registry, UObject heap,
or UMG tree.

For example, register an actor-owned object in C++ (the same fields are
editable in the Details panel or Blueprint):

```cpp
UPacmanRegistryComponent* Registry = NewObject<UPacmanRegistryComponent>(Actor);
Registry->RegisterComponent();

FPacmanRegistration Registration;
Registration.StableId = TEXT("object:fixture");
Registration.Kind = EPacmanResourceKind::Object;
Registration.Label = TEXT("Fixture");
Registration.Target = Actor;
Registration.Actions = { TEXT("resource.describe"), TEXT("object.visibility.set") };
Registry->Registrations.Add(Registration);
```

Stable IDs and action names are part of the caller-owned allowlist. Pacman does
not discover arbitrary Unreal objects, actors, widgets, or assets.

The initial semantic handlers provide `resource.describe` and
`object.visibility.set`. `FPacmanWebSocketHost` authenticates an already
upgraded caller-owned WebSocket with a bearer token, enforces the Pacman message
limit, wraps requests and responses, and invokes the registry on the Unreal
game thread. A platform binding supplies the listen/upgrade implementation via
`IPacmanWebSocketConnection`. Stopping a transport must only detach Pacman; it
must never quit the game, destroy the World, or terminate the host process.

The package includes a UE 5.8 `WebSocketServer` listen/upgrade adapter. It
accepts one authenticated connection on the configured port and forwards text
frames into the Pacman host. The fixture uses it when
`JANGOLOVA_PACMAN_TOKEN` is present.

## Transport integration

`FPacmanWebSocketHost` is the protocol/authentication boundary. For a packaged
UE 5.8 application, `FPacmanWebSocketServer` provides the built-in HTTP Upgrade
listener through the engine `WebSocketServer` module:

```cpp
FPacmanWebSocketServer Server(Token);
Server.Start(8090, Registry);
```

Applications with another networking stack can instead perform the upgrade
themselves, call `StartHost`, and pass an `IPacmanWebSocketConnection` to
`AcceptConnection`. When the platform server cannot expose request headers,
the adapter accepts the first frame as `{ "type": "auth", "token": "..." }`.

Pass the expected bearer token when constructing the host. Requests must send
the matching header:

```text
Authorization: Bearer <runtime-token>
```

Keep the token in runtime secret configuration; do not place it in the project,
plugin, Dockerfile, or source control.
