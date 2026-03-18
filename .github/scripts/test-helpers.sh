# =============================================================================
# TW Shared Test Helpers
# =============================================================================
# Common test infrastructure for TW CI test scripts.
# Source this file; do not execute directly.
#
# Provides:
#   - Counters: PASS, FAIL, TOTAL, CATEGORY
#   - FAILURE_REPORT file path (env-overridable via TW_FAILURE_REPORT)
#   - record_failure()    — append a structured failure line to the report
#   - expect_deny()       — assert `tw test` exits 1 (denied)
#   - expect_allow()      — assert `tw test` exits 0 (allowed)
#   - expect_hook_deny()  — assert `tw hook` exits 1 (denied)
#   - expect_hook_allow() — assert `tw hook` exits 0 (allowed)
#   - section()           — print a section header and set CATEGORY
# =============================================================================

PASS=0
FAIL=0
TOTAL=0
CATEGORY=""
FAILURE_REPORT="${TW_FAILURE_REPORT:-/tmp/tw-destructive-failures.txt}"

: > "$FAILURE_REPORT"

record_failure() {
  # $1=type (DENY|ALLOW), $2=label, $3=command, $4=expected_exit, $5=actual_exit, $6=category
  printf "%s | %s | %s | expected=%s got=%s | %s\n" \
    "$1" "$6" "$2" "$4" "$5" "$3" >> "$FAILURE_REPORT"
}

# Expect a command to be DENIED (exit 1)
expect_deny() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(tw test "$1" 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 1 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [DENY]:  %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [DENY]:  %s (expected exit 1, got %s)\n" "$2" "$EXIT"
    printf "                cmd: %s\n" "$1"
    record_failure "DENY" "$2" "$1" "1" "$EXIT" "$CATEGORY"
  fi
}

# Expect a command to be ALLOWED (exit 0)
expect_allow() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(tw test "$1" 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 0 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [ALLOW]: %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [ALLOW]: %s (expected exit 0, got %s)\n" "$2" "$EXIT"
    printf "                cmd: %s\n" "$1"
    record_failure "ALLOW" "$2" "$1" "0" "$EXIT" "$CATEGORY"
  fi
}

# Expect a command to be DENIED via hook protocol (exit 1)
expect_hook_deny() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(echo "$1" | tw hook 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 1 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [DENY]:  %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [DENY]:  %s (expected exit 1, got %s)\n" "$2" "$EXIT"
    record_failure "HOOK_DENY" "$2" "$1" "1" "$EXIT" "$CATEGORY"
  fi
}

# Expect a command to be ALLOWED via hook protocol (exit 0)
expect_hook_allow() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(echo "$1" | tw hook 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 0 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [ALLOW]: %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [ALLOW]: %s (expected exit 0, got %s)\n" "$2" "$EXIT"
    record_failure "HOOK_ALLOW" "$2" "$1" "0" "$EXIT" "$CATEGORY"
  fi
}

section() {
  CATEGORY="$1"
  echo ""
  echo "=== $1 ==="
}
