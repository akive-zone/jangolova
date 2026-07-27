#!/usr/bin/env bash
set -euo pipefail

export CDP_HOST="${CDP_HOST:-127.0.0.1}"
export CDP_PORT="${CDP_PORT:-9222}"
export DISPLAY_NUM="${DISPLAY_NUM:-99}"
export DISPLAY=":${DISPLAY_NUM}"
export VNC_LOCALHOST="${VNC_LOCALHOST:-1}"
export PROFILE_DIR="${SMOKE_PROFILE_DIR:-/tmp/xpost-smoke-profile}"
MODE="${1:-all}"

cleanup() {
  scripts/stop-stack.sh || true
}
trap cleanup EXIT

mkdir -p bin out
if [[ ! -x bin/xpost ]]; then
  go build -o bin/xpost ./cmd/xpost
fi
if [[ ! -x bin/xpost-playwright ]]; then
  go build -o bin/xpost-playwright ./cmd/xpost-playwright
fi

run_cdp_smoke() {
  export START_CHROMIUM=1
  scripts/start-stack.sh

  for _ in $(seq 1 40); do
    if curl -fsS "http://127.0.0.1:${CDP_PORT}/json/version" >/dev/null 2>&1; then
      break
    fi
    sleep 0.5
  done

  curl -fsS "http://127.0.0.1:${CDP_PORT}/json/version" >/dev/null

  fixture_url="file://$(pwd)/tests/fixture.html"
  scripts/xpost.sh --mode cdp \
    --cdp "http://127.0.0.1:${CDP_PORT}" \
    --url "${fixture_url}" \
    --text "docker cdp smoke test" \
    --publish \
    --screenshot out/docker-cdp-smoke.png

  test -s out/docker-cdp-smoke.png
  scripts/stop-stack.sh
  echo "docker cdp smoke test passed"
}

run_playwright_smoke() {
  export START_CHROMIUM=0
  scripts/start-stack.sh

  fixture_url="file://$(pwd)/tests/fixture.html"
  scripts/xpost.sh --mode playwright \
    --url "${fixture_url}" \
    --text "docker playwright smoke test" \
    --publish \
    --screenshot out/docker-playwright-smoke.png \
    --profile /tmp/xpost-playwright-smoke-profile

  test -s out/docker-playwright-smoke.png
  scripts/stop-stack.sh
  echo "docker playwright smoke test passed"
}

run_puppeteer_smoke() {
  export START_CHROMIUM=0
  scripts/start-stack.sh

  fixture_url="file://$(pwd)/tests/fixture.html"
  scripts/xpost.sh --mode puppeteer \
    --url "${fixture_url}" \
    --text "docker puppeteer smoke test" \
    --publish \
    --screenshot out/docker-puppeteer-smoke.png \
    --profile /tmp/xpost-puppeteer-smoke-profile

  test -s out/docker-puppeteer-smoke.png
  scripts/stop-stack.sh
  echo "docker puppeteer smoke test passed"
}

case "${MODE}" in
  all)
    run_cdp_smoke
    run_playwright_smoke
    run_puppeteer_smoke
    ;;
  cdp)
    run_cdp_smoke
    ;;
  playwright)
    run_playwright_smoke
    ;;
  puppeteer)
    run_puppeteer_smoke
    ;;
  *)
    echo "unknown smoke mode: ${MODE}; expected all, cdp, playwright, or puppeteer" >&2
    exit 1
    ;;
esac
