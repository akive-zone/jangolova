#!/usr/bin/env bash
set -euo pipefail

RUN_DIR="${RUN_DIR:-${HOME}/.local/run/xpost}"

stop_pid() {
  local name="$1"
  local pid_file="$2"
  if [[ ! -s "${pid_file}" ]]; then
    echo "${name} not running"
    return 0
  fi
  local pid
  pid="$(cat "${pid_file}")"
  if kill -0 "${pid}" 2>/dev/null; then
    kill "${pid}" || true
    echo "stopped ${name}: pid ${pid}"
  else
    echo "${name} pid file was stale: ${pid}"
  fi
  rm -f "${pid_file}"
}

stop_pid "Chromium" "${RUN_DIR}/chromium.pid"
stop_pid "x11vnc" "${RUN_DIR}/x11vnc.pid"
stop_pid "Xvfb" "${RUN_DIR}/xvfb.pid"
