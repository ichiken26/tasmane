#!/usr/bin/env bash
set -euo pipefail

# 自宅サーバー .env と Worker Secrets（secrets/worker.env）向け乱数シークレットの生成
binding="$(openssl rand -base64 48)"
echo "# Generated secrets — do not commit."
echo
echo "# --- home server .env ---"
echo "TASMANE_INTERNAL_BINDING_SECRET=$binding"
echo "TASMANE_IMPACT_TOKEN_SECRET=$(openssl rand -base64 48)"
echo "TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY=$(openssl rand -base64 32)"
echo
echo "# --- secrets/worker.env (cf deploy --secrets-file) ---"
echo "NUXT_INTERNAL_BINDING_SECRET=$binding"
echo "NUXT_SESSION_SECRET=$(openssl rand -base64 48)"
