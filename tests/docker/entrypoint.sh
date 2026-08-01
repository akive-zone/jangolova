#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -gt 0 ]]; then
  exec "$@"
fi
tests/docker/browser-interaction-smoke-test.sh
exec tests/docker/web-presentation-smoke-test.sh
