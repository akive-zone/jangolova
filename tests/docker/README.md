# Container Interaction Test Harness

This directory is a reproducible Linux portability fixture. It is not a
Jangolova deployment topology. The fixture may create Xvfb because tests need
to prove that Playwright and Puppeteer can attach to a caller-owned browser.

Build the fixture:

```bash
docker compose -f tests/docker/compose.yaml build engine-test
```

Run both browser interaction engines against test-owned Chromium and Xvfb:

```bash
docker compose -f tests/docker/compose.yaml run --rm engine-test
```

Run the Unity package contract test:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/unity-package-contract-test.sh \
  engine-test
```

The test launches Chromium directly, gives its CDP endpoint to Jangolova, and
verifies that disconnecting Jangolova leaves Chromium running. No VNC,
session, public-port, profile-volume, or production placement configuration
belongs to this harness.
