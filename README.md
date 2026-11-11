# タスマネ

仕様駆動・TDDで構築するタスク管理アプリ。Go APIの認証境界・共通UseCaseと、Nuxt 3のレスポンシブなタスク管理UIを提供する。

## 開発と検証

Go 1.22以上（検証済み: 1.22.12）、Node.js 22/npm 10、Python 3を用意する。Go 1.22.12は[公式配布ページ](https://go.dev/dl/)からOS/CPUに合うアーカイブを取得し、同ページ掲載のSHA256と照合して任意のtoolchainディレクトリへ展開する。ユーザー固有の場所には依存しない。

```bash
export TASMANE_GO_ROOT=/absolute/path/to/go1.22.12
export PATH="$TASMANE_GO_ROOT/bin:$PATH"
export TASMANE_GO_BIN="$TASMANE_GO_ROOT/bin/go"
export GOFLAGS=-buildvcs=false
export DATABASE_URL='postgres://...'
export TASMANE_INTERNAL_BINDING_SECRET='at-least-32-random-bytes-here'
export TASMANE_IMPACT_TOKEN_SECRET='a-different-at-least-32-byte-secret'
export TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY='base64-of-exactly-32-random-bytes'
bash scripts/bootstrap.sh
bash scripts/verify.sh
"$TASMANE_GO_BIN" run ./cmd/api
```

Google Tasks同期workerはAPIとは別processで常駐させる。OAuth callbackを担当するWorkerは取得したrefresh tokenを内部bootstrapへ渡し、Go APIが同じAES-256-GCM鍵で暗号化して`google_integrations.encrypted_refresh_token`へ保存する。APIは標準Base64、workerはraw 32 byteまたはBase64表現を受け付ける。平文tokenをDB・ログ・Browserへ出さない。

```bash
export DATABASE_URL='postgres://...'
export GOOGLE_OAUTH_CLIENT_ID='...apps.googleusercontent.com'
export GOOGLE_OAUTH_CLIENT_SECRET='...'
export TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY='32-byte-secret-or-base64'
"$TASMANE_GO_BIN" run ./cmd/google-worker
```

workerはPostgreSQLのdurable queueを`FOR UPDATE SKIP LOCKED`でleaseし、429/5xxを指数backoff、401/403をtoken refresh後に再試行する。ユーザー単位同期lock、専用`Tasmane` Task List、field snapshot conflict、Google-only import candidateをDBへ永続化する。`POST /v1/google/sync`は同期をqueueへ投入して202、既に同期中なら409を返す。

Frontend開発は`npm run dev`、本番相当ビルドは`npm run build`。`verify.sh`はFrontendのtest/lint/typecheck/buildとGoのformat/vet/test/buildを一工程でも省略せず実行する。

### Cloudflare Worker認証Gateway

Nuxt/Nitro Workerが唯一の公開入口となり、Google Authorization Code + PKCEでログインする。OAuth transactionと7日間のlogin sessionはAES-GCMで暗号化・改ざん検知した`HttpOnly; Secure; SameSite=Lax; Path=/`の`__Host-` Cookieへ保存され、Google tokenや内部Binding secretをBrowserへ返さない。`POST /auth/logout`は同一Originのみ受理する。

Worker Secret は `secrets/worker.env`（Git 管理外。`secrets/worker.env.example` 参照）に置き、`cf deploy --prebuilt --secrets-file secrets/worker.env`（`npm run deploy:worker`）で Worker のバージョンと同時に投入する。

- `NUXT_GOOGLE_CLIENT_ID` / `NUXT_GOOGLE_CLIENT_SECRET` / `NUXT_GOOGLE_REDIRECT_URI`
- `NUXT_SESSION_SECRET`: 32文字以上。session/OAuth Cookie暗号化用
- `NUXT_INTERNAL_BINDING_SECRET`: 32文字以上。自宅サーバーの `TASMANE_INTERNAL_BINDING_SECRET` と同値
- Googleログインは事前のメールアドレス登録不要。Google Cloud OAuth は「外部 / 本番環境」に公開し、必要な検証を完了する。
- `TASMANE_API`: `cloudflare.config.ts` で定義する Workers VPC Service binding（Cloudflare Tunnel 経由で自宅サーバーの Go API へ）。local開発だけは`NUXT_API_INTERNAL_URL=http://127.0.0.1:8080`へfallbackする

Browserの`/api/v1/**` requestはsession認証（状態変更は同一Originのみ）後にWorkerがVPC Service bindingへ転送する。その際、client由来の全`X-Tasmane-*`およびforwarding headerを除去し、検証済みuser UUIDと内部Binding secretだけを注入する。OAuth callbackは検証済みGoogle subject/profileとrefresh tokenを内部専用`POST /internal/auth/bootstrap`へ送り、Go側でuserをupsertしrefresh tokenをAES-256-GCM暗号化してNeonへ保存する。

自宅サーバーでは API は Docker 内部の tunnel ネットワークとホストの `127.0.0.1:8080` のみで待受ける。Worker（VPC Service binding）から `X-Tasmane-Internal-Binding` と、OAuth検証後の `X-Tasmane-Authenticated-User` を注入する。BrowserからGoコンテナを直接公開しない。クライアント指定の `userId` は拒否される。

状態遷移例:

```bash
curl -X POST http://127.0.0.1:8080/v1/tasks/TASK_ID/transition \
  -H 'X-Tasmane-Internal-Binding: development-only-secret' \
  -H 'X-Tasmane-Authenticated-User: USER_ID' \
  -H 'Content-Type: application/json' \
  -d '{"toStatus":"IN_PROGRESS","expectedVersion":1}'
```

本番ではBinding secretをWorker Secretと自宅サーバーの `.env` で管理し、ログやBrowserへ出さない。更新は必ず最新 `version` を `expectedVersion` として送り、409時は最新Taskを再取得してユーザー変更を再適用する。

## Migrationと運用

`db/migrations` を番号順に単一transactionで適用する。既存migrationは変更せず、変更は新しい番号で追加する。デプロイ前後に `bash scripts/verify.sh` を実行する。

Dockerやsystem-wide installなしのfresh PostgreSQL検証は `./scripts/test-postgres.sh` で実行する。Ubuntu 22.04のPostgreSQL packageを一時directoryへ展開し、loopback限定のisolated instanceへmigrationを3回適用してidempotency、制約、concurrent `updated_at` を検証後、processと一時fileを削除する。

障害対応:

- 401: Worker Binding secretと認証済みUser Context注入を確認。外部からの直接到達は許可しない。
- 409 version conflict: 最新entityを再取得し、変更内容を確認して新versionで再送。
- 409 domain conflict: 未完了子孫・未確認Confirmationを解消。Google同期も制約を迂回しない。
- verifyのtoolchain error: Go 1.22以上とPython 3を導入。検証は欠落工程を成功扱いしない。

## 想定構成

- Frontend: Nuxt / Cloudflare Workers
- API: Go + chi（自宅サーバー。Cloudflare Tunnel + Workers VPC Service 経由のみ・非公開）
- DB: Neon PostgreSQL
- Queue: PostgreSQL durable queue（Google同期）
- Cloudflare CLI: `cf`（Worker のビルドは `scripts/cf-build.sh` → `cf deploy --prebuilt`）
- AI: Gemini API（`cmd/advice-scheduler`）
- Root: `https://tasmane.kokage-studio.com`
- API: `https://tasmane.kokage-studio.com/api/v1/*`

## 本番リリース

手順の正本は [`docs/OPERATIONS.md`](docs/OPERATIONS.md)（要約: [`docs/PRODUCTION.md`](docs/PRODUCTION.md)、現行仕様: [`docs/SYSTEM_SPEC.md`](docs/SYSTEM_SPEC.md)）。

最短コマンド:

```bash
bash scripts/gen-secrets.sh            # .env / secrets/worker.env 用の値
npm run setup:cloudflare               # cf で Tunnel・VPC Service 作成（初回）
cp .env.example .env && npm run deploy:go          # 自宅サーバー
TASMANE_API_VPC_SERVICE_ID=… npm run deploy:worker # 作業PC / CI
# 日次: bash scripts/run-cleanup.sh
```

## 使い方

このディレクトリを実装対象リポジトリのルートへコピーします。

```bash
cp -R tasmane-tdd-harness/* /path/to/tasmane/
cd /path/to/tasmane
chmod +x scripts/*.sh
```

エージェントCLIを環境変数に設定します。

### Codex CLI例

```bash
export AGENT_CMD='codex exec --full-auto -'
./scripts/tdd-loop.sh
```

### Claude Code例

```bash
export AGENT_CMD='claude -p'
./scripts/tdd-loop.sh
```

CLIごとにオプションが異なる場合は `AGENT_CMD` を調整してください。

## ループの停止条件

以下がすべて満たされたときだけ終了します。

1. `scripts/verify.sh` が成功
2. `state/blocked.md` に未解決P0/P1がない
3. `state/implementation-checklist.md` の必須項目がすべて完了
4. Domain / DB / API / integration tests がすべて成功
5. エージェント自身による最終レビューで Critical / High 指摘が0件

未決仕様が実装をブロックする場合、推測で埋めず `state/blocked.md` に記録して停止します。

## 重要

`spec/PRODUCT_SPEC.md` が正本です。実装中にコードと仕様が食い違った場合、勝手に仕様を変更しないでください。
