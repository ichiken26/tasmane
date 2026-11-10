#!/usr/bin/env bash
# Nuxt Worker（tasmane-web）の cf によるビルドとデプロイ
set -euo pipefail

# リポジトリルートへの移動
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# 前提確認（cf は Node.js 22.23 以上が必要）
command -v npx >/dev/null 2>&1 || { echo "npx is required" >&2; exit 1; }
node -e 'const [a,b]=process.versions.node.split(".").map(Number);process.exit(a>22||(a===22&&b>=23)?0:1)' \
  || { echo "Node.js 22.23+ is required by cf (found $(node --version))" >&2; exit 1; }
[[ -d node_modules ]] || npm ci
: "${TASMANE_API_VPC_SERVICE_ID:?set TASMANE_API_VPC_SERVICE_ID (cf workers-vpc services list)}"
secrets_file="${TASMANE_WORKER_SECRETS_FILE:-secrets/worker.env}"
[[ -f "$secrets_file" ]] || { echo "$secrets_file is required (see docs/OPERATIONS.md)" >&2; exit 1; }

# Nuxt ビルド → Build Output 生成 → cf でデプロイ（Worker Secrets は同じバージョンとして投入）
bash scripts/cf-build.sh
npx cf deploy --prebuilt --secrets-file "$secrets_file" "$@"
echo "Deployed tasmane-web. Smoke test: https://${TASMANE_PUBLIC_HOSTNAME:-tasmane.kokage-studio.com}/"
