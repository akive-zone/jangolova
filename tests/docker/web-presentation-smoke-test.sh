#!/usr/bin/env bash
set -euo pipefail

# Chromium, Xvfb, and the presentation HTTP server are fixtures owned by this
# test. Jangolova receives only their source URL and CDP endpoint.
display_number="${DISPLAY_NUM:-100}"
export DISPLAY=":${display_number}"
profile_path="/tmp/jangolova-presentation-target-profile"
provider_log="/tmp/jangolova-presentation-provider.log"
target_log="/tmp/jangolova-presentation-target.log"
server_log="/tmp/jangolova-presentation-server.log"
asset_server_log="/tmp/jangolova-presentation-asset-server.log"
relay_log="/tmp/jangolova-authenticated-cdp-relay.log"
token="test-only-web-presentation-token"
cdp_authorization="Bearer test-only-remote-cdp-secret"
credential_ref="direct-container-session"

Xvfb "${DISPLAY}" -screen 0 1280x720x24 -ac +extension GLX +render -noreset > /tmp/jangolova-presentation-xvfb.log 2>&1 &
xvfb_pid=$!
PRESENTATION_TARGET_PORT=8081 node tests/web-presentation-target-server.mjs > "${server_log}" 2>&1 &
server_pid=$!
PRESENTATION_TARGET_PORT=8082 node tests/web-presentation-target-server.mjs > "${asset_server_log}" 2>&1 &
asset_server_pid=$!
chromium \
  --no-first-run \
  --no-default-browser-check \
  --no-sandbox \
  --disable-dev-shm-usage \
  --remote-debugging-address=127.0.0.1 \
  --remote-debugging-port=9224 \
  --user-data-dir="${profile_path}" \
  about:blank > "${target_log}" 2>&1 &
target_pid=$!
CDP_AUTHORIZATION="${cdp_authorization}" node tests/authenticated-cdp-relay.mjs > "${relay_log}" 2>&1 &
relay_pid=$!
credential_expires_at="$(node -e 'process.stdout.write(new Date(Date.now() + 300000).toISOString())')"
credential_document="{\"apiVersion\":\"interaction.connection/v1alpha1\",\"kind\":\"credential\",\"headers\":{\"Authorization\":\"${cdp_authorization}\"},\"expiresAt\":\"${credential_expires_at}\"}"
JANGOLOVA_CREDENTIAL_DIRECT_2DCONTAINER_2DSESSION="${credential_document}" \
  JANGOLOVA_PROVIDER_TOKEN="${token}" \
  bin/jangolova serve-engine-provider --bind 127.0.0.1:7392 > "${provider_log}" 2>&1 &
provider_pid=$!

cleanup() {
  kill -TERM "${provider_pid}" "${relay_pid}" "${target_pid}" "${server_pid}" "${asset_server_pid}" "${xvfb_pid}" 2>/dev/null || true
}
trap cleanup EXIT
trap 'cat "${provider_log}" >&2; cat "${relay_log}" >&2; cat "${target_log}" >&2; cat "${server_log}" >&2; cat "${asset_server_log}" >&2' ERR

for _ in $(seq 1 150); do
  if curl -fsS http://127.0.0.1:9224/json/version >/dev/null 2>&1 && \
     curl -fsS http://127.0.0.1:7392/healthz >/dev/null 2>&1 && \
     curl -fsS -H "Authorization: ${cdp_authorization}" http://127.0.0.1:9333/json/version >/dev/null 2>&1 && \
     curl -fsS http://127.0.0.1:8081/ >/dev/null 2>&1 && \
     curl -fsS http://127.0.0.1:8082/ >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
curl -fsS http://127.0.0.1:9224/json/version >/dev/null
curl -fsS http://127.0.0.1:7392/healthz >/dev/null
if curl -fsS http://127.0.0.1:9333/json/version >/dev/null 2>&1; then
  echo "authenticated CDP relay accepted an unauthenticated request" >&2
  exit 1
fi
curl -fsS -H "Authorization: ${cdp_authorization}" http://127.0.0.1:9333/json/version >/dev/null
curl -fsS http://127.0.0.1:8081/ >/dev/null
curl -fsS http://127.0.0.1:8082/ >/dev/null

JANGOLOVA_PROVIDER_TOKEN="${token}" \
  PRESENTATION_AUTHENTICATED_CDP_BASE="ws://127.0.0.1:9333" \
  PRESENTATION_CREDENTIAL_REF="${credential_ref}" \
  node tests/web-presentation-smoke-client.mjs
if grep -F "${cdp_authorization}" "${provider_log}" >/dev/null 2>&1; then
  echo "resolved CDP credential leaked into provider logs" >&2
  exit 1
fi
echo "Authenticated credential-reference CDP attachment passed"

# Disconnecting Jangolova must not terminate the caller-owned Chromium or its
# independently served presentation target.
curl -fsS http://127.0.0.1:9224/json/version >/dev/null
curl -fsS -H "Authorization: ${cdp_authorization}" http://127.0.0.1:9333/json/version >/dev/null
curl -fsS http://127.0.0.1:8081/ >/dev/null
curl -fsS http://127.0.0.1:8082/ >/dev/null

echo "Direct-container web presentation remained target-preserving"
