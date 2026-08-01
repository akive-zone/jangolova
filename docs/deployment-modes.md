# Deployment Modes

Jangolova owns display-engine lifecycle. The caller owns placement and the
display environment. The same engine adapters and endpoint contract are used
in every deployment mode.

## Standalone native host

Discover registered engine adapters, availability, and capabilities:

```sh
jangolova engines
jangolova engines --json
```

Launch one engine directly:

```sh
jangolova launch-engine \
  --adapter chromium \
  --source https://example.com \
  --options '{"headless":false}'
```

The engine inherits the current process environment. On macOS and Windows,
Chromium uses the native host display by default. On Linux, an existing
`DISPLAY` or `WAYLAND_DISPLAY` selects headed operation; otherwise Chromium
defaults to headless operation. Engine options can always override that choice.

The command writes an initial instance description followed by lifecycle
events as JSONL. It owns the engine until interrupted or until the engine exits
on its own.

## Standalone external display

Jangolova can use a display created by any external system:

```sh
jangolova launch-engine \
  --adapter chromium \
  --env DISPLAY=:99 \
  --handle native.window=caller-owned-window-1234 \
  --source https://example.com
```

Jangolova does not start or stop the referenced X server. The same rule applies
to Wayland variables, native handles, externally managed browser profiles, and
container networking.

Handles are opaque adapter inputs. Supplying one never transfers ownership of
the referenced window, view, layer, device, or runtime object to Jangolova.

## Standalone provider

The authenticated provider API is usable without Xallet:

```sh
export JANGOLOVA_PROVIDER_TOKEN="replace-with-a-random-secret"
jangolova serve-engine-provider --bind 127.0.0.1:7391
```

Any authorized client can discover and launch engines through the versioned
provider protocol. Loopback is the default bind and should remain the default
outside a protected private network.

## Independent container

An operator may package or run Jangolova in any OCI environment. The operator
supplies display variables, mounts, profiles, ports, and networking:

```sh
docker run --rm \
  -e JANGOLOVA_PROVIDER_TOKEN \
  -e DISPLAY=surface:99 \
  -p 127.0.0.1:7391:7391 \
  -p 127.0.0.1:9222:9222 \
  jangolova-engine-runtime \
  serve-engine-provider --bind 0.0.0.0:7391
```

The optional engine-runtime image is a binary/engine artifact. It does not
create Xvfb or publish VNC.

## Xallet-managed

Xallet decides whether Jangolova runs on the host or through Docker, Podman, or
Apple Container. Xallet creates the surface, network, volumes, secrets, and
port mappings, then starts Jangolova and supplies the resolved environment.

```text
Xallet surface/placement configuration
              |
              v
       DISPLAY / native handle
              |
              v
       Jangolova engine provider
              |
              v
        engine endpoint metadata
              |
              v
    Xallet controllers and gateways
```

Jangolova does not import Xallet packages and remains fully usable when Xallet
is absent.

## Ownership invariant

Jangolova consumes a display environment; it does not create or publish the
display environment. Test fixtures may create temporary displays or containers
to verify portability, but they are not product deployment configuration.
