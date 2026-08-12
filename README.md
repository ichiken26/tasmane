# タスマネ

仕様駆動・TDDで構築するタスク管理アプリ。Go APIの認証境界・共通UseCaseと、Nuxt 3のレスポンシブなタスク管理UIを提供する。

## 開発と検証

Go 1.22以上（検証済み: 1.22.12）、Node.js 22/npm 10、Python 3を用意する。Go 1.22.12は[公式配布ページ](https://go.dev/dl/)からOS/CPUに合うアーカイブを取得し、同ページ掲載のSHA256と照合して任意のtoolchainディレクトリへ展開する。ユーザー固有の場所には依存しない。

```bash
export TASMANE_GO_ROOT=/absolute/path/to/go1.22.12
export PATH="$TASMANE_GO_ROOT/bin:$PATH"
export TASMANE_GO_BIN="$TASMANE_GO_ROOT/bin/go"
export GOFLAGS=-buildvcs=false
export TASMANE_INTERNAL_BINDING_SECRET='development-only-secret'
bash scripts/bootstrap.sh
bash scripts/verify.sh
"$TASMANE_GO_BIN" run ./cmd/api
```

Frontend開発は`npm run dev`、本番相当ビルドは`npm run build`。`verify.sh`はFrontendのtest/lint/typecheck/buildとGoのformat/vet/test/buildを一工程でも省略せず実行する。

APIは `127.0.0.1:8080` のみで待受ける。Cloudflare Worker Bindingから `X-Tasmane-Internal-Binding` と、OAuth検証後の `X-Tasmane-Authenticated-User` を注入する。BrowserからGoコンテナを直接公開しない。クライアント指定の `userId` は拒否される。

状態遷移例:

```bash
curl -X POST http://127.0.0.1:8080/v1/tasks/TASK_ID/transition \
  -H 'X-Tasmane-Internal-Binding: development-only-secret' \
  -H 'X-Tasmane-Authenticated-User: USER_ID' \
  -H 'Content-Type: application/json' \
  -d '{"toStatus":"IN_PROGRESS","expectedVersion":1}'
```

本番ではBinding secretをCloudflare Secretで管理し、ログやBrowserへ出さない。更新は必ず最新 `version` を `expectedVersion` として送り、409時は最新Taskを再取得してユーザー変更を再適用する。

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
- API: Go + chi / Cloudflare Containers
- DB: Neon PostgreSQL
- Queue: Cloudflare Queues
- AI: Gemini API
- Root: `https://tasmane.kokage-studio.com`
- API: `https://tasmane.kokage-studio.com/api/*`

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
