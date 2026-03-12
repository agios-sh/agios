#!/usr/bin/env bash
set -euo pipefail

# Compute changed Go files between base and head
BASE_SHA="${AIRLOCK_BASE_SHA:-HEAD~1}"
HEAD_SHA="${AIRLOCK_HEAD_SHA:-HEAD}"

CHANGED_GO_FILES=$(git diff --name-only --diff-filter=ACMR "$BASE_SHA" "$HEAD_SHA" -- '*.go' | grep -v '_test.go$' || true)
CHANGED_GO_TEST_FILES=$(git diff --name-only --diff-filter=ACMR "$BASE_SHA" "$HEAD_SHA" -- '*.go' || true)
ALL_CHANGED_GO="$CHANGED_GO_TEST_FILES"

if [ -z "$ALL_CHANGED_GO" ]; then
  echo "No Go files changed — nothing to lint."
  exit 0
fi

# Collect unique packages from changed files
PACKAGES=$(echo "$ALL_CHANGED_GO" | xargs -I{} dirname {} | sort -u | sed 's|^|./|')

echo "==> Changed Go files:"
echo "$ALL_CHANGED_GO"
echo ""

# Step 1: Auto-fix formatting
echo "==> Running gofmt -w (auto-fix)..."
echo "$ALL_CHANGED_GO" | xargs gofmt -w
echo "    Done."

# Step 2: Run go vet on affected packages
echo "==> Running go vet on changed packages..."
echo "$PACKAGES" | xargs go vet
echo "    Done."

# Step 3: Verify formatting is clean
echo "==> Verifying formatting..."
UNFORMATTED=$(echo "$ALL_CHANGED_GO" | xargs gofmt -l || true)
if [ -n "$UNFORMATTED" ]; then
  echo "ERROR: Files still not formatted after auto-fix:"
  echo "$UNFORMATTED"
  exit 1
fi
echo "    All files formatted correctly."

echo ""
echo "==> All lint checks passed."
