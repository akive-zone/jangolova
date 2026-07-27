#!/usr/bin/env bash
set -euo pipefail

DISPLAY_NUM="${DISPLAY_NUM:-99}"
DISPLAY=":${DISPLAY_NUM}"
GEOMETRY="${GEOMETRY:-1920x1080x24}"
CDP_PORT="${CDP_PORT:-9222}"
CDP_HOST="${CDP_HOST:-127.0.0.1}"
VNC_PORT="${VNC_PORT:-$((5900 + DISPLAY_NUM))}"
VNC_HOST="127.0.0.1"
PROFILE_DIR="${PROFILE_DIR:-${HOME}/.local/share/chromium-xpost-profile}"
RUN_DIR="${RUN_DIR:-${HOME}/.local/run/xpost}"
LOG_DIR="${LOG_DIR:-${HOME}/.local/state/xpost}"
VNC_LOCALHOST="${VNC_LOCALHOST:-1}"
START_CHROMIUM="${START_CHROMIUM:-1}"

mkdir -p "${PROFILE_DIR}" "${RUN_DIR}" "${LOG_DIR}"

find_chromium() {
  for bin in chromium chromium-browser google-chrome google-chrome-stable; do
    if command -v "${bin}" >/dev/null 2>&1; then
      command -v "${bin}"
      return 0
    fi
  done
  echo "chromium/google-chrome not found" >&2
  return 1
}

start_if_missing() {
  local name="$1"
  local pid_file="$2"
  local log_file="${LOG_DIR}/${name}.log"
  shift 2
  if [[ -s "${pid_file}" ]] && kill -0 "$(cat "${pid_file}")" 2>/dev/null; then
    echo "${name} already running: pid $(cat "${pid_file}")"
    return 0
  fi
  "$@" >>"${log_file}" 2>&1 &
  local pid=$!
  echo "${pid}" >"${pid_file}"
  echo "started ${name}: pid ${pid}, log ${log_file}"
}

CHROMIUM_BIN=""
if [[ "${START_CHROMIUM}" == "1" ]]; then
  CHROMIUM_BIN="$(find_chromium)"
fi

start_if_missing "Xvfb" "${RUN_DIR}/xvfb.pid" \
  Xvfb "${DISPLAY}" -screen 0 "${GEOMETRY}" -ac +extension GLX +render -noreset

sleep 1

VNC_ARGS=(-display "${DISPLAY}" -rfbport "${VNC_PORT}" -forever -shared -nopw)
if [[ "${VNC_LOCALHOST}" == "1" ]]; then
  VNC_ARGS+=(-localhost)
else
  VNC_HOST="0.0.0.0"
fi
if [[ -n "${VNC_PASSWORD:-}" ]]; then
  PASS_FILE="${RUN_DIR}/vnc.pass"
  x11vnc -storepasswd "${VNC_PASSWORD}" "${PASS_FILE}" >/dev/null
  chmod 600 "${PASS_FILE}"
  VNC_ARGS=(-display "${DISPLAY}" -rfbport "${VNC_PORT}" -forever -shared -rfbauth "${PASS_FILE}")
  if [[ "${VNC_LOCALHOST}" == "1" ]]; then
    VNC_ARGS+=(-localhost)
  else
    VNC_HOST="0.0.0.0"
  fi
fi

start_if_missing "x11vnc" "${RUN_DIR}/x11vnc.pid" \
  x11vnc "${VNC_ARGS[@]}"

export DISPLAY
if [[ "${START_CHROMIUM}" == "1" ]]; then
  if [[ "${CHROMIUM_CLEAR_STALE_LOCKS:-0}" == "1" ]]; then
    rm -f "${PROFILE_DIR}/SingletonLock" "${PROFILE_DIR}/SingletonCookie" "${PROFILE_DIR}/SingletonSocket"
  fi
  CHROMIUM_ARGS=(
    --remote-debugging-address="${CDP_HOST}"
    --remote-debugging-port="${CDP_PORT}"
    --user-data-dir="${PROFILE_DIR}"
    --no-first-run
    --no-default-browser-check
    --disable-dev-shm-usage
    --password-store=basic
    --start-maximized
  )
  if [[ "${EUID}" -eq 0 ]]; then
    CHROMIUM_ARGS+=(--no-sandbox)
  fi
  start_if_missing "Chromium" "${RUN_DIR}/chromium.pid" \
    "${CHROMIUM_BIN}" \
      "${CHROMIUM_ARGS[@]}" \
      https://x.com
else
  rm -f "${RUN_DIR}/chromium.pid"
fi

cat <<EOF

Stack is running.
DISPLAY=${DISPLAY}
CDP=http://${CDP_HOST}:${CDP_PORT}
VNC=${VNC_HOST}:${VNC_PORT}
PROFILE_DIR=${PROFILE_DIR}
START_CHROMIUM=${START_CHROMIUM}

For a remote VPS, tunnel VNC and CDP from your laptop:
ssh -L ${VNC_PORT}:127.0.0.1:${VNC_PORT} -L ${CDP_PORT}:127.0.0.1:${CDP_PORT} user@your-vps
EOF
