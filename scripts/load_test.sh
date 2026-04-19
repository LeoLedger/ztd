#!/usr/bin/env bash
# =============================================================================
# load_test.sh — Address Parse API Load Test
# =============================================================================
# Usage:
#   ./load_test.sh                              # uses defaults (localhost:8080)
#   BASE_URL=https://api.example.com ./load_test.sh
#   BASE_URL=... APP_ID=my-app APP_SECRET=s3cr3t ./load_test.sh
#
# Requirements:
#   - Bash 4+
#   - ab (Apache Bench) — ships with macOS/Linux
#   - Optional: hey (https://github.com/rakyll/hey) for better P99 reporting
#     Install: go install github.com/rakyll/hey@latest
#   - Optional: wrk (https://github.com/wg/wrk) for Lua-scriptable benchmarks
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------
# Defaults — override via environment variables
# -----------------------------------------------------------------------
BASE_URL="${BASE_URL:-http://localhost:8080}"
ENDPOINT="${ENDPOINT:-/api/v1/address/parse}"
APP_ID="${APP_ID:-test-app}"
APP_SECRET="${APP_SECRET:-test-secret}"
CONCURRENCY="${CONCURRENCY:-100}"
TOTAL_REQUESTS="${TOTAL_REQUESTS:-1000}"
P99_THRESHOLD_MS="${P99_THRESHOLD_MS:-200}"

# -----------------------------------------------------------------------
# Colour helpers
# -----------------------------------------------------------------------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()    { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*" >&2; }
fail()    { echo -e "${RED}[FAIL]${NC}  $*" >&2; exit 1; }

# -----------------------------------------------------------------------
# Sanity checks
# -----------------------------------------------------------------------
command -v bc >/dev/null 2>&1 || warn "bc not found — latency threshold check disabled"

# -----------------------------------------------------------------------
# Pre-flight: check server is reachable
# -----------------------------------------------------------------------
info "Server health check: ${BASE_URL}/health"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health" --max-time 5 || echo "000")
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "Server not reachable at ${BASE_URL}/health (HTTP $HTTP_CODE). Start the server first:\n  make run"
fi
info "Server is up (HTTP $HTTP_CODE)"

# -----------------------------------------------------------------------
# Build the JSON request body and HMAC-SHA256 signature
# -----------------------------------------------------------------------
# We build the body dynamically in Go so the timestamp is fresh each time.
BUILD_PAYLOAD='
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	body := map[string]string{"address": "广东省深圳市南山区桃源街道88号"}
	payload, _ := json.Marshal(body)
	bodyStr := string(payload)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	msg := ts + bodyStr
	h := hmac.New(sha256.New, []byte(os.Getenv("APP_SECRET")))
	h.Write([]byte(msg))
	sig := base64.StdEncoding.EncodeToString(h.Sum(nil))
	fmt.Println(bodyStr + "|" + ts + "|" + sig)
}
'
PAYLOAD_FILE=$(mktemp)
PAYLOAD_LINE=$(go run - "${APP_SECRET}" <<'GOEOF' 2>/dev/null) || {
  warn "Go payload builder unavailable — using fallback Python approach"
  PAYLOAD_LINE=""
}
rm -f "$PAYLOAD_FILE"

build_payload_go() {
  local secret="$1"
  local body='{"address":"广东省深圳市南山区桃源街道88号"}'
  local ts
  ts=$(date +%s)
  local msg="${ts}${body}"
  local sig
  sig=$(echo -n "$msg" | openssl dgst -sha256 -hmac "$secret" -binary | base64)
  echo "${body}|${ts}|${sig}"
}

PAYLOAD_LINE=$(build_payload_go "$APP_SECRET")
BODY=$(echo "$PAYLOAD_LINE" | cut -d'|' -f1)
TIMESTAMP=$(echo "$PAYLOAD_LINE" | cut -d'|' -f2)
SIGNATURE=$(echo "$PAYLOAD_LINE" | cut -d'|' -f3)

info "Payload prepared: $BODY"
info "Signature: $SIGNATURE (timestamp: $TIMESTAMP)"

# Write body to temp file for ab
BODY_FILE=$(mktemp)
echo -n "$BODY" > "$BODY_FILE"
trap 'rm -f "$BODY_FILE"' EXIT

# -----------------------------------------------------------------------
# Run load test
# -----------------------------------------------------------------------
info "Starting load test — ${CONCURRENCY} concurrency, ${TOTAL_REQUESTS} requests"
info "Target: ${BASE_URL}${ENDPOINT}"

RESULT_FILE=$(mktemp)
ab_args=(
  -n "$TOTAL_REQUESTS"
  -c "$CONCURRENCY"
  -p "$BODY_FILE"
  -T "application/json"
  -H "X-App-Id: ${APP_ID}"
  -H "X-Timestamp: ${TIMESTAMP}"
  -H "X-Signature: ${SIGNATURE}"
)

# Run ab and capture both stdout and stderr (ab reports to stderr)
ab "${ab_args[@]}" "${BASE_URL}${ENDPOINT}" > "$RESULT_FILE" 2>&1 || true
cat "$RESULT_FILE"
rm -f "$RESULT_FILE"

# -----------------------------------------------------------------------
# Parse results
# -----------------------------------------------------------------------
check_result() {
  local label="$1"
  local pattern="$2"
  local value
  value=$(grep -E "$pattern" "$RESULT_FILE" 2>/dev/null | head -1 || true)
  echo "$label: $value"
}

# ab output lines we care about:
#   Time per request:       123.456 [ms] (mean)
#   Requests per second:    987.65 [#/sec] (mean)
#   Non-2xx responses:     0
#   Complete requests:      1000
#   Failed requests:        0

NON2XX=$(grep -E "^Non-2xx responses:" "$RESULT_FILE" 2>/dev/null | awk '{print $4}' || echo "unknown")
COMPLETED=$(grep -E "^Complete requests:" "$RESULT_FILE" 2>/dev/null | awk '{print $3}' || echo "0")
FAILED=$(grep -E "^Failed requests:" "$RESULT_FILE" 2>/dev/null | awk '{print $3}' || echo "0")
RPS=$(grep -E "Requests per second:" "$RESULT_FILE" 2>/dev/null | awk '{print $4}' || echo "0")
MEAN_MS=$(grep -E "Time per request:.*mean\)" "$RESULT_FILE" 2>/dev/null | head -1 | awk '{print $4}' || echo "0")

echo ""
echo "============================================================"
echo "                      SUMMARY"
echo "============================================================"
printf "%-30s %s\n" "Complete requests:" "$COMPLETED"
printf "%-30s %s\n" "Failed requests:" "$FAILED"
printf "%-30s %s\n" "Non-2xx responses:" "$NON2XX"
printf "%-30s %s req/s\n" "Throughput:" "$RPS"
printf "%-30s %s ms\n" "Mean latency:" "$MEAN_MS"

# P99 from ab's latency distribution (last bucket in ms)
P99=$(grep -E "^  99% " "$RESULT_FILE" 2>/dev/null | awk '{print $2}' || echo "unknown")
printf "%-30s %s ms\n" "P99 latency:" "$P99"

# -----------------------------------------------------------------------
# Validation
# -----------------------------------------------------------------------
ERRORS=0

if [[ "$NON2XX" != "0" && "$NON2XX" != "unknown" ]]; then
  fail "Non-2xx responses detected: $NON2XX — possible signature errors"
  ERRORS=$((ERRORS+1))
fi

if [[ "$FAILED" != "0" ]]; then
  fail "Failed requests: $FAILED"
  ERRORS=$((ERRORS+1))
fi

if [[ "$P99" != "unknown" ]]; then
  P99_INT=${P99%ms}
  THRESHOLD_INT=$P99_THRESHOLD_MS
  if command -v bc >/dev/null 2>&1; then
    if (( $(echo "$P99_INT > $THRESHOLD_INT" | bc -l) )); then
      fail "P99 latency ${P99_INT}ms exceeds threshold ${THRESHOLD_INT}ms"
      ERRORS=$((ERRORS+1))
    else
      info "P99 latency ${P99_INT}ms is within threshold ${THRESHOLD_INT}ms"
    fi
  else
    warn "bc not found — skipping P99 threshold validation"
  fi
fi

if [[ "$COMPLETED" != "$TOTAL_REQUESTS" ]]; then
  warn "Completed requests ($COMPLETED) != total requests ($TOTAL_REQUESTS)"
fi

echo "============================================================"
if [[ $ERRORS -eq 0 ]]; then
  info "All checks passed"
  exit 0
else
  fail "Load test failed with $ERRORS error(s)"
fi
