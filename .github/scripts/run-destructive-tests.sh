#!/bin/bash
set -e

# =============================================================================
# TW Destructive Command Test Runner
# =============================================================================
# Runs all test files under .github/scripts/tests/
# Sources shared helpers and aggregates results.
#
# Usage: .github/scripts/run-destructive-tests.sh [test-file...]
#   With no arguments, runs all test-*.sh files in .github/scripts/tests/
#   With arguments, runs only the specified test files.
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Source shared test helpers (variables + functions)
. "$SCRIPT_DIR/test-helpers.sh"

if [ $# -gt 0 ]; then
  # Run only specified test files
  for testfile in "$@"; do
    if [ -f "$testfile" ]; then
      . "$testfile"
    else
      echo "ERROR: test file not found: $testfile"
      exit 1
    fi
  done
else
  # Run all test files
  for testfile in "$SCRIPT_DIR"/tests/test-*.sh; do
    . "$testfile"
  done
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "==========================================="
echo "  Results: $PASS/$TOTAL passed, $FAIL failed"
echo "==========================================="

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "--- FAILURE REPORT (regression guide) ---"
  echo ""
  printf "%-12s %-30s %-12s %s\n" "TYPE" "CATEGORY" "EXIT CODES" "LABEL"
  printf "%-12s %-30s %-12s %s\n" "----" "--------" "----------" "-----"
  while IFS='|' read -r type category label exitinfo cmd; do
    type="${type#"${type%%[![:space:]]*}"}"
    type="${type%"${type##*[![:space:]]}"}"
    category="${category#"${category%%[![:space:]]*}"}"
    category="${category%"${category##*[![:space:]]}"}"
    label="${label#"${label%%[![:space:]]*}"}"
    label="${label%"${label##*[![:space:]]}"}"
    exitinfo="${exitinfo#"${exitinfo%%[![:space:]]*}"}"
    exitinfo="${exitinfo%"${exitinfo##*[![:space:]]}"}"
    printf "%-12s %-30s %-12s %s\n" "$type" "$category" "$exitinfo" "$label"
  done < "$FAILURE_REPORT"
  echo ""
  echo "Full failure details: $FAILURE_REPORT"
  echo ""
  echo "Each line in the report follows the format:"
  echo "  TYPE | CATEGORY | LABEL | expected=N got=N | COMMAND"
  echo ""
  echo "FAILED: $FAIL test(s) did not pass."
  exit 1
fi

echo "All destructive command tests passed!"
exit 0
