# Xpost Browser Automation Prototype

This is a Go and shell implementation of a headed-browser VPS setup, without OpenClaw.

The stack is:

- Xvfb creates a virtual display, default `:99`.
- x11vnc exposes that display for manual login/debugging, default `127.0.0.1:5999`.
- Chromium runs headed on the Xvfb display with CDP on `127.0.0.1:9222`.
- CDP mode uses the Go command to connect to Chromium's CDP endpoint, open `https://x.com/compose/post`, fill the composer, and optionally click Post.
- Playwright mode uses the community Go binding to launch a browser with the same persistent profile concept and post options.
- Puppeteer mode uses `puppeteer-core` with the installed Chromium/Chrome binary and the same persistent profile and post options.

Use this only with accounts and workflows you are authorized to automate, and keep the browser behavior within the target site rules.

## VPS Setup

Ubuntu/Debian:

```bash
# Install Node.js 22.12+ before running the dependency script.
sudo scripts/install-deps-ubuntu.sh
npm install
go run ./cmd/playwright-install
scripts/start-stack.sh
```

The scripts bind VNC and CDP to localhost by default. From your laptop:

```bash
ssh -L 5999:127.0.0.1:5999 -L 9222:127.0.0.1:9222 user@your-vps
```

Connect your VNC viewer to `127.0.0.1:5999`, open X, and log in manually once. The session is persisted in:

```text
~/.local/share/chromium-xpost-profile
```

## Post Flow: CDP Mode

Build the Go command:

```bash
mkdir -p bin
go build -o bin/xpost ./cmd/xpost
```

Fill the composer without publishing:

```bash
bin/xpost --text "Testing from the VPS browser stack"
```

Publish:

```bash
bin/xpost --text "Testing from the VPS browser stack" --publish
```

Optional screenshot:

```bash
bin/xpost --text "Testing" --screenshot out/xpost.png
```

You can also use the dispatcher:

```bash
scripts/xpost.sh --mode cdp --text "Testing from CDP mode"
```

## Post Flow: Go Playwright Mode

Install the version-matched Playwright driver once:

```bash
go run ./cmd/playwright-install
```

The Go binding uses Playwright's protocol driver but launches the Chromium/Chrome already installed on the machine, so it does not download a separate browser.

Start only the display/VNC stack and let Go Playwright launch the browser:

```bash
START_CHROMIUM=0 scripts/start-stack.sh
```

Run the command with the Xvfb display selected. Fill the composer without publishing:

```bash
DISPLAY=:99 scripts/xpost.sh --mode playwright --text "Testing from Go Playwright mode"
```

Publish:

```bash
DISPLAY=:99 scripts/xpost.sh --mode playwright --text "Testing from Go Playwright mode" --publish
```

Optional screenshot:

```bash
DISPLAY=:99 scripts/xpost.sh --mode playwright --text "Testing" --screenshot out/xpost-playwright.png
```

Override browser detection with:

```bash
DISPLAY=:99 PLAYWRIGHT_BROWSER_PATH=/usr/bin/google-chrome-stable \
  scripts/xpost.sh --mode playwright --text "Testing"
```

The Go Playwright binding is maintained at
[`github.com/mxschmitt/playwright-go`](https://github.com/mxschmitt/playwright-go).
It is a community binding rather than an official Playwright language client.

## Post Flow: Puppeteer Mode

Puppeteer requires Node.js 22.12+ and the project dependencies:

```bash
npm install
```

As with Go Playwright, start the display/VNC stack without its own Chromium process:

```bash
START_CHROMIUM=0 scripts/start-stack.sh
```

Fill the composer without publishing:

```bash
DISPLAY=:99 scripts/xpost.sh --mode puppeteer --text "Testing from Puppeteer"
```

Publish:

```bash
DISPLAY=:99 scripts/xpost.sh --mode puppeteer \
  --text "Testing from Puppeteer" --publish
```

Optional screenshot:

```bash
DISPLAY=:99 scripts/xpost.sh --mode puppeteer \
  --text "Testing" --screenshot out/xpost-puppeteer.png
```

Puppeteer mode uses `puppeteer-core`, so it does not download another browser.
Override browser detection with `PUPPETEER_BROWSER_PATH` or `--executable`.

## Operations

```bash
scripts/status.sh
scripts/stop-stack.sh
```

## Docker Testing

Build the image:

```bash
docker compose build
```

Run the automated smoke test for both modes:

```bash
docker compose run --rm --entrypoint scripts/docker-smoke-test.sh xpost
```

Run one mode only:

```bash
docker compose run --rm --entrypoint scripts/docker-smoke-test.sh xpost cdp
docker compose run --rm --entrypoint scripts/docker-smoke-test.sh xpost playwright
docker compose run --rm --entrypoint scripts/docker-smoke-test.sh xpost puppeteer
```

The smoke test starts the headed browser stack in the container and uses a local HTML fixture in `tests/fixture.html`, so it verifies CDP, Go Playwright, Puppeteer, Xvfb, Chromium, fill/click behavior, and screenshot capture without logging in to X or publishing anything. Each mode uses a temporary profile separate from the persisted manual-login profile volume.

Run the container for manual browser testing:

```bash
docker compose up
```

Then connect:

- CDP: `http://127.0.0.1:9222`
- VNC: `127.0.0.1:5999`

The Docker profile is persisted in the `xpost-profile` volume.
Docker enables stale Chromium profile-lock cleanup at container start. Do not run multiple containers against the same profile volume at the same time.

Useful environment variables:

- `DISPLAY_NUM=99`
- `GEOMETRY=1920x1080x24`
- `CDP_PORT=9222`
- `START_CHROMIUM=1`
- `PROFILE_DIR=$HOME/.local/share/chromium-xpost-profile`
- `PLAYWRIGHT_BROWSER_PATH=/path/to/chrome-or-chromium`
- `PUPPETEER_BROWSER_PATH=/path/to/chrome-or-chromium`
- `PUPPETEER_HEADLESS=1`
- `VNC_LOCALHOST=1`
- `VNC_PASSWORD=...`

If this runs on a machine where exposing VNC is acceptable, set `VNC_LOCALHOST=0`; otherwise prefer SSH tunneling.
