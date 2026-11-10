#!/usr/bin/env bash
# 自宅サーバー上の Go スタック（migrate / api / cloudflared / workers）の更新・起動
set -euo pipefail

# リポジトリルートへの移動
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# docker と .env の前提確認
command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
[[ -f .env ]] || { echo ".env is required (copy from .env.example)" >&2; exit 1; }

# ビルドと起動（migrate 完了後に api / cloudflared / workers）
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d --remove-orphans migrate api cloudflared google-worker advice-scheduler
docker compose -f docker-compose.prod.yml ps

# ループバックでのヘルスチェック（Tunnel 越しの疎通は Worker 側で確認）
host_port="$(sed -n 's/^TASMANE_API_HOST_PORT=//p' .env | tail -n 1 | tr -d '"[:space:]')"
host_port="${host_port:-8080}"
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${host_port}/readyz" >/dev/null 2>&1; then
    echo "Go API is ready (127.0.0.1:${host_port}). cloudflared logs: docker compose -f docker-compose.prod.yml logs -f cloudflared"
    exit 0
  fi
  sleep 2
done
echo "Go API did not become ready; check: docker compose -f docker-compose.prod.yml logs api" >&2
exit 1
