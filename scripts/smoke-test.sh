#!/usr/bin/env bash
#
# Smoke test for request-service — ACRM-79.
#
# A quick (<10 min) sanity check that the service is up and the core happy path,
# authorization and negative branches work. Run it after every deployment or after
# starting the local environment.
#
# Covers SM-01..SM-10 from ACRM-79:
#   SM-01  GET  /health                     -> 200 {"status":"ok"}
#   SM-02  GET  /swagger/index.html         -> 200 (Swagger UI loads)
#   SM-03  POST /requests            client -> 201, body has id + status "new"
#   SM-04  GET  /requests/{id}       client -> 200, returns the created request
#   SM-05  PATCH /requests/{id}/status appr -> 200, status -> in_progress
#   SM-06  GET  /requests/{id}       client -> status in_progress, updated_at bumped
#   SM-07  GET  /requests/{id}       no tok -> 401
#   SM-08  PATCH /requests/{id}/status client-> 403
#   SM-09  GET  /requests/<zero-uuid> appr  -> 404
#   SM-10  POST /requests (bad email) client-> 400
#
# Requirements: bash, curl, jq.
#
# Configuration via environment variables (defaults match infra/docker-compose.yml
# and docs/qa-testing.md):
#
#   BASE_URL         request-service base URL     (default http://localhost:8080)
#   KEYCLOAK_URL     Keycloak base URL            (default http://localhost:8180)
#   REALM            Keycloak realm               (default appraisal)
#   CLIENT_ID        OIDC public client_id        (default appraisal-frontend)
#   CLIENT_USER      username with role client    (default qa_client)
#   CLIENT_PASS      password for CLIENT_USER      (default test)
#   APPRAISER_USER   username with role appraiser  (default qa_appraiser)
#   APPRAISER_PASS   password for APPRAISER_USER   (default test)
#
# Usage:
#   ./scripts/smoke-test.sh
#   CLIENT_USER=alice CLIENT_PASS=secret ./scripts/smoke-test.sh
#
# Exit code 0 if every check passes, 1 otherwise.

set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8180}"
REALM="${REALM:-appraisal}"
CLIENT_ID="${CLIENT_ID:-appraisal-frontend}"
CLIENT_USER="${CLIENT_USER:-qa_client}"
CLIENT_PASS="${CLIENT_PASS:-test}"
APPRAISER_USER="${APPRAISER_USER:-qa_appraiser}"
APPRAISER_PASS="${APPRAISER_PASS:-test}"

ZERO_UUID="00000000-0000-0000-0000-000000000000"
TOKEN_URL="${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/token"

PASS=0
FAIL=0

# --- tiny output helpers -----------------------------------------------------
c_green=$'\033[0;32m'; c_red=$'\033[0;31m'; c_dim=$'\033[0;90m'; c_off=$'\033[0m'
[ -t 1 ] || { c_green=; c_red=; c_dim=; c_off=; }

pass() { printf '%s  PASS%s  %s\n' "$c_green" "$c_off" "$1"; PASS=$((PASS+1)); }
fail() { printf '%s  FAIL%s  %s\n' "$c_red" "$c_off" "$1"; [ -n "${2:-}" ] && printf '        %s%s%s\n' "$c_dim" "$2" "$c_off"; FAIL=$((FAIL+1)); }
info() { printf '%s· %s%s\n' "$c_dim" "$1" "$c_off"; }

die() { printf '%sfatal:%s %s\n' "$c_red" "$c_off" "$1" >&2; exit 2; }

# assert HTTP status; $1=label $2=expected $3=actual [$4=extra detail on fail]
check_code() {
  if [ "$3" = "$2" ]; then
    pass "$1 (HTTP $3)"
  else
    fail "$1 (expected $2, got $3)" "${4:-}"
  fi
}

HTTP_CODE=""

# --- preflight ---------------------------------------------------------------
command -v curl >/dev/null 2>&1 || die "curl is not installed"
command -v jq   >/dev/null 2>&1 || die "jq is not installed"

# HTTP helper. Prints the response status code to stdout and writes the response
# body to $BODY_FILE. Returning the code via stdout (rather than a global) keeps
# it correct even when the caller runs http in a command-substitution subshell.
BODY_FILE="$(mktemp)"
trap 'rm -f "$BODY_FILE"' EXIT

http() {
  local method="$1" url="$2"; shift 2
  curl -s -o "$BODY_FILE" -w '%{http_code}' -X "$method" "$url" "$@" 2>/dev/null || printf '000'
}
body() { cat "$BODY_FILE"; }

get_token() {
  local user="$1" pass="$2" resp token
  resp="$(curl -s -X POST "$TOKEN_URL" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=password&client_id=${CLIENT_ID}&username=${user}&password=${pass}")"
  token="$(printf '%s' "$resp" | jq -r '.access_token // empty' 2>/dev/null)"
  if [ -z "$token" ]; then
    local err
    err="$(printf '%s' "$resp" | jq -r '.error_description // .error // "no access_token in response"' 2>/dev/null)"
    die "could not get token for '${user}': ${err} (checked ${TOKEN_URL}, client_id=${CLIENT_ID})"
  fi
  printf '%s' "$token"
}

echo "request-service smoke test"
info "service:  $BASE_URL"
info "keycloak: $TOKEN_URL (client_id=$CLIENT_ID)"
echo

info "obtaining tokens…"
CLIENT_TOKEN="$(get_token "$CLIENT_USER" "$CLIENT_PASS")"
APPRAISER_TOKEN="$(get_token "$APPRAISER_USER" "$APPRAISER_PASS")"
echo

# --- SM-01  health -----------------------------------------------------------
HTTP_CODE="$(http GET "$BASE_URL/health")"
if [ "$HTTP_CODE" = "200" ] && [ "$(body | jq -r '.status // empty' 2>/dev/null)" = "ok" ]; then
  pass "SM-01 GET /health -> 200 {\"status\":\"ok\"}"
else
  fail "SM-01 GET /health" "code=$HTTP_CODE body=$(body)"
fi

# --- SM-02  swagger ----------------------------------------------------------
HTTP_CODE="$(http GET "$BASE_URL/swagger/index.html")"
check_code "SM-02 GET /swagger/index.html" 200 "$HTTP_CODE"

# --- SM-03  create request (client) -----------------------------------------
create_body='{"email":"smoke@test.com","phone_number":"+79161234567","object_type":"apartment","address":"Moscow, Test Street, 1"}'
HTTP_CODE="$(http POST "$BASE_URL/requests" \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$create_body")"
REQ_ID="$(body | jq -r '.id // empty' 2>/dev/null)"
created_status="$(body | jq -r '.status // empty' 2>/dev/null)"
created_updated_at="$(body | jq -r '.updated_at // empty' 2>/dev/null)"
if [ "$HTTP_CODE" = "201" ] && [ -n "$REQ_ID" ] && [ "$created_status" = "new" ]; then
  pass "SM-03 POST /requests -> 201 (id=$REQ_ID, status=new)"
else
  fail "SM-03 POST /requests" "code=$HTTP_CODE body=$(body)"
fi

# Everything below needs a request id. If create failed, skip the dependent checks.
if [ -z "$REQ_ID" ]; then
  info "SM-03 produced no request id; skipping SM-04..SM-09"
else
  # --- SM-04  get created request (client) ----------------------------------
  HTTP_CODE="$(http GET "$BASE_URL/requests/$REQ_ID" -H "Authorization: Bearer $CLIENT_TOKEN")"
  got_id="$(body | jq -r '.id // empty' 2>/dev/null)"
  if [ "$HTTP_CODE" = "200" ] && [ "$got_id" = "$REQ_ID" ]; then
    pass "SM-04 GET /requests/{id} -> 200 (returns created request)"
  else
    fail "SM-04 GET /requests/{id}" "code=$HTTP_CODE body=$(body)"
  fi

  # --- SM-05  change status new -> in_progress (appraiser) -------------------
  HTTP_CODE="$(http PATCH "$BASE_URL/requests/$REQ_ID/status" \
    -H "Authorization: Bearer $APPRAISER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"in_progress"}')"
  new_status="$(body | jq -r '.status // empty' 2>/dev/null)"
  if [ "$HTTP_CODE" = "200" ] && [ "$new_status" = "in_progress" ]; then
    pass "SM-05 PATCH /requests/{id}/status -> 200 (status=in_progress)"
  else
    fail "SM-05 PATCH /requests/{id}/status" "code=$HTTP_CODE body=$(body)"
  fi

  # --- SM-06  re-read: status in_progress, updated_at bumped ----------------
  HTTP_CODE="$(http GET "$BASE_URL/requests/$REQ_ID" -H "Authorization: Bearer $CLIENT_TOKEN")"
  reread_status="$(body | jq -r '.status // empty' 2>/dev/null)"
  reread_updated_at="$(body | jq -r '.updated_at // empty' 2>/dev/null)"
  if [ "$HTTP_CODE" = "200" ] && [ "$reread_status" = "in_progress" ] && \
     [ -n "$reread_updated_at" ] && [ "$reread_updated_at" != "$created_updated_at" ]; then
    pass "SM-06 GET /requests/{id} -> status in_progress, updated_at bumped"
  else
    fail "SM-06 GET /requests/{id} after status change" \
         "code=$HTTP_CODE status=$reread_status updated_at(old=$created_updated_at new=$reread_updated_at)"
  fi

  # --- SM-07  get without token -> 401 --------------------------------------
  HTTP_CODE="$(http GET "$BASE_URL/requests/$REQ_ID")"
  check_code "SM-07 GET /requests/{id} without token" 401 "$HTTP_CODE"

  # --- SM-08  client tries to change status -> 403 --------------------------
  HTTP_CODE="$(http PATCH "$BASE_URL/requests/$REQ_ID/status" \
    -H "Authorization: Bearer $CLIENT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"inspection_scheduled"}')"
  check_code "SM-08 PATCH /requests/{id}/status as client" 403 "$HTTP_CODE" "body=$(body)"
fi

# --- SM-09  unknown request -> 404 ------------------------------------------
HTTP_CODE="$(http GET "$BASE_URL/requests/$ZERO_UUID" -H "Authorization: Bearer $APPRAISER_TOKEN")"
check_code "SM-09 GET /requests/<zero-uuid>" 404 "$HTTP_CODE" "body=$(body)"

# --- SM-10  invalid email -> 400 --------------------------------------------
HTTP_CODE="$(http POST "$BASE_URL/requests" \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"not-an-email","phone_number":"+79161234567"}')"
check_code "SM-10 POST /requests with invalid email" 400 "$HTTP_CODE" "body=$(body)"

# --- summary -----------------------------------------------------------------
echo
printf 'Result: %s%d passed%s, %s%d failed%s\n' \
  "$c_green" "$PASS" "$c_off" "$([ "$FAIL" -gt 0 ] && printf '%s' "$c_red")" "$FAIL" "$c_off"

[ "$FAIL" -eq 0 ]
