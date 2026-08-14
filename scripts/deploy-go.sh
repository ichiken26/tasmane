#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
[[ -f .env ]] || { echo ".env is required (copy from .env.example)" >&2; exit 1; }

docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d migrate api google-worker advice-scheduler
docker compose -f docker-compose.prod.yml ps
echo "Private Go stack started on 127.0.0.1:8080. Point Cloudflare Tunnel/Binding at the API only."
