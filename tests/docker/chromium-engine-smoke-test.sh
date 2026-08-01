#!/usr/bin/env bash
set -euo pipefail

display_number="${DISPLAY_NUM:-99}"
export DISPLAY=":${display_number}"
output_path="/tmp/jangolova-chromium-output.jsonl"
xvfb_log="/tmp/jangolova-xvfb.log"

Xvfb "${DISPLAY}" -screen 0 1280x720x24 -ac +extension GLX +render -noreset \
  >"${xvfb_log}" 2>&1 &
xvfb_pid=$!

bin/jangolova launch-engine \
  --adapter chromium \
  --source "file://$(pwd)/tests/fixture.html" \
  --env "DISPLAY=${DISPLAY}" \
  --options '{"address":"http://127.0.0.1:9222","headless":false,"noSandbox":true}' \
  >"${output_path}" &
launcher_pid=$!

cleanup() {
  kill -TERM "${launcher_pid}" 2>/dev/null || true
  kill -TERM "${xvfb_pid}" 2>/dev/null || true
}
trap cleanup EXIT

for _ in $(seq 1 100); do
  if curl -fsS http://127.0.0.1:9222/json/version >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
curl -fsS http://127.0.0.1:9222/json/version >/dev/null

kill -INT "${launcher_pid}"
wait "${launcher_pid}"
kill -TERM "${xvfb_pid}" 2>/dev/null || true
wait "${xvfb_pid}" 2>/dev/null || true
trap - EXIT

node -e '
const fs = require("fs");
const lines = fs.readFileSync(process.argv[1], "utf8").trim().split("\n").map(JSON.parse);
if (lines[0]?.adapter !== "chromium" || lines[0]?.endpoints?.[0]?.protocol !== "cdp") {
  throw new Error(`Chromium endpoint was not reported: ${JSON.stringify(lines)}`);
}
' "${output_path}"

echo "Chromium engine smoke test passed"
