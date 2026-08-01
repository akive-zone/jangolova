#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -gt 0 ]]; then
  exec "$@"
fi
exec tests/docker/chromium-engine-smoke-test.sh
