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

Run Puppeteer over WebDriver BiDi against test-owned Firefox:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/firefox-bidi-smoke-test.sh \
  engine-test
```

Run WebDriver Classic against a test-owned WebKitGTK session:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/webkit-webdriver-smoke-test.sh \
  engine-test
```

Run the Unity package contract test:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/unity-package-contract-test.sh \
  engine-test
```

The fixtures launch Chromium, Firefox, or WebKitGTK directly, give only the
CDP, WebDriver BiDi, or existing WebDriver-session coordinates to Jangolova,
and verify that disconnecting Jangolova leaves the browser running. No VNC,
public-port, profile-volume, or production placement configuration belongs to
this harness.
