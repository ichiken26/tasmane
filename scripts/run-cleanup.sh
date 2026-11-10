#!/usr/bin/env bash
set -euo pipefail

# リポジトリルートへの移動
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# docker と .env の前提確認
command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
[[ -f .env ]] || { echo ".env is required" >&2; exit 1; }

# ops プロファイルの cleanup コンテナ実行
docker compose -f docker-compose.prod.yml --profile ops run --rm cleanup
