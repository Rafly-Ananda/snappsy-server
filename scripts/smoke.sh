#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080}"
ADMIN_USERNAME="${ADMIN_USERNAME:?set ADMIN_USERNAME}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:?set ADMIN_PASSWORD}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

json() {
  jq -r "$1"
}

echo "health"
curl -fsS "$API_BASE/healthz" | grep -q '"status":"live"' || fail "healthz"
curl -fsS "$API_BASE/readyz" | grep -q '"status":"ready"' || fail "readyz"

echo "admin login"
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" \
  "$API_BASE/api/v1/admin/login" | grep -q '"authenticated":true' || fail "admin login"
curl -fsS -b "$COOKIE_JAR" "$API_BASE/api/v1/admin/me" >/dev/null || fail "admin session"

echo "plans"
curl -fsS "$API_BASE/api/v1/plans" | json '.data | length' | grep -Eq '^[1-9][0-9]*$' || fail "plans"

echo "events"
EVENTS="$(curl -fsS -b "$COOKIE_JAR" "$API_BASE/api/v1/admin/events")"
echo "$EVENTS" | json '.data | length' >/dev/null || fail "event list"
LIVE_SLUG="$(echo "$EVENTS" | json '[.data[] | select(.status=="live")][0].slug // ""')"

if [[ -n "$LIVE_SLUG" ]]; then
  echo "public event $LIVE_SLUG"
  curl -fsS "$API_BASE/api/v1/events/slug/$LIVE_SLUG" >/dev/null || fail "public event"
  curl -fsS "$API_BASE/api/v1/events/slug/$LIVE_SLUG/design" >/dev/null || fail "public design"
else
  echo "skip public event: no live event"
fi

echo "smoke OK"
