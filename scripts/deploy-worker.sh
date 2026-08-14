#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

command -v npx >/dev/null 2>&1 || { echo "npx is required" >&2; exit 1; }
[[ -d node_modules ]] || npm ci

npm run build
npx wrangler deploy "$@"
echo "Worker deploy requested. Confirm custom domain + Service Binding TASMANE_API -> tasmane-api."
