#!/usr/bin/env bash
set -euo pipefail

MODE="${XPOST_MODE:-cdp}"
ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)
      MODE="${2:-}"
      if [[ -z "${MODE}" ]]; then
        echo "--mode requires cdp or playwright" >&2
        exit 1
      fi
      shift 2
      ;;
    --mode=*)
      MODE="${1#--mode=}"
      shift
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

case "${MODE}" in
  cdp)
    if [[ ! -x bin/xpost ]]; then
      mkdir -p bin
      go build -o bin/xpost ./cmd/xpost
    fi
    exec bin/xpost "${ARGS[@]}"
    ;;
  playwright)
    if [[ ! -x bin/xpost-playwright ]]; then
      mkdir -p bin
      go build -o bin/xpost-playwright ./cmd/xpost-playwright
    fi
    exec bin/xpost-playwright "${ARGS[@]}"
    ;;
  puppeteer)
    exec node scripts/xpost-puppeteer.mjs "${ARGS[@]}"
    ;;
  *)
    echo "unknown mode: ${MODE}; expected cdp, playwright, or puppeteer" >&2
    exit 1
    ;;
esac
