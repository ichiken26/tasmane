#!/usr/bin/env bash
# cf 用ビルド: Nuxt(Nitro) を組み立て、cf deploy --prebuilt が読む Build Output(.cloudflare/output/v0) を生成する
# cf (beta) は Nuxt の Build Output を直接生成しないため、Wrangler bundler の Build Output 生成を利用する
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
npx nuxt build
npx wrangler build --experimental-new-config --experimental-cf-build-output
test -f .cloudflare/output/v0/config.json || { echo "Build Output was not generated" >&2; exit 1; }
