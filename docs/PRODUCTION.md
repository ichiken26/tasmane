# 本番リリース手順

この文書は `tasmane.kokage-studio.com` へ上げるまでの全ステップ。  
コード側で用意済みのもの → Docker / Worker deploy script / healthz / `/api/v1` / CI / `.env.example`。  
人手必須のもの → アカウント・DNS・Secret実値・OAuth・Neon。

## アーキテクチャ（本番）

```text
Browser
  → Cloudflare Worker (Nuxt, 唯一の公開入口)
      → Service Binding / Tunnel → Go API (非公開)
Go API / google-worker / advice-scheduler
  → Neon PostgreSQL
google-worker → Google Tasks API
advice-scheduler → Gemini API
```

補足:
- Google同期キューは現状 **PostgreSQL durable queue**（`FOR UPDATE SKIP LOCKED`）。SPECの Cloudflare Queues とは実装が異なるが、Worker非公開・永続lease・retry は満たす。
- 公開APIパスは SPEC どおり **`/api/v1/**`**。互換のため `/v1/**` も残す。

## A. 人手で先に用意するもの

1. **Neon**
   - Production プロジェクト作成
   - `DATABASE_URL`（`sslmode=require`）取得
   - 可能なら staging DB も分離

2. **Cloudflare**
   - アカウント / Workers 有効化
   - カスタムドメイン `tasmane.kokage-studio.com` を Worker へ接続
   - Go API への到達経路をどちらかで用意
     - **推奨A**: 非公開ホスト + Cloudflare Tunnel（`127.0.0.1:8080` のみ）
     - **推奨B**: Cloudflare Containers（別途 Container DO 配線。本リポは Docker image を提供）

3. **Google Cloud OAuth**
   - Web クライアント作成
   - Authorized redirect URI: `https://tasmane.kokage-studio.com/auth/callback`
   - Scope: OpenID / email / profile + Google Tasks
   - Client ID/Secret を Worker と google-worker の両方で使う（同一クライアントで可）

4. **Gemini**
   - API key 発行
   - モデル決定（既定例: `gemini-2.0-flash`）

5. **Secret 生成**
   ```bash
   bash scripts/gen-secrets.sh
   ```
   - `TASMANE_INTERNAL_BINDING_SECRET` と `NUXT_INTERNAL_BINDING_SECRET` は **同一値**
   - `TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY` は **32 raw bytes の標準Base64**（API必須）

## B. リポジトリ側（実装済みコマンド）

### 1. 環境ファイル

```bash
cp .env.example .env
# .env を本番値で埋める
```

### 2. 検証

```bash
export PATH="$TASMANE_GO_ROOT/bin:$PATH"
export TASMANE_GO_BIN="$(command -v go)"
export GOFLAGS=-buildvcs=false
bash scripts/bootstrap.sh
bash scripts/verify.sh
```

CI: `.github/workflows/verify.yml` が PR/push で同じゲートを実行。

### 3. Go 私有スタック起動

```bash
bash scripts/deploy-go.sh
# API health
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/readyz
```

常駐プロセス:
| プロセス | 役割 |
|---|---|
| `api` | HTTP API + migrate-on-boot + auth bootstrap |
| `google-worker` | Google Tasks 同期 consumer |
| `advice-scheduler` | 各ユーザーTZの 04:00 以降に Advice 生成 |
| `cleanup`（ops profile） | soft-delete 30日後の hard delete（cron） |

日次 cleanup 例（ホスト cron）:

```cron
15 3 * * * cd /path/to/tasmane && bash scripts/run-cleanup.sh >>/var/log/tasmane-cleanup.log 2>&1
```

### 4. Worker Secrets

```bash
npx wrangler secret put NUXT_GOOGLE_CLIENT_ID
npx wrangler secret put NUXT_GOOGLE_CLIENT_SECRET
npx wrangler secret put NUXT_GOOGLE_REDIRECT_URI
npx wrangler secret put NUXT_SESSION_SECRET
npx wrangler secret put NUXT_INTERNAL_BINDING_SECRET
```

### 5. Worker デプロイ

```bash
bash scripts/deploy-worker.sh
```

確認:
- `wrangler.jsonc` の Service Binding `TASMANE_API` → `tasmane-api`（または Tunnel 経由の内部 Worker）が本番で解決すること
- Custom Domain が Worker に付いていること
- Browser から直接 Go ポートへ到達できないこと

### 6. 公開後スモーク

1. `https://tasmane.kokage-studio.com/` 表示
2. `/auth/login` → Google → callback → session
3. Genre/Task 作成
4. `/settings` で timezone / sync interval 保存
5. Google sync 手動実行
6. Dashboard に Advice（翌日04:00以降でも可）

## C. Go 側 Secrets（Container/ホスト）

必須:
- `DATABASE_URL`
- `TASMANE_INTERNAL_BINDING_SECRET`
- `TASMANE_IMPACT_TOKEN_SECRET`
- `TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY`
- `GOOGLE_OAUTH_CLIENT_ID` / `GOOGLE_OAUTH_CLIENT_SECRET`（worker）
- `GEMINI_API_KEY` / `GEMINI_MODEL`（scheduler）
- `TASMANE_HTTP_ADDR=0.0.0.0:8080`（container 内）

## D. ロールアウト順（推奨）

1. Neon 作成 → migrate（`deploy-go` の migrate service または `cmd/migrate`）
2. Go API + workers 起動（非公開）
3. Tunnel/Binding 接続確認（`/readyz`）
4. Worker secrets 投入
5. Worker deploy + DNS
6. OAuth redirect 動作確認
7. cleanup cron 設置
8. 監視（Worker observability は `wrangler.jsonc` で有効）

## E. 切り戻し

- Worker: 直前の成功デプロイへ `wrangler rollback`（または前バージョン再deploy）
- Go: 前 image tag へ `docker compose ... up -d`
- DB: migration は **追加のみ**。破壊的巻き戻しは新規 migration で前進修復

## F. まだ人手/別作業が必要な項目

- Cloudflare 上での `tasmane-api` Service / Tunnel の実体作成と配線
- Cloudflare Containers を使う場合の Durable Object Container class 配線（本リポは image 提供まで）
- 本番監視アラート（5xx、readyz 失敗、queue backlog）
- MCP を本番公開するかの決定（`cmd/mcp` は別プロセス。不要なら起動しない）
- staging 環境の複製

## G. セキュリティ確認リスト

- [ ] Go API が Internet から直接到達できない
- [ ] Binding secret が Worker/Go で一致し、Browser に出ていない
- [ ] Session / Impact / Google token key がすべて独立
- [ ] OAuth redirect URI が本番 URL と完全一致
- [ ] `.env` / secrets が Git に含まれていない
- [ ] CSP / XFO / nosniff が応答に付いている
