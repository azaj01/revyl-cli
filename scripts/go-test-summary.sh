#!/usr/bin/env bash

set -uo pipefail

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

if [ -n "${REVYL_GO_TEST_LOG:-}" ]; then
  LOG_FILE="$REVYL_GO_TEST_LOG"
else
  TMP_BASE="${TMPDIR:-/tmp}"
  LOG_FILE="$(mktemp "${TMP_BASE%/}/revyl-go-test-XXXXXX")"
fi
KEEP_LOG="${REVYL_GO_TEST_KEEP_LOG:-0}"

printf 'Running: go test'
printf ' %q' "$@"
printf '\n'

go test "$@" 2>&1 | tee "$LOG_FILE"
TEST_EXIT=${PIPESTATUS[0]}

emit_failure_summary() {
  printf '%s\n' "$*"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '%s\n' "$*" >> "$GITHUB_STEP_SUMMARY"
  fi
}

if [ "$TEST_EXIT" -eq 0 ]; then
  if [ "$KEEP_LOG" != "1" ] && [ -f "$LOG_FILE" ]; then
    rm -f "$LOG_FILE"
  fi
  echo ""
  echo "Go test summary: all packages passed."
  exit 0
fi

emit_failure_summary ""
emit_failure_summary "### Go test failure summary"

FAILED_PACKAGES="$(awk '/^FAIL[[:space:]]/ && $0 != "FAIL" { print }' "$LOG_FILE" | sort -u || true)"
FAILED_TESTS="$(awk '/^--- FAIL: / { print }' "$LOG_FILE" | sort -u || true)"
BUILD_CONTEXT="$(awk '/^# / { print }' "$LOG_FILE" | sort -u || true)"

PRINTED=0

if [ -n "$FAILED_PACKAGES" ]; then
  emit_failure_summary ""
  emit_failure_summary "Failed package lines:"
  emit_failure_summary '```'
  emit_failure_summary "$FAILED_PACKAGES"
  emit_failure_summary '```'
  PRINTED=1
fi

if [ -n "$FAILED_TESTS" ]; then
  emit_failure_summary ""
  emit_failure_summary "Failed test lines:"
  emit_failure_summary '```'
  emit_failure_summary "$FAILED_TESTS"
  emit_failure_summary '```'
  PRINTED=1
fi

if [ -n "$BUILD_CONTEXT" ]; then
  emit_failure_summary ""
  emit_failure_summary "Build/setup context:"
  emit_failure_summary '```'
  emit_failure_summary "$BUILD_CONTEXT"
  emit_failure_summary '```'
  PRINTED=1
fi

if [ "$PRINTED" -eq 0 ]; then
  emit_failure_summary ""
  emit_failure_summary "No explicit FAIL/test marker was found. Last 80 log lines:"
  emit_failure_summary '```'
  emit_failure_summary "$(tail -n 80 "$LOG_FILE")"
  emit_failure_summary '```'
fi

emit_failure_summary ""
emit_failure_summary "Full go test log captured at: $LOG_FILE"
exit "$TEST_EXIT"
