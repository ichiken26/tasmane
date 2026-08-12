#!/usr/bin/env bash
set -euo pipefail

echo "[verify] starting"

[[ -f go.mod ]] || { echo "go.mod is required"; exit 1; }
if [[ -n "${TASMANE_GO_BIN:-}" ]]; then
  GO_BIN="$TASMANE_GO_BIN"
elif command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
else
  echo "Go toolchain is required (set TASMANE_GO_BIN to a Go 1.22+ executable)"
  exit 1
fi
[[ -x "$GO_BIN" ]] || { echo "TASMANE_GO_BIN is not executable: $GO_BIN"; exit 1; }
GO_VERSION="$($GO_BIN env GOVERSION)"
GOFMT_BIN="$(dirname "$GO_BIN")/gofmt"
[[ -x "$GOFMT_BIN" ]] || { echo "gofmt not found beside Go executable"; exit 1; }
echo "[verify] $GO_VERSION"
case "$GO_VERSION" in go1.22.*|go1.2[3-9].*|go1.[3-9][0-9].*) ;; *) echo "Go 1.22 or newer is required"; exit 1;; esac
echo "[verify] formatter"
test -z "$("$GOFMT_BIN" -l cmd db internal | head -n 1)" || { echo "gofmt required"; exit 1; }
echo "[verify] lint/typecheck/static analysis"
GOFLAGS="${GOFLAGS:-} -buildvcs=false" "$GO_BIN" vet ./...
echo "[verify] test"
GOFLAGS="${GOFLAGS:-} -buildvcs=false" "$GO_BIN" test ./...
GOFLAGS="${GOFLAGS:-} -buildvcs=false" "$GO_BIN" test -race ./...
python3 -m unittest discover -s test -p '*_test.py'
echo "[verify] build"
VERIFY_BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$VERIFY_BUILD_DIR"' EXIT
GOFLAGS="${GOFLAGS:-} -buildvcs=false" "$GO_BIN" build ./...

[[ -f package.json ]] || { echo "package.json is required"; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "npm is required"; exit 1; }
[[ -d node_modules ]] || { echo "node_modules missing; run npm ci"; exit 1; }
for script in test lint typecheck build; do
  npm pkg get "scripts.$script" | grep -vq '^{}$' || { echo "npm script is required: $script"; exit 1; }
  npm run "$script"
done

if command -v spectral >/dev/null 2>&1; then
  spectral lint api/openapi.yaml
fi

python3 scripts/validate_contracts.py

echo "[verify] OK"
