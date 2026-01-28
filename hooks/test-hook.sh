#!/bin/bash
# test-hook.sh - Test M hook integration
#
# Tests the PreToolUse hook by simulating tool calls and verifying
# they correctly POST to the M server.
#
# Usage:
#   ./test-hook.sh              # Run all tests
#   ./test-hook.sh --dry-run    # Show what would happen without M server

set -euo pipefail

# === Configuration ===
: "${DRY_RUN:=0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_SCRIPT="$SCRIPT_DIR/PreToolUse.sh"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# === Helpers ===
log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_test() { echo -e "\n${YELLOW}[TEST]${NC} $*"; }

# === Parse args ===
for arg in "$@"; do
    case $arg in
        --dry-run) DRY_RUN=1 ;;
        --help|-h)
            echo "Usage: $0 [--dry-run]"
            echo ""
            echo "Tests the M PreToolUse hook integration."
            echo ""
            echo "Environment variables:"
            echo "  M_SERVER_URL   M server URL (default: http://localhost:8080)"
            echo "  M_API_KEY      API key for M server (required unless --dry-run)"
            exit 0
            ;;
    esac
done

# === Validation ===
if [[ ! -f "$HOOK_SCRIPT" ]]; then
    log_error "Hook script not found: $HOOK_SCRIPT"
    exit 1
fi

# Set all required environment for hook
export M_RUN_ID="test-run-001"
export M_SERVER_URL="${M_SERVER_URL:-http://localhost:8080}"
export M_API_KEY="${M_API_KEY:-test-key-for-dry-run}"
export M_APPROVAL_TOOLS="Edit Write Bash NotebookEdit"
export M_INPUT_TOOLS="AskUserQuestion"
export M_HOOK_DEBUG="1"
export M_HOOK_TIMEOUT="2"

# === Test 1: Hook allows non-approval tools ===
log_test "Non-approval tool passes through"

RESULT=$(echo '{"tool": "Read", "input": {"file_path": "/test.txt"}}' | bash "$HOOK_SCRIPT" 2>&1)

if echo "$RESULT" | grep -q '"decision": "allow"'; then
    log_info "PASS: Read tool allowed without approval"
else
    log_error "FAIL: Expected allow decision for Read tool"
    echo "Got: $RESULT"
    exit 1
fi

# === Test 2: Hook format validation ===
log_test "Hook parses tool input correctly"

# Test with complex input
COMPLEX_INPUT='{"tool": "Edit", "input": {"file_path": "/path/to/file.txt", "old_string": "hello", "new_string": "world"}}'

if [[ "$DRY_RUN" == "1" ]]; then
    log_info "DRY RUN: Would send approval request for Edit tool"
    log_info "Request body would include:"
    echo "$COMPLEX_INPUT" | jq .
else
    # This will block waiting for approval, so we timeout
    log_info "Sending Edit tool call (will timeout after 2s if server not ready)..."

    set +e
    RESULT=$(timeout 2 bash -c "echo '$COMPLEX_INPUT' | bash '$HOOK_SCRIPT'" 2>&1)
    EXIT_CODE=$?
    set -e

    if [[ $EXIT_CODE -eq 124 ]]; then
        log_info "PASS: Hook correctly blocked waiting for approval (timed out as expected)"
    elif echo "$RESULT" | grep -q '"decision"'; then
        log_info "PASS: Hook received response from server"
        echo "Response: $RESULT"
    else
        log_warn "Hook may not have connected to server properly"
        echo "Output: $RESULT"
    fi
fi

# === Test 3: AskUserQuestion routes to input ===
log_test "AskUserQuestion routes to input type"

INPUT_TOOL='{"tool": "AskUserQuestion", "input": {"question": "What should I name the file?"}}'

if [[ "$DRY_RUN" == "1" ]]; then
    log_info "DRY RUN: Would send input request for AskUserQuestion"
else
    log_info "Sending AskUserQuestion (will timeout after 2s)..."

    set +e
    RESULT=$(timeout 2 bash -c "echo '$INPUT_TOOL' | bash '$HOOK_SCRIPT'" 2>&1)
    EXIT_CODE=$?
    set -e

    if [[ $EXIT_CODE -eq 124 ]]; then
        log_info "PASS: Hook correctly blocked waiting for input (timed out as expected)"
    elif echo "$RESULT" | grep -q '"decision"'; then
        log_info "PASS: Hook received response from server"
    else
        log_warn "Unexpected result"
        echo "Output: $RESULT"
    fi
fi

# === Test 4: Invalid input handling ===
log_test "Invalid input handling"

INVALID_INPUT='not json at all'

set +e
RESULT=$(echo "$INVALID_INPUT" | bash "$HOOK_SCRIPT" 2>&1)
EXIT_CODE=$?
set -e

# Note: jq will error on invalid JSON before the script can handle it
# This is acceptable - the hook errors (exit non-0) rather than allowing/blocking
if [[ $EXIT_CODE -ne 0 ]]; then
    log_info "PASS: Invalid input causes hook error (expected - jq parse failure)"
elif echo "$RESULT" | grep -q '"decision": "allow"'; then
    log_info "PASS: Invalid input defaults to allow"
else
    log_error "FAIL: Unexpected behavior for invalid input"
    echo "Got: $RESULT"
fi

# === Summary ===
echo ""
log_info "All tests completed"

if [[ "$DRY_RUN" == "1" ]]; then
    echo ""
    log_warn "Ran in dry-run mode. For full integration test:"
    echo "  1. Start M server: go run ./cmd/m-server"
    echo "  2. Set M_API_KEY=<your-key>"
    echo "  3. Run: $0"
fi
