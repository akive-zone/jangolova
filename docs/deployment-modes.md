# Deployment Modes

Jangolova's code is the same in every mode: it receives an already-running
target and attaches an interaction engine.

## Native host

Start Chromium using the native system's preferred mechanism and enable a
private CDP endpoint. Then attach Jangolova:

```sh
jangolova connect-engine \
  --adapter playwright \
  --target-kind browser \
  --endpoint cdp=http://127.0.0.1:9222
```

Puppeteer uses the same command with `--adapter puppeteer`. Interrupting the
command disconnects the adapter and leaves Chromium running.

## Standalone provider

```sh
export JANGOLOVA_PROVIDER_TOKEN="replace-with-a-random-secret"
jangolova serve-engine-provider --bind 127.0.0.1:7391
```

Any authorized caller can submit target endpoints and use the semantic API.

## Independent container

The operator places Jangolova where it can reach the target endpoint:

```sh
docker run --rm \
  -e JANGOLOVA_PROVIDER_TOKEN \
  -p 127.0.0.1:7391:7391 \
  jangolova/engine-runtime:latest \
  serve-engine-provider --bind 0.0.0.0:7391
```

The image contains Jangolova, Node.js, Playwright Core, Puppeteer Core, and the
browser interaction worker. It deliberately contains no Chromium or display
server.

## Xallet-managed

```text
Xallet provisions target
  Chromium / Unity / display / OCI placement
                 |
                 | typed endpoint or opaque handle
                 v
Jangolova interaction engine
  Playwright / Puppeteer / Unity bridge / Three.js
                 |
                 v
          semantic agent calls
```

Xallet owns target termination and endpoint publication. Jangolova owns only
its interaction connection and can be restarted or disconnected independently.
