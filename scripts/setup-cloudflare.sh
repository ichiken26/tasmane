#!/usr/bin/env bash
# Cloudflare 側の初回セットアップ（cf を使用）
#  1. リモート管理の Cloudflare Tunnel を作成
#  2. cloudflared 用トークンを取得（自宅サーバーの .env の TASMANE_TUNNEL_TOKEN へ）
#  3. Tunnel 先 Go API の Workers VPC Service を作成（Worker の TASMANE_API バインディング先）
# 前提: `npx cf auth login` 済み、または CLOUDFLARE_API_TOKEN / CLOUDFLARE_ACCOUNT_ID を設定済み
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
export CF_SEND_TELEMETRY=false

tunnel_name="${TASMANE_TUNNEL_NAME:-tasmane-home}"
service_name="${TASMANE_VPC_SERVICE_NAME:-tasmane-api}"
api_ip="${TASMANE_API_TUNNEL_IP:-172.29.80.10}"

# 1. Tunnel 作成（既存なら TASMANE_TUNNEL_ID を指定して再利用）
if [[ -z "${TASMANE_TUNNEL_ID:-}" ]]; then
  echo "Creating Cloudflare Tunnel: $tunnel_name"
  npx cf tunnels create --name "$tunnel_name" --config-source cloudflare
  echo "Copy the tunnel id above and re-run with TASMANE_TUNNEL_ID=<id>"
  exit 0
fi

# 2. cloudflared トークン（秘密情報。画面共有・ログ保存に注意）
mkdir -p secrets
umask 077
# cf は JSON 文字列（引用符付き）で出力するため、トークン本体だけを保存
npx cf tunnels token get "$TASMANE_TUNNEL_ID" | tr -d '"[:space:]' > secrets/tunnel-token.txt
echo "Tunnel token saved to secrets/tunnel-token.txt -> set TASMANE_TUNNEL_TOKEN in the home server .env"

# 3. Workers VPC Service（Tunnel 経由で api の固定 IP:8080 へ。HTTP のみ・公開ホスト名なし）
npx cf workers-vpc services create \
  --name "$service_name" \
  --type http \
  --host-ipv4 "$api_ip" \
  --host-network-tunnel-id "$TASMANE_TUNNEL_ID" \
  --http-port 8080
echo "Set TASMANE_API_VPC_SERVICE_ID to the service id above (deploy env / GitHub secret)."
