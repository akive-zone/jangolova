#!/usr/bin/env bash
set -euo pipefail

output_path="/tmp/jangolova-native-process-output.jsonl"

bin/jangolova launch-engine \
  --adapter native-process \
  --source examples/native-process/fixture.sh \
  --options '{"startupGrace":"100ms","stopTimeout":"2s"}' \
  >"${output_path}" &
launcher_pid=$!

cleanup() {
  kill -TERM "${launcher_pid}" 2>/dev/null || true
}
trap cleanup EXIT

for _ in $(seq 1 50); do
  if [[ -s "${output_path}" ]]; then
    break
  fi
  sleep 0.1
done

kill -INT "${launcher_pid}"
wait "${launcher_pid}"
trap - EXIT

node -e '
const fs = require("fs");
const lines = fs.readFileSync(process.argv[1], "utf8").trim().split("\n").map(JSON.parse);
if (lines[0]?.adapter !== "native-process" || lines[0]?.status !== "running") {
  throw new Error(`native-process did not launch: ${JSON.stringify(lines)}`);
}
' "${output_path}"

echo "native-process engine smoke test passed"
