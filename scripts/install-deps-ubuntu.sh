#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "run as root: sudo $0" >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y \
  ca-certificates \
  curl \
  fonts-liberation \
  golang-go \
  unzip \
  x11vnc \
  xvfb

if ! command -v chromium >/dev/null 2>&1 && ! command -v chromium-browser >/dev/null 2>&1; then
  apt-get install -y chromium || apt-get install -y chromium-browser
fi

if ! command -v node >/dev/null 2>&1; then
  echo "Node.js 22.12+ is required for Puppeteer; install it and rerun this script" >&2
  exit 1
fi

node_version="$(node -p 'process.versions.node')"
node_major="${node_version%%.*}"
node_minor="${node_version#*.}"
node_minor="${node_minor%%.*}"
if (( node_major < 22 || (node_major == 22 && node_minor < 12) )); then
  echo "Node.js 22.12+ is required for Puppeteer; found ${node_version}" >&2
  exit 1
fi

cat <<'EOF'
system dependencies installed

Install JavaScript dependencies:
  npm install

Install the Go Playwright protocol driver as the deployment user:
  go run ./cmd/playwright-install
EOF
