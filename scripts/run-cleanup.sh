#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
[[ -f .env ]] || { echo ".env is required" >&2; exit 1; }

docker compose -f docker-compose.prod.yml --profile ops run --rm cleanup
