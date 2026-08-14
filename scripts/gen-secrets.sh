#!/usr/bin/env bash
set -euo pipefail

echo "# Generated secrets — paste into Cloudflare Secrets / .env. Do not commit."
echo
echo "TASMANE_INTERNAL_BINDING_SECRET=$(openssl rand -base64 48)"
echo "NUXT_INTERNAL_BINDING_SECRET=<same value as TASMANE_INTERNAL_BINDING_SECRET>"
echo "TASMANE_IMPACT_TOKEN_SECRET=$(openssl rand -base64 48)"
echo "NUXT_SESSION_SECRET=$(openssl rand -base64 48)"
echo "TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY=$(openssl rand -base64 32)"
