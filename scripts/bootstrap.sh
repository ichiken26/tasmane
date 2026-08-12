#!/usr/bin/env bash
set -euo pipefail

mkdir -p state/logs

chmod +x scripts/*.sh

if [[ -n "${TASMANE_GO_BIN:-}" && -x "$TASMANE_GO_BIN" ]]; then
  :
elif command -v go >/dev/null 2>&1; then
  TASMANE_GO_BIN="$(command -v go)"
else
  echo "Go 1.22+ not found. Install it or export TASMANE_GO_BIN=/absolute/path/to/go." >&2
  exit 1
fi

GO_VERSION="$($TASMANE_GO_BIN env GOVERSION 2>/dev/null || true)"
case "$GO_VERSION" in
  go1.22.*|go1.2[3-9].*|go1.[3-9][0-9].*) ;;
  *) echo "Go 1.22 or newer is required (found: ${GO_VERSION:-unknown})." >&2; exit 1 ;;
esac

command -v npm >/dev/null 2>&1 || { echo "Node.js/npm is required." >&2; exit 1; }
npm ci

echo "Tasmane harness bootstrapped."
echo "Next:"
echo "  export TASMANE_GO_BIN='$TASMANE_GO_BIN'"
echo "  export PATH=\"$(dirname "$TASMANE_GO_BIN"):\$PATH\""
echo "  export GOFLAGS='-buildvcs=false'"
echo "  export AGENT_CMD='codex exec --full-auto -'"
echo "  npm run dev"
echo "  ./scripts/tdd-loop.sh"
