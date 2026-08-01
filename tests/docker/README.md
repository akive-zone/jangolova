# Container Engine Test Harness

This directory is a reproducible Linux portability fixture. It is not a
Jangolova deployment topology. The fixture may create Xvfb because tests need
to prove that an engine can consume a caller-owned external display.

Build the fixture:

```bash
docker compose -f tests/docker/compose.yaml build engine-test
```

Run headed Chromium against test-owned Xvfb:

```bash
docker compose -f tests/docker/compose.yaml run --rm engine-test
```

Run the native-process lifecycle test:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/native-process-smoke-test.sh \
  engine-test
```

Run the Unity package contract test:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/unity-package-contract-test.sh \
  engine-test
```

No VNC, controller, session, public-port, profile-volume, or production
placement configuration belongs to this harness.
