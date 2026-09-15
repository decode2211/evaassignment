#!/usr/bin/env bash
# This script is a plain-English sanity check for a *running* copy of the
# ticket system - either the binary running directly on your machine, or a
# Docker container. It uses nothing but curl, so anyone can read it and
# understand exactly what it is checking without knowing Go.
#
# It walks through the whole user story: register a brand-new account,
# log in, create a ticket, list it, fetch it directly, move it through its
# status lifecycle, and finally confirm that a closed ticket can never be
# reopened. Each step prints PASS or FAIL, and the script exits non-zero
# the moment anything unexpected happens, so it is safe to use as a CI gate.
#
# Usage:
#   ./scripts/smoke_test.sh [BASE_URL]
#   BASE_URL defaults to http://localhost:8080

set -u

BASE_URL="${1:-http://localhost:8080}"
FAILURES=0

# A random email means the script can be re-run over and over against the
# same running server without ever hitting "email already registered".
RAND_ID=$((RANDOM * RANDOM))
EMAIL="smoketest+${RAND_ID}@example.com"
PASSWORD="smoke-test-password"

pass() { echo "  PASS - $1"; }
fail() { echo "  FAIL - $1"; FAILURES=$((FAILURES + 1)); }

# check_status calls a JSON endpoint and verifies the HTTP status code.
# It prints the body too, so a failure is easy to debug by eye.
# Sets the global BODY variable to the response body for the caller to
# inspect further (e.g. to pull a token or ticket ID out of it).
check_status() {
  local description="$1" expected="$2" method="$3" path="$4" data="${5:-}" auth="${6:-}"
  local args=(-s -o /tmp/smoke_body.$$ -w "%{http_code}" -X "$method" "${BASE_URL}${path}")
  if [ -n "$data" ]; then
    args+=(-H "Content-Type: application/json" -d "$data")
  fi
  if [ -n "$auth" ]; then
    args+=(-H "Authorization: Bearer $auth")
  fi

  local code
  code=$(curl "${args[@]}")
  BODY=$(cat /tmp/smoke_body.$$)
  rm -f /tmp/smoke_body.$$

  if [ "$code" = "$expected" ]; then
    pass "$description (HTTP $code)"
  else
    fail "$description (expected HTTP $expected, got HTTP $code, body: $BODY)"
  fi
}

json_field() {
  # Tiny dependency-free JSON string-field extractor: json_field body key
  echo "$1" | grep -o "\"$2\":\"[^\"]*\"" | head -1 | cut -d'"' -f4
}

json_number_field() {
  echo "$1" | grep -o "\"$2\":[0-9]*" | head -1 | grep -o '[0-9]*$'
}

echo "Running smoke test against ${BASE_URL}"
echo "Using test account: ${EMAIL}"
echo

echo "1) Health check"
check_status "GET /health returns ok" "200" "GET" "/health"
if ! echo "$BODY" | grep -q '"status":"ok"'; then
  fail "health body did not contain status:ok (got: $BODY)"
fi
echo

echo "2) Register a new user"
check_status "register succeeds" "201" "POST" "/auth/register" \
  "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\",\"name\":\"Smoke Test\"}"
echo

echo "3) Duplicate registration is rejected"
check_status "duplicate register is 409" "409" "POST" "/auth/register" \
  "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\",\"name\":\"Smoke Test\"}"
echo

echo "4) Login"
check_status "login succeeds" "200" "POST" "/auth/login" \
  "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}"
TOKEN=$(json_field "$BODY" "token")
if [ -z "$TOKEN" ]; then
  fail "login response did not contain a token"
else
  pass "received a token"
fi
echo

echo "5) Wrong password is rejected"
check_status "wrong password is 401" "401" "POST" "/auth/login" \
  "{\"email\":\"${EMAIL}\",\"password\":\"wrong-password\"}"
echo

echo "6) Tickets require authentication"
check_status "GET /tickets with no token is 401" "401" "GET" "/tickets"
echo

echo "7) Create a ticket"
check_status "create ticket succeeds" "201" "POST" "/tickets" \
  '{"title":"Smoke test ticket","description":"created by smoke_test.sh"}' "$TOKEN"
TICKET_ID=$(json_number_field "$BODY" "id")
if ! echo "$BODY" | grep -q '"status":"open"'; then
  fail "new ticket did not start as open (body: $BODY)"
else
  pass "new ticket starts as open"
fi
echo

echo "8) List my tickets"
check_status "list tickets succeeds" "200" "GET" "/tickets" "" "$TOKEN"
if ! echo "$BODY" | grep -q "\"id\":${TICKET_ID}"; then
  fail "ticket list did not contain the ticket we just created"
else
  pass "ticket list contains our new ticket"
fi
echo

echo "9) Get the ticket by ID"
check_status "get ticket by id succeeds" "200" "GET" "/tickets/${TICKET_ID}" "" "$TOKEN"
echo

echo "10) Move ticket open -> in_progress"
check_status "open -> in_progress succeeds" "200" "PATCH" "/tickets/${TICKET_ID}/status" \
  '{"status":"in_progress"}' "$TOKEN"
echo

echo "11) Move ticket in_progress -> closed"
check_status "in_progress -> closed succeeds" "200" "PATCH" "/tickets/${TICKET_ID}/status" \
  '{"status":"closed"}' "$TOKEN"
echo

echo "12) A closed ticket can never be reopened"
check_status "closed -> open is rejected" "400" "PATCH" "/tickets/${TICKET_ID}/status" \
  '{"status":"open"}' "$TOKEN"
echo

echo "----------------------------------------"
if [ "$FAILURES" -eq 0 ]; then
  echo "ALL CHECKS PASSED"
  exit 0
else
  echo "${FAILURES} CHECK(S) FAILED"
  exit 1
fi
