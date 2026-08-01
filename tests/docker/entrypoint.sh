#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -gt 0 ]]; then
  exec "$@"
fi
exec tests/docker/browser-interaction-smoke-test.sh
