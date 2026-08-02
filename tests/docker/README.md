# Container Interaction Test Harness

This directory is a reproducible Linux portability fixture. It is not a
Jangolova deployment topology. The fixture may create Xvfb because tests need
to prove that Playwright and Puppeteer can attach to a caller-owned browser.

Build the fixture:

```bash
docker compose -f tests/docker/compose.yaml build engine-test
```

Run both browser interaction engines and the authored-presentation path against
test-owned Chromium and Xvfb:

```bash
docker compose -f tests/docker/compose.yaml run --rm engine-test
```

Run the direct-container presentation conformance test:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/web-presentation-smoke-test.sh \
  engine-test
```

Run Puppeteer over WebDriver BiDi against test-owned Firefox:

```bash
docker compose -f tests/docker/compose.yaml run --rm \
  --entrypoint tests/docker/firefox-bidi-smoke-test.sh \
  engine-test
```

The Chromium fixture also runs the complete reversible Cymonkey augmentation
lifecycle over CDP. The Firefox fixture runs the same client and augmentation
document over WebDriver BiDi. Both verify capability metadata, DOM queries,
overlays, session storage, enable/disable/uninstall, event cursors, and that the
caller-owned browser remains alive after Cymonkey disconnects. The CDP fixture
additionally exercises a non-matching owned interception rule.

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

The presentation fixture co-locates test-owned Xvfb, Chromium, local artifact
servers, and Jangolova to prove the direct-container mode without Xallet. It
supplies Chromium through the generic target descriptor, lets Jangolova select
the presentation engine from the CDP protocol and required capabilities,
resolves an expiring credential reference, authenticates through a test-owned
CDP relay,
mounts a versioned artifact between two localhost origins, verifies state and
artifact revisions, and leaves the target components alive after disconnect.

The fixtures launch Chromium, Firefox, or WebKitGTK directly, give only the
source URL, CDP, WebDriver BiDi, or existing WebDriver-session coordinates to
Jangolova, and verify that disconnecting Jangolova leaves the browser and
independently served presentation target running. No VNC, public-port,
profile-volume, or production placement configuration belongs to this
harness.
