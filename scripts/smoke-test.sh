#!/bin/bash
# M API Smoke Test
# Quick health check script for verifying the API is operational before demos.
#
# Usage: ./scripts/smoke-test.sh [BASE_URL] [API_KEY]
#
# Environment variables (override defaults):
#   M_BASE_URL - Server URL (default: http://localhost:8080)
#   M_API_KEY  - API key for authentication (default: test-api-key)

set -e

# Configuration
BASE_URL="${1:-${M_BASE_URL:-http://localhost:8080}}"
API_KEY="${2:-${M_API_KEY:-test-api-key}}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0

# Helper functions
pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

info() {
    echo -e "${YELLOW}→${NC} $1"
}

# Test a curl request
# Usage: test_endpoint "description" expected_status curl_args...
test_endpoint() {
    local desc="$1"
    local expected="$2"
    shift 2

    local response
    local status

    response=$(curl -s -w "\n%{http_code}" "$@" 2>/dev/null) || true
    status=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$status" = "$expected" ]; then
        pass "$desc (HTTP $status)"
        echo "$body"
        return 0
    else
        fail "$desc (expected $expected, got $status)"
        echo "$body"
        return 1
    fi
}

echo "================================"
echo "M API Smoke Test"
echo "================================"
echo "Base URL: $BASE_URL"
echo "API Key:  ${API_KEY:0:8}..."
echo ""

# Track created resources for cleanup
REPO_ID=""
RUN_ID=""

cleanup() {
    if [ -n "$REPO_ID" ]; then
        info "Cleaning up test repo..."
        curl -s -X DELETE "$BASE_URL/api/repos/$REPO_ID" \
            -H "Authorization: Bearer $API_KEY" > /dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

# ========================================
# Test 1: Health Check (no auth required)
# ========================================
echo "--- Health Check ---"
test_endpoint "GET /health" 200 "$BASE_URL/health" || true
echo ""

# ========================================
# Test 2: Auth Required (should fail without auth)
# ========================================
echo "--- Auth Validation ---"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/repos" 2>/dev/null) || true
status=$(echo "$response" | tail -n1)
if [ "$status" = "401" ]; then
    pass "Unauthenticated request rejected (HTTP 401)"
else
    fail "Unauthenticated request should return 401, got $status"
fi
echo ""

# ========================================
# Test 3: GET /api/repos
# ========================================
echo "--- List Repos ---"
test_endpoint "GET /api/repos" 200 \
    "$BASE_URL/api/repos" \
    -H "Authorization: Bearer $API_KEY" || true
echo ""

# ========================================
# Test 4: POST /api/repos (create test repo)
# ========================================
echo "--- Create Test Repo ---"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/repos" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"name":"smoke-test-repo"}' 2>/dev/null) || true
status=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$status" = "201" ]; then
    pass "POST /api/repos (HTTP $status)"
    REPO_ID=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "Created repo: $REPO_ID"
else
    fail "POST /api/repos (expected 201, got $status)"
    echo "$body"
fi
echo ""

# ========================================
# Test 5: GET /api/repos/{id}
# ========================================
if [ -n "$REPO_ID" ]; then
    echo "--- Get Repo by ID ---"
    test_endpoint "GET /api/repos/$REPO_ID" 200 \
        "$BASE_URL/api/repos/$REPO_ID" \
        -H "Authorization: Bearer $API_KEY" || true
    echo ""
fi

# ========================================
# Test 6: POST /api/repos/{repo_id}/runs
# ========================================
if [ -n "$REPO_ID" ]; then
    echo "--- Create Test Run ---"
    response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/repos/$REPO_ID/runs" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{"prompt":"Smoke test run"}' 2>/dev/null) || true
    status=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$status" = "201" ]; then
        pass "POST /api/repos/{id}/runs (HTTP $status)"
        RUN_ID=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        echo "Created run: $RUN_ID"
    else
        fail "POST /api/repos/{id}/runs (expected 201, got $status)"
        echo "$body"
    fi
    echo ""
fi

# ========================================
# Test 7: GET /api/runs/{id}
# ========================================
if [ -n "$RUN_ID" ]; then
    echo "--- Get Run by ID ---"
    test_endpoint "GET /api/runs/$RUN_ID" 200 \
        "$BASE_URL/api/runs/$RUN_ID" \
        -H "Authorization: Bearer $API_KEY" || true
    echo ""
fi

# ========================================
# Test 8: WebSocket Connection
# ========================================
if [ -n "$RUN_ID" ]; then
    echo "--- WebSocket Connection Test ---"
    WS_URL="${BASE_URL/http/ws}/api/runs/$RUN_ID/events"

    # Check if websocat or wscat is available
    if command -v websocat &> /dev/null; then
        info "Testing WebSocket with websocat..."
        # Try to connect and read one message with timeout
        timeout 3 websocat -t --header "Authorization: Bearer $API_KEY" "$WS_URL" 2>/dev/null && \
            pass "WebSocket connection successful" || \
            fail "WebSocket connection failed (this may be expected if server doesn't support WS auth headers)"
    elif command -v wscat &> /dev/null; then
        info "Testing WebSocket with wscat..."
        # wscat doesn't support custom headers easily, skip
        info "wscat doesn't support custom auth headers - skipping"
    else
        info "No WebSocket client found (install websocat or wscat for WS testing)"
        info "WebSocket endpoint: $WS_URL"
    fi
    echo ""
fi

# ========================================
# Test 9: List Approvals
# ========================================
echo "--- List Approvals ---"
test_endpoint "GET /api/approvals" 200 \
    "$BASE_URL/api/approvals" \
    -H "Authorization: Bearer $API_KEY" || true
echo ""

# ========================================
# Test 10: List Pending Approvals
# ========================================
echo "--- List Pending Approvals ---"
test_endpoint "GET /api/approvals/pending" 200 \
    "$BASE_URL/api/approvals/pending" \
    -H "Authorization: Bearer $API_KEY" || true
echo ""

# ========================================
# Summary
# ========================================
echo "================================"
echo "Summary"
echo "================================"
echo -e "${GREEN}Passed:${NC} $PASSED"
echo -e "${RED}Failed:${NC} $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
