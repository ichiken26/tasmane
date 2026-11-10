# タスマネ 運用手順書

構成: **Nuxt = Cloudflare Workers**、**Go = 自宅サーバー（Docker Compose）**、両者を **Cloudflare Tunnel**（Workers VPC Service）で接続。Cloudflare の操作には **`cf` CLI** を使います。
仕様は `docs/SYSTEM_SPEC.md` を参照してください。

> 秘密情報（`.env`、`secrets/*`、Tunnel トークン）は Git に入れないこと。`.gitignore` 済みです。

---

## 0. 必要なもの

| 場所 | 必要なもの |
|---|---|
| 作業PC / CI | Node.js **22.23 以上**（`cf` の要件。CI は 24）、npm、Git |
| 自宅サーバー | Docker Engine + Compose v2、Git、curl。外向き HTTPS（443）と Cloudflare Tunnel（7844/UDP・TCP）への通信 |
| アカウント | Cloudflare（ドメイン `kokage-studio.com` を管理）、Neon、Google Cloud（OAuth クライアント）、Gemini API キー |

`cf` はリポジトリの devDependency（`npx cf …`）として入ります。初回は `npx cf auth login`（CI は `CLOUDFLARE_API_TOKEN` / `CLOUDFLARE_ACCOUNT_ID`）。

---

## 1. 初回セットアップ

### 1.1 Neon

1. Production プロジェクトを作成し、接続文字列（`sslmode=require`）を控える → 自宅サーバー `.env` の `DATABASE_URL`。
2. マイグレーションは Go 起動時に自動適用されます（`migrate` サービス）。

### 1.2 Google Cloud（OAuth）

1. OAuth クライアント（Web）を作成。
   - 同じ Google Cloud プロジェクトで **Google Tasks API を有効化**する（API とサービス → ライブラリ → Google Tasks API）。無効のままだと google-worker が `Google API 403 ... SERVICE_DISABLED` で失敗し、Google Tasks に何も追加されない。
2. 承認済みリダイレクト URI: `https://tasmane.kokage-studio.com/auth/callback`
3. スコープ: `openid` `email` `profile` `https://www.googleapis.com/auth/tasks`
4. Client ID / Secret を Worker（`secrets/worker.env`）と自宅サーバー（`.env` の `GOOGLE_OAUTH_*`）の両方に設定（同じクライアントで可）。

### 1.3 シークレット生成

```bash
bash scripts/gen-secrets.sh
```

出力の「home server .env」部分を自宅サーバーの `.env` へ、「secrets/worker.env」部分を作業PCの `secrets/worker.env` へ転記します。
`TASMANE_INTERNAL_BINDING_SECRET` と `NUXT_INTERNAL_BINDING_SECRET` は **同じ値** です。

### 1.4 Cloudflare Tunnel と Workers VPC Service（作業PCで `cf`）

```bash
npx cf auth login                       # 初回のみ
npm run setup:cloudflare                # Tunnel を作成（ID が表示される）
TASMANE_TUNNEL_ID=<表示された tunnel id> npm run setup:cloudflare
```

2 回目の実行で次を行います（`scripts/setup-cloudflare.sh`）。

- `cf tunnels token get` → `secrets/tunnel-token.txt`（自宅サーバー `.env` の `TASMANE_TUNNEL_TOKEN` へ転記。転記後は削除推奨）
- `cf workers-vpc services create --type http --host-ipv4 172.29.80.10 --host-network-tunnel-id <id> --http-port 8080`
  → 表示された **service id** を `TASMANE_API_VPC_SERVICE_ID` として控える（デプロイ時の環境変数・GitHub Secret）

Tunnel には **Public Hostname を作りません**（Go API はインターネットに出ない）。

### 1.5 自宅サーバー

```bash
git clone <repo> tasmane && cd tasmane
cp .env.example .env    # DATABASE_URL / 各シークレット / TASMANE_TUNNEL_TOKEN / GOOGLE_OAUTH_* / GEMINI_API_KEY を記入
chmod 600 .env
npm run deploy:go       # = bash scripts/deploy-go.sh（ビルド→migrate→起動→/readyz 確認）
```

確認:

```bash
curl -sS http://127.0.0.1:8080/readyz                         # ready
docker compose -f docker-compose.prod.yml logs -f cloudflared  # "Registered tunnel connection" が出ること
```

`TASMANE_TUNNEL_SUBNET` / `TASMANE_API_TUNNEL_IP` を変える場合は VPC Service の `--host-ipv4` も合わせて作り直します（`cf workers-vpc services update`）。

### 1.6 Worker（作業PC）

```bash
cp secrets/worker.env.example secrets/worker.env   # 値を記入（NUXT_ALLOWED_EMAILS は必ず自分のアドレス）
export TASMANE_API_VPC_SERVICE_ID=<1.4 の service id>
npm ci
npm run deploy:worker   # = scripts/cf-build.sh → cf deploy --prebuilt --secrets-file secrets/worker.env
```

カスタムドメイン `tasmane.kokage-studio.com` は `cloudflare.config.ts` の `domains` で付与されます（`workers.dev`・Preview URL は無効）。

### 1.6.1 メモアプリ連携（任意）

1. **Firebase**: メモアプリの Firebase プロジェクトの Authentication → Sign-in method → Google を開く。「外部プロジェクトのクライアント ID を許可リストに追加」に、タスマネの OAuth クライアント ID（`NUXT_GOOGLE_CLIENT_ID`）を追加する。これがないと、メモアプリへのログイン引き継ぎが `auth/invalid-credential` で失敗する。
2. **メモアプリ**（`memoAppNuxtTraining`）:
   - `NUXT_PUBLIC_EMBED_PARENT_ORIGINS` を確認する。既定は `https://tasmane.kokage-studio.com,http://localhost:3000,http://localhost:3100`。
   - 確認できたらデプロイする（`npm run deploy:workers`）。
3. **タスマネ**:
   - メモアプリのオリジンは `NUXT_PUBLIC_MEMO_APP_ORIGIN` で指定する（既定 `https://memo.kokage-studio.com`、ビルド時に埋め込まれる）。
   - `/settings` の「メモアプリと連携する」をオンにする。
4. **ローカルで試す場合**: メモアプリを別ポートで起動し、オリジンを合わせる。
   - メモアプリ: `NUXT_PUBLIC_EMBED_PARENT_ORIGINS=http://localhost:3000 npx nuxt dev --port 3201`
   - タスマネ: `NUXT_PUBLIC_MEMO_APP_ORIGIN=http://localhost:3201 npm run dev`

### 1.7 GitHub Actions（任意・推奨）

Repository → Settings → Environments に `production` を作り、Secrets を登録します。

| Secret | 値 |
|---|---|
| `CLOUDFLARE_API_TOKEN` | Workers Scripts:Edit・Workers VPC:Bind 等を持つ API トークン |
| `CLOUDFLARE_ACCOUNT_ID` | アカウント ID |
| `TASMANE_API_VPC_SERVICE_ID` | 1.4 の service id |
| `NUXT_GOOGLE_CLIENT_ID` / `NUXT_GOOGLE_CLIENT_SECRET` / `NUXT_GOOGLE_REDIRECT_URI` | OAuth |
| `NUXT_SESSION_SECRET` / `NUXT_INTERNAL_BINDING_SECRET` | 1.3 |
| `NUXT_ALLOWED_EMAILS` | ログインを許可するアドレス |

- `verify.yml`: PR / push で Go・Node のテスト（PostgreSQL 統合テスト込み）と `cf deploy --dry-run`。
- `deploy-web.yml`: main への push で Worker をデプロイ（Go 関連だけの変更では動かない）。

### 1.8 定期ジョブ（自宅サーバーの cron）

```cron
# 30日経過したソフト削除の完全削除
15 3 * * * cd /path/to/tasmane && bash scripts/run-cleanup.sh >> /var/log/tasmane-cleanup.log 2>&1
```

---

## 2. 公開後のスモークテスト

1. `https://tasmane.kokage-studio.com/` が表示される
2. ログイン → Google → 戻ってくる（許可リスト外のアカウントは 403）
3. ジャンルとタスクを作成し、子タスク追加時に確認ダイアログが出る
4. `/settings` でタイムゾーン保存、「今すぐ同期」→ Google Tasks に「タスマネ」リストができる
5. タスクを削除 → `/trash` から復元できる
6. 外部から Go に届かないこと: 自宅のグローバル IP:8080 に接続できない／Tunnel に Public Hostname が無い

---

## 3. 日常の更新

| 対象 | 手順 |
|---|---|
| フロント（Nuxt） | main へ push（Actions）または `npm run deploy:worker` |
| Go（API・ワーカー） | 自宅サーバーで `git pull && npm run deploy:go`（migrate が先に走る） |
| 両方に跨る API 変更 | **Go → Worker の順**で更新（Go は旧フロントとも互換に保つ） |
| Worker のシークレットだけ変更 | `secrets/worker.env` を更新して `npm run deploy:worker`（新バージョンとして同時に反映） |
| Go のシークレット変更 | `.env` を更新して `docker compose -f docker-compose.prod.yml up -d` |

Binding secret をローテーションする場合は、Worker と Go の値を同時に変える必要があります（片方だけだと 401）。Go を更新した直後に Worker をデプロイしてください。

---

## 4. ローカル開発

```bash
npm ci
# PostgreSQL（例: Docker）
docker run -d --name tasmane-pg -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=tasmane -p 5432:5432 postgres:16
export DATABASE_URL='postgres://postgres:dev@127.0.0.1:5432/tasmane?sslmode=disable'
# マイグレーション（ローカルでは自動適用されないので先に実行）
go run ./cmd/migrate
# Go API
TASMANE_INTERNAL_BINDING_SECRET=<32文字以上> TASMANE_IMPACT_TOKEN_SECRET=<32文字以上> \
TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY=$(openssl rand -base64 32) go run ./cmd/api
# Nuxt（VPC binding が無いときだけ NUXT_API_INTERNAL_URL の loopback へ転送）
NUXT_GOOGLE_CLIENT_ID=... NUXT_GOOGLE_CLIENT_SECRET=... NUXT_SESSION_SECRET=<32文字以上> \
NUXT_INTERNAL_BINDING_SECRET=<API と同じ値> NUXT_API_INTERNAL_URL=http://127.0.0.1:8080 npm run dev
```

## 5. テスト・検証

```bash
bash scripts/verify.sh            # gofmt / go vet / go test（-race 含む）/ Python 契約テスト / npm test・lint・typecheck・build
# PostgreSQL 統合テスト（任意。拡張は public に事前作成）
TEST_DATABASE_URL=postgres://...?sslmode=disable go test ./internal/repository ./db
# Worker ビルドの事前検証（認証不要）
TASMANE_API_VPC_SERVICE_ID=00000000-0000-4000-8000-000000000000 bash scripts/cf-build.sh && npx cf deploy --prebuilt --dry-run
```

OpenAPI とルータの一致は `TestRouterMatchesOpenAPIOperations`、MCP の削除ツール非公開は `mcp` と `frontend/tests/webmcp.test.ts` で検査されます。

---

## 6. 監視・ログ

| 対象 | 方法 |
|---|---|
| Worker | Cloudflare ダッシュボードの Workers Observability（ログ・トレース。`cloudflare.config.ts` で有効）。ライブログは `cf` 未対応のため必要時のみ `npx wrangler tail tasmane-web` |
| Go | `docker compose -f docker-compose.prod.yml logs -f api google-worker advice-scheduler` |
| Tunnel | `docker compose … logs -f cloudflared`、Cloudflare ダッシュボードの Tunnel 状態 |
| 死活 | 自宅サーバーで `curl -fsS http://127.0.0.1:8080/readyz` を cron/監視ツールで定期実行 |
| 同期 | `/settings` の Google Tasks（最終同期・失敗・コンフリクト） |

---

## 7. 障害対応

| 症状 | 確認・対処 |
|---|---|
| 画面で「Tasmane API is unreachable」(503) | 自宅サーバー停止・回線断・cloudflared 停止。`docker compose ps`、`logs cloudflared`。VPC Service の宛先 IP と `TASMANE_API_TUNNEL_IP` の一致 |
| 全 API が 401 | Worker と Go の Binding secret 不一致。両方を揃えて再デプロイ |
| ログイン後 403 | `NUXT_ALLOWED_EMAILS` に含まれていない |
| 409 `IMPACT_TOKEN_INVALID` | 確認中に他の変更が入った。画面を再読み込みしてやり直す |
| Google 同期が進まない（worker ログに `403 ... SERVICE_DISABLED`） | OAuth クライアントのプロジェクトで Google Tasks API が無効（1.2）。有効化して数分待てば、残っているジョブは自動で再実行される |
| Google 同期が進まない | `/settings` の失敗一覧、`logs google-worker`。refresh token 失効時は一度ログアウト→ログイン（同意画面で再許可） |
| メモ欄に「メモアプリにログインできませんでした（auth/invalid-credential）」 | Firebase の Google プロバイダの許可リストにタスマネの OAuth クライアント ID がない（1.6.1） |
| メモ欄に「Googleでの再認証が必要」 | ログイン時の ID トークン（1 時間）が切れた。ボタンから再認証する。メモアプリ側は一度ログインすれば Firebase のセッションが続く |
| メモ欄が「応答がありません」 | メモアプリが未デプロイ（連携非対応）か、`NUXT_PUBLIC_EMBED_PARENT_ORIGINS` にタスマネのオリジンがない |
| Google の専用リストを消した | `/settings` に警告が出る。「専用リストを作り直す」で新しいリストへ全タスクを再作成（連携解除したタスクを除く） |
| DB 障害 | Neon の状態確認。Go は再接続するため復旧後に `docker compose restart api` |

## 8. 切り戻し

- **Worker**: Cloudflare ダッシュボードの Deployments から前のバージョンへ戻すか、`npx cf workers deployments create --worker tasmane-web --strategy percentage --versions '[{"version_id":"<前のID>","percentage":100}]'`。
- **Go**: 前のコミットを checkout して `npm run deploy:go`。
- **DB**: migration は追加のみ。巻き戻しは新しい migration で前進修正（既存 migration は書き換えない）。

## 9. セキュリティ確認リスト

- [ ] Tunnel に Public Hostname が無く、ルーターのポート開放も無い
- [ ] `127.0.0.1:8080` 以外で Go に到達できない
- [ ] `NUXT_ALLOWED_EMAILS` が設定されている
- [ ] Binding / Session / Impact / Google token 暗号鍵がすべて別値かつ 32 文字以上（暗号鍵は 32 bytes）
- [ ] `.env`・`secrets/*` のパーミッションが 600 で Git に入っていない
- [ ] OAuth リダイレクト URI が本番 URL と完全一致
- [ ] 応答に CSP / X-Frame-Options / nosniff が付いている
