#!/usr/bin/env bash
# FlowForge API smoke test: builds the real binary, boots it with local
# development settings, and verifies every currently exposed endpoint.
# Usage: ./scripts/smoke-api.sh
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${SMOKE_PORT:-18080}"
BASE="http://127.0.0.1:${PORT}"
BIN="$(mktemp -t flowforge-api.XXXXXX)"
LOG="$(mktemp -t flowforge-api-log.XXXXXX)"
trap 'kill "${API_PID:-}" 2>/dev/null || true; rm -f "$BIN" "$LOG"' EXIT

echo "==> building api binary"
go build -o "$BIN" ./apps/api

echo "==> starting api on ${BASE}"
APP_ENV=development \
AUTH_DEV_MODE=true \
API_ADDR="127.0.0.1:${PORT}" \
DATABASE_URL=postgres://flowforge:flowforge@localhost:5432/flowforge \
REDIS_URL=redis://localhost:6379/0 \
SESSION_SECRET=dev-only-insecure-secret-change-me \
"$BIN" >"$LOG" 2>&1 &
API_PID=$!

for _ in $(seq 1 50); do
  if curl -sf "${BASE}/healthz" >/dev/null 2>&1; then break; fi
  sleep 0.1
done

failures=0

check() {
  local name="$1" want="$2" actual="$3"
  if [[ "$actual" == "$want" ]]; then
    echo "PASS ${name} (${actual})"
  else
    echo "FAIL ${name}: got ${actual}, want ${want}"
    failures=$((failures + 1))
  fi
}

check "GET /healthz -> 200" 200 \
  "$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/healthz")"

check "GET /api/v1/ping -> 200" 200 \
  "$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/api/v1/ping")"

check "GET /api/v1/me without credentials -> 401" 401 \
  "$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/api/v1/me")"

check "GET /api/v1/me with dev identity -> 200" 200 \
  "$(curl -s -o /dev/null -w '%{http_code}' -H 'X-Dev-User: smoke' "${BASE}/api/v1/me")"

body="$(curl -s -H 'X-Dev-User: smoke' "${BASE}/api/v1/me")"
if [[ "$body" == *'"subject":"dev:smoke"'* ]]; then
  echo "PASS /api/v1/me returns dev identity"
else
  echo "FAIL /api/v1/me body: ${body}"
  failures=$((failures + 1))
fi

# Readiness reflects real dependency reachability: 200 when postgres+redis
# are up locally, 503 otherwise. Both are valid smoke outcomes; the body
# must always be well-formed.
ready_code="$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/readyz")"
ready_body="$(curl -s "${BASE}/readyz")"
if [[ "$ready_code" =~ ^(200|503)$ ]] && [[ "$ready_body" == *'"checks"'* ]]; then
  echo "PASS /readyz -> ${ready_code} with structured body"
else
  echo "FAIL /readyz: code=${ready_code} body=${ready_body}"
  failures=$((failures + 1))
fi

req_id="$(curl -s -D - -o /dev/null -H 'X-Request-ID: smoke-corr-1' "${BASE}/api/v1/ping" | tr -d '\r' | awk 'tolower($1)=="x-request-id:" {print $2}')"
check "X-Request-ID echoed" "smoke-corr-1" "$req_id"

echo "==> stopping api"
kill "$API_PID" 2>/dev/null || true
wait "$API_PID" 2>/dev/null || true

if [[ "$failures" -gt 0 ]]; then
  echo "SMOKE FAILED (${failures} failure(s)); server log: ${LOG}"
  tail -20 "$LOG" || true
  exit 1
fi
echo "SMOKE OK"
