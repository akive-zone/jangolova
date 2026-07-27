#!/usr/bin/env bash
set -euo pipefail

cleanup() {
  scripts/stop-stack.sh || true
}
trap cleanup EXIT INT TERM

scripts/start-stack.sh

echo "container ready"
tail -F "${LOG_DIR:-${HOME}/.local/state/xpost}"/*.log &
wait "$!"
