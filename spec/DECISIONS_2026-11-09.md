# Decisions — 2026-11-09

オーナー指示および実装レビューにより確定した事項。

1. 実行基盤: Nuxt は Cloudflare Workers、Go コンテナは自宅サーバー（Docker Compose）。両者は Cloudflare Tunnel で接続し、Worker からは Workers VPC Service binding（`TASMANE_API`）経由でのみ Go API へ到達する。Cloudflare Containers は使わない。
2. ツール: Cloudflare 側の操作は wrangler ではなく `cf` CLI を使う。cf（beta）は Nuxt の Build Output を生成しないため、ビルドは `scripts/cf-build.sh`（nuxt build → Wrangler bundler の Build Output 生成）で行い、`cf deploy --prebuilt` でデプロイする。wrangler パッケージは bundler として内部利用するのみ。
3. Google同期キュー: Cloudflare Queues ではなく PostgreSQL durable queue（タスク更新と同一 Transaction の outbox）。Go が自宅サーバーにあるため Queue consumer も自宅サーバーで動かす方が単純で、更新と投入の原子性も保てる。仕様の要求（DB保存後即応答・非同期・retry・user単位lock）は満たす。
4. Refresh token 暗号化鍵: 自宅サーバーの環境変数で管理する（Worker・Browser へ渡さない）。
5. ログイン許可リスト: 自宅サーバーへの負荷・Gemini 利用量の保護のため `NUXT_ALLOWED_EMAILS` を追加。
6. 削除の Impact Token は削除集合に束縛する（承認後に影響範囲が増えた場合は再確認）。
7. 状態変更 API は同一 Origin のみ受け付ける（SameSite=Lax は同一サイトの別サブドメインからの CSRF を防げないため）。
8. 互換用の `/v1/**` 公開パスは廃止し、公開 API は仕様どおり `/api/v1/**` のみとする。
