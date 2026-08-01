# Jangolova Engine Runtime Image

This optional artifact packages the Jangolova engine provider with Chromium.
It can be run independently or as a Xallet-managed workload. It contains engine
lifecycle code only and does not define display/container topology.

Build it locally from the Jangolova repository:

```sh
docker build -f deploy/engine-runtime/Containerfile \
  -t jangolova/engine-runtime:latest .
```

This is currently a local image name; no registry publication is implied.

The provider requires `JANGOLOVA_PROVIDER_TOKEN`. Its operator supplies that
secret and any `DISPLAY` or native environment. The image intentionally does
not declare volumes, published ports, display services, or a production
network topology. When Xallet is the operator, Xallet owns those values and
the private application network.

Chromium CDP remains loopback-only by default. Xallet's container launch adds
the explicit `allowRemoteDebugging` option, publishes CDP only on host
loopback, and resolves the returned container endpoint before attaching its
controller.
