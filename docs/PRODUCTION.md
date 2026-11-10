# 本番リリース手順（要約）

詳細な手順は [`docs/OPERATIONS.md`](OPERATIONS.md)、現行仕様は [`docs/SYSTEM_SPEC.md`](SYSTEM_SPEC.md) を参照。

## アーキテクチャ（本番）

```text
Browser
  → Cloudflare Worker「tasmane-web」（Nuxt、唯一の公開入口。cf でデプロイ）
      → Workers VPC Service「TASMANE_API」→ Cloudflare Tunnel（cloudflared）
          → 自宅サーバー Docker Compose: Go API（非公開）
Go API / google-worker / advice-scheduler / cleanup → Neon PostgreSQL
google-worker → Google Tasks API ／ advice-scheduler → Gemini API
```

- Google 同期キューは PostgreSQL durable queue（タスク更新と同一トランザクションの outbox）。
- 公開 API パスは `/api/v1/**` のみ。

## 手順チェックリスト

1. Neon 作成 → `DATABASE_URL`
2. Google OAuth クライアント作成（リダイレクト `https://tasmane.kokage-studio.com/auth/callback`）
3. `bash scripts/gen-secrets.sh` → 自宅サーバー `.env` と `secrets/worker.env`
4. `npm run setup:cloudflare`（`cf` で Tunnel・トークン・VPC Service 作成）
5. 自宅サーバー: `.env` に `TASMANE_TUNNEL_TOKEN` 等を記入 → `npm run deploy:go`
6. 作業PC: `TASMANE_API_VPC_SERVICE_ID=… npm run deploy:worker`
7. スモークテスト（`docs/OPERATIONS.md` §2）
8. cleanup の cron 設定
9. GitHub Environment `production` の Secrets 登録（自動デプロイを使う場合）

## 切り戻し

- Worker: 前のバージョンへ（ダッシュボードまたは `cf workers deployments create`）
- Go: 前のコミットで `npm run deploy:go`
- DB: migration は追加のみ（前進修正）
