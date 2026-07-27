#!/usr/bin/env bash
set -euo pipefail

RUN_DIR="${RUN_DIR:-${HOME}/.local/run/xpost}"
CDP_PORT="${CDP_PORT:-9222}"

check_pid() {
  local name="$1"
  local pid_file="$2"
  if [[ -s "${pid_file}" ]] && kill -0 "$(cat "${pid_file}")" 2>/dev/null; then
    echo "${name}: running pid $(cat "${pid_file}")"
  else
    echo "${name}: stopped"
  fi
}

check_pid "Xvfb" "${RUN_DIR}/xvfb.pid"
check_pid "x11vnc" "${RUN_DIR}/x11vnc.pid"
check_pid "Chromium" "${RUN_DIR}/chromium.pid"

if command -v curl >/dev/null 2>&1; then
  if curl -fsS "http://127.0.0.1:${CDP_PORT}/json/version" >/dev/null 2>&1; then
    echo "CDP: reachable on http://127.0.0.1:${CDP_PORT}"
  else
    echo "CDP: not reachable on http://127.0.0.1:${CDP_PORT}"
  fi
fi
