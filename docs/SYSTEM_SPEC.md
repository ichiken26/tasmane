# タスマネ システム仕様書（現行実装）

この文書は **現在のコードベースが実際にどう動くか** をまとめたものです。
要求の正本は `spec/PRODUCT_SPEC.md`、運用手順は `docs/OPERATIONS.md` を参照してください。
両者が食い違う場合は `spec/PRODUCT_SPEC.md` が優先されます（相違は末尾「仕様との差分」に記録）。

- 対象バージョン: OpenAPI `0.3.0` / DB migration `0009`
- 公開 URL: `https://tasmane.kokage-studio.com`（API は `/api/v1/*`）

---

## 1. システム構成

```text
Browser
  │ HTTPS（唯一の公開入口）
  ▼
Cloudflare Worker「tasmane-web」（Nuxt 3 / Nitro cloudflare_module, cf でデプロイ）
  │  - Google OAuth（Authorization Code + PKCE）とセッション Cookie
  │  - /api/v1/** を認証済みユーザーとして Go API へ転送
  │  - WebMCP Tool の登録（対応ブラウザのみ）
  │ Workers VPC Service binding「TASMANE_API」
  ▼
Cloudflare Tunnel（cloudflared。自宅サーバーからの外向き接続のみ・公開ホスト名なし）
  ▼
自宅サーバー（Docker Compose「tasmane」）
  ├─ api               Go + chi HTTP API（:8080。tunnel ネットワークの固定IP）
  ├─ cloudflared       Tunnel コネクタ
  ├─ google-worker     Google Tasks 同期ジョブの消費
  ├─ advice-scheduler  ユーザーTZ 04:00 の AI Advice 生成（Gemini）
  ├─ migrate           起動時マイグレーション（一回実行）
  └─ cleanup           Soft Delete 30日後の Hard Delete（ops プロファイル・cron）
  ▼
Neon PostgreSQL
```

| 要素 | 実装 | 設定ファイル |
|---|---|---|
| Web/BFF | Nuxt 3.21 + Nuxt UI 3 + Tailwind v4（SPA 描画） | `nuxt.config.ts`, `cloudflare.config.ts`, `wrangler.config.ts` |
| API | Go 1.22 + chi | `cmd/api`, `internal/transport/httpapi` |
| ドメイン | `internal/domain`（純粋ロジック）、`internal/application`（UseCase） | — |
| 永続化 | `internal/repository`（PostgreSQL / テスト用 Memory） | `db/migrations/*.sql` |
| MCP | stdio JSON-RPC（`cmd/mcp`, `mcp/`） | `mcp/tools.json` |
| WebMCP | `frontend/layers/core/plugins/webmcp.client.ts` | — |
| デプロイ | Worker: `cf`、Go: Docker Compose | `scripts/*.sh`, `docker-compose.prod.yml` |

### 1.1 ネットワークと信頼境界

- Browser が到達できるのは Worker のみ。Go API はインターネットに公開しない（ルーターのポート開放なし、Tunnel の Public Hostname なし）。
- Worker → Go は Workers VPC Service（宛先: Tunnel 経由の `api` コンテナ固定 IP `172.29.80.10:8080`）。
- Go API は全リクエストで `X-Tasmane-Internal-Binding`（Worker と共有する 32 文字以上の秘密）を定数時間比較で検証し、`X-Tasmane-Authenticated-User`（Worker が検証済みセッションから注入した内部ユーザー UUID）を利用者とする。
- Worker はクライアント由来の `X-Tasmane-*`・`Cookie`・`Authorization`・転送系ヘッダをすべて除去してから注入する。
- ホストからのヘルスチェック用に `127.0.0.1:8080` のみ公開（LAN からは到達不可）。

---

## 2. 認証・セッション・セキュリティ

| 項目 | 実装 |
|---|---|
| ログイン | `GET /auth/login` → Google（`openid email profile tasks`、`access_type=offline`、PKCE S256、state、nonce） |
| コールバック | `GET /auth/callback`：state・PKCE・ID トークン（署名・aud・iss・exp・nonce・email_verified）を検証 → 許可リスト確認 → `POST /internal/auth/bootstrap`（Go）でユーザー upsert・refresh token を AES-256-GCM で暗号化保存 |
| ログイン許可 | `NUXT_ALLOWED_EMAILS`（カンマ区切り）。未設定なら全員許可 |
| セッション | `__Host-tasmane-session`（AES-GCM 封緘、HttpOnly / Secure / SameSite=Lax / Path=/、7日） |
| CSRF | 状態変更（POST/PUT/PATCH/DELETE）は `Origin` が自オリジン（または `Sec-Fetch-Site: same-origin`）のみ受理。ログアウトも同一 Origin のみ |
| ログアウト | `POST /auth/logout`（Cookie 削除） |
| 応答ヘッダ | CSP（`default-src 'self'`、`connect-src 'self'`、`frame-ancestors 'none'` 等）、`X-Frame-Options: DENY`、`nosniff`、`Referrer-Policy` |
| アイコン | すべてクライアントへ同梱（外部 Iconify API を使わない） |
| API エラー | 500 は詳細をログのみに出し、クライアントへは `internal error` のみ返す |
| 入力上限 | JSON 1MB（bootstrap は 64KB）、未知フィールド拒否 |
| コンテナ | distroless nonroot、`read_only`、`cap_drop: ALL`、`no-new-privileges` |

---

## 3. ドメイン仕様（実装済みルール）

### 3.1 Task

- 状態: `NOT_STARTED → IN_PROGRESS → COMPLETED`。`IN_PROGRESS → NOT_STARTED` と `COMPLETED → IN_PROGRESS` は Impact Plan の確認つき。`NOT_STARTED → COMPLETED`、`COMPLETED → NOT_STARTED` は不可。
- 進捗: NOT_STARTED は NULL、COMPLETED は 100、COMPLETED → IN_PROGRESS は 95。
- 葉の進捗は IN_PROGRESS のときのみユーザーが 0〜95（5刻み）で設定（`PUT /v1/tasks/{id}/progress`、進捗履歴に記録）。
- 親の進捗 = Σ(子の進捗 × 子の実効優先度の重み) / Σ重み（HIGH=5 / MEDIUM=3 / LOW=1）。未完了の親は最大 95。
- 完了条件: 未完了の子孫がなく、未確認（UNCONFIRMED / CHECKING）の確認事項がないこと。満たすと応答の `canComplete=true`（画面に「完了可能」）。
- 子を全て失った親は葉へ戻り、進捗を状態に合わせて正規化（NOT_STARTED→NULL / IN_PROGRESS→0 / COMPLETED→100）。

### 3.2 優先度・スコア

- `effectivePriority = manualPriority ?? calculatedPriority`。
- `calculatedPriority` は期限（親は子孫の最短期限）・進捗・後続依存数から 0〜100 を算出し、0–34 LOW / 35–64 MEDIUM / 65–100 HIGH。期限なしは MEDIUM。
- TaskScore（0〜100、UI に数値は出さない）: 期限 30% / 実効優先度 25% / 依存 20% / 進捗 15% / 経過日数 5% / 確認 5%。未完了の前提 −40、確認中 −35（期限切れなら −10）。COMPLETED は対象外。係数は `internal/domain/scoring/constants.go`。
- これらの算出は `domain.ProjectTasks`（`internal/domain/projection.go`）に一本化し、HTTP（全タスク応答）・MCP・WebMCP・ダッシュボード・検索で共通に使う。

### 3.3 期限・炎

- `isOverdue`: 未完了かつ期限 < 現在。`showFire`: 自身または未完了子孫が期限切れ（COMPLETED は対象外）。

### 3.4 階層（Hierarchy Tree）

- 同一ユーザー・同一ジャンルのみ。自己参照・循環は禁止。
- 子タスク作成・親の付け替えで、子を持たないタスクを親にする場合は Leaf→Parent 確認（「進捗率は自動で再計算されます。子タスクを追加しますか？」）。COMPLETED のタスクは親にできない。
- 子の日付が親の範囲外になる作成・付け替え・日付変更は、祖先・子孫の日付伝播を Impact Plan で確認してから一括適用。
- ジャンル変更は部分木ごと移動し、境界を跨ぐ依存関係は確認後に解除。元の親から切り離され新ジャンルの根になる。

### 3.5 依存関係（Dependency Graph）

- 同一ユーザー・同一ジャンルのみ、自己依存・循環は禁止。
- 未完了の前提タスクがある状態で着手（NOT_STARTED → IN_PROGRESS）すると、前提タスクを列挙した Impact Plan で確認を求める（承認すれば着手可）。
- 日程矛盾（前提タスクの期限より前に開始／後続タスクの開始より後に期限）は保存した上で、更新応答の `warnings` で警告。

### 3.6 削除・復元

- 削除は Soft Delete。子孫と、削除タスクに（再帰的に）依存するタスクを含む集合を Impact Plan で提示し、承認後に同一トランザクションで削除。Impact Token は削除集合に束縛され、承認後に集合が増えたら再確認。
- `/trash` で 30 日以内の削除を「同じ削除操作の単位」で復元。削除済みの親は根として復元、存在する親が COMPLETED なら不可、ジャンル削除済みなら不可。
- 30 日経過で `cleanup` が Hard Delete。

### 3.7 確認事項（Confirmation）

- 状態: `UNCONFIRMED ⇄ CHECKING ⇄ CONFIRMED`（UNCONFIRMED と CONFIRMED の直接遷移は不可）。
- COMPLETED のタスクには未確認の確認事項を追加できず、CONFIRMED から戻すこともできない（完了制約の不変条件）。
- 確認先はプリセット（ユーザー個人、大文字小文字を区別せず一意）から選ぶか自由入力。入力時に「プリセットに保存」可能。
- ダッシュボードの推薦順は ConfirmationScore（期限 50% / 状態 20% / 親 TaskScore 30%）。

### 3.8 ジャンル・タスクタイプ

- 名前は同一ユーザーで大文字小文字を区別せず一意。タスクがあるジャンルは削除不可。
- タスクタイプはユーザー共通。ジャンルとは多対多。使用中のものはジャンルから外せず、削除もできない。

### 3.9 Impact Plan / 確認フロー

- 確認が必要な操作は HTTP 409 `IMPACT_CONFIRMATION_REQUIRED`（`impactToken`・`affectedEntities`・`warnings`）。同じ操作を `impactToken` 付きで再送すると、サーバーが version と計画を再検証して同一トランザクションで適用。
- トークンは HMAC 署名・5 分有効・ユーザー・操作内容（計画のハッシュ）・対象 version に束縛。
- 対象: 削除、ジャンル移動、日付伝播、Leaf→Parent 変換、状態の出戻り、未完了前提への着手。

### 3.10 楽観ロック

全更新は `expectedVersion` 必須。不一致は 409 `CONFLICT`。

---

## 4. Google Tasks 連携

| 項目 | 実装 |
|---|---|
| 対象リスト | 専用リスト「タスマネ」を自動作成し ID を保存。Google 側で削除されたら設定画面に警告し、「専用リストを作り直す」で全タスクを新リストへ再作成（復旧フロー） |
| 同期項目 | title⇄title、body⇄notes（紐付けメモがあれば notes 末尾に「── memo(tasmane) ──」見出しとタイトル・URL を追記。取込時は見出し以降を除いて本文に戻す。メモの紐付け・解除でも同期）、期限の日付（ユーザー TZ）⇄due、完了⇄status。開始日・時刻・階層・依存・優先度・確認事項・進捗は同期しない |
| タスマネ→Google | タスク更新と同一トランザクションで outbox（`google_sync_jobs`）へ投入し即応答。`google-worker` が lease 付きで消費 |
| Google→タスマネ | 手動同期、または認証済みユーザーの API アクセス時に「最終同期から設定間隔以上」ならプル同期を投入（ユーザー単位 lock、プロセス内で 1 分に 1 回まで判定） |
| マージ | フィールド単位の 3-way。別フィールドの変更は自動マージ、同一フィールドの双方変更は CONFLICT |
| due 変更 | 既存の時刻（hh:mm）はユーザー TZ で保持。新規 due は終日（ユーザー TZ の当日終わり）。祖先の期限伝播が必要なら CONFLICT |
| Google で完了 | 通常の完了 UseCase（NOT_STARTED→IN_PROGRESS→COMPLETED、履歴 2 件、via GOOGLE_TASKS）。完了不可なら CONFLICT |
| Google で未完了化 | COMPLETED→IN_PROGRESS/95、必要な祖先も戻す |
| Google で削除 | CONFLICT。UI で「タスマネでも削除（通常の削除・2段階確認）」「タスマネに残して連携解除」「Google に再作成」 |
| Field conflict | UI で双方の値を表示。「タスマネの値を残す」（Google を上書き）／「Google の値で編集」（タスク詳細で通常の更新・Impact 確認を経て反映後 Google へ） |
| タスマネで削除 | Google 側も削除（既に無ければ成功扱い）。30 日以内に復元すると Google に新規作成し再紐付け |
| Google-only | 取込候補として表示。ユーザーがジャンルを選んで取り込む。Google で削除された候補は消える |
| 再試行 | 429/5xx は指数バックオフ（最大 5 回）、401/403 はトークン更新後に再試行、恒久 400 は FAILED、CONFLICT は再試行しない |
| 失敗表示 | 設定画面に最終 FAILED のタスク一覧。「今すぐ同期」で再送 |
| 1 件の CONFLICT | 他タスクの同期は継続 |

---

## 4.1 メモアプリ連携

設定「メモアプリと連携する」（`memoIntegrationEnabled`）がオンのユーザーだけ有効。

**表示**
- タスクのメモ欄にメモアプリ（`NUXT_PUBLIC_MEMO_APP_ORIGIN`、既定 `https://memo.kokage-studio.com`）を iframe で埋め込む。
- 新規作成時は新規メモを開く。保存通知（postMessage `memo-saved`）を受けると、そのメモを Task に紐付ける。作成前は保留し、作成後に保存する。
- 紐付けたメモはリンクチップで表示する。
  - 通常クリック: 右側パネルに表示し、作成・編集フォームは左へ寄せる。`lg` 未満の幅ではスライドオーバー。
  - Ctrl/⌘/Shift/中クリック: 別タブで開く。
- 紐付けの保存は `task_memo_links`。同一 Task・同一 URL はタイトル更新のみ。解除してもメモ本体は消さない。
- メモアプリのメモ ID（`memo-yyyymmdd-名前`）は、表示タイトルを名前だけにし、日付は `memo_date`（URL 末尾のメモ ID から抽出）に保持する（migration 0011。既存行も補完）。API の `MemoLink.memoDate`。

**ログイン引き継ぎ**
- ログイン時の Google ID トークンを `__Host-tasmane-memo`（暗号化・HttpOnly・トークン期限まで）に保持する。
- `GET /api/memo/handoff`（同一オリジンのみ）で取り出し、iframe へ postMessage（送信先オリジン固定）する。
- メモアプリは `signInWithCredential`（`google.com`、raw nonce 付き）で Firebase にログインする。
- Google へ送る nonce は SHA-256 16 進、トークン検証も同じ値で行う。
- 期限切れのときは「Googleで再認証」を案内する。

**セキュリティ**
- タスマネ: CSP `frame-src` をメモアプリのオリジンだけに限定する。受信メッセージは送信元フレーム・オリジン・形式をすべて確認する。
- メモアプリ: `frame-ancestors` を `NUXT_PUBLIC_EMBED_PARENT_ORIGINS` に限定する。親オリジンは `ancestorOrigins`（なければ referrer）で特定し、許可外なら連携しない。

**メモアプリ側の変更**（`../memoAppNuxtTraining`）
- `shared/embedBridge.ts`
- `app/composables/useEmbedBridge.ts`
- `app/plugins/embed.client.ts`
- `server/middleware/frame-ancestors.ts`
- 保存時通知（`pages/memo/new.vue` / `[id].vue`）

---

## 5. AI Advice

- `advice-scheduler` がユーザー TZ の 04:00 以降に全体・ジャンル別の Advice を Gemini で生成し保存（入力状態の `source_hash` で重複生成を防止、30 日保持）。
- ランキングには使わない（おすすめ順は決定論的な TaskScore）。

---

## 6. HTTP API（Go。Worker 経由で `/api/v1/*`）

正確なスキーマは `api/openapi.yaml`（ルータとの一致はテスト `TestRouterMatchesOpenAPIOperations` で保証）。

性能（Neon は遠隔で1往復 約100ms のため、往復回数を最小にする）:
- 接続は `db.Open` / `db.OpenMigrated` に集約。プール最大16本・待機16本を保ち（アイドル4分・寿命30分）、pgx は `QueryExecModeExec`（prepare の往復なしで常に1往復）を既定にする。URL に `default_query_exec_mode` を書けばそれを優先。JSON 列へは文字列で渡す（`[]byte` は bytea 扱いになる）。
- 投影（ダッシュボード・検索・タスク詳細・ツリー）はタスク・依存グラフ・確認事項（ジャンル単位）を goroutine で並行取得し1往復。ダッシュボードの助言、検索の一致判定も並行。依存グラフは `UNION ALL` の1 SQL、Google 同期状態も1 SQL。
- フロントは同時に発生した同じ GET を1回の通信にまとめる（`shareInFlight`）。
- google-worker はユーザー・タスク単位の直列性（Google 側の二重作成防止）を保つため1本で消費する。

| メソッド・パス | 用途 |
|---|---|
| `GET /v1/tasks` | 検索（`q`, `genreIds`, `priority`=実効優先度, `deadline`=today/tomorrow/week/month/later/overdue, `sort`=taskScore/priority/deadline） |
| `POST /v1/tasks` | 作成（Leaf→Parent・日付伝播は 409 で確認） |
| `GET/PATCH/DELETE /v1/tasks/{id}` | 取得・部分更新（`null` で解除）・削除（`expectedVersion` 必須、2 段階） |
| `POST /v1/tasks/{id}/transition` | 状態遷移 |
| `PUT /v1/tasks/{id}/progress` | 葉の進捗 |
| `GET /v1/tasks/{id}/tree` | 所属する階層全体 |
| `GET/POST /v1/tasks/{id}/dependencies`, `DELETE …/{dependsOnTaskId}` | 依存関係 |
| `GET/POST /v1/tasks/{id}/confirmations`, `PATCH /v1/confirmations/{id}` | 確認事項 |
| `GET/POST /v1/confirmation-targets`, `DELETE …/{id}` | 確認先プリセット |
| `GET /v1/deleted-tasks`, `POST /v1/deleted-tasks/{id}/restore` | ゴミ箱・復元 |
| `GET/POST /v1/genres`, `GET/PATCH/DELETE /v1/genres/{id}` | ジャンル |
| `GET/POST /v1/genres/{id}/task-types`, `DELETE …/{taskTypeId}` | ジャンルのタスクタイプ |
| `GET/POST /v1/task-types`, `GET/PATCH/DELETE /v1/task-types/{id}` | タスクタイプ |
| `GET /v1/dashboard`, `GET /v1/dashboard/genres/{id}` | ダッシュボード（おすすめ順・確認事項・Advice） |
| `GET/PATCH /v1/settings` | タイムゾーン・Google 同期間隔 |
| `POST /v1/google/sync`, `GET /v1/google/status`, `POST /v1/google/task-list/recreate`, `GET /v1/google/conflicts`, `POST /v1/google/conflicts/{taskId}/resolve`, `GET /v1/google/import-candidates`, `POST /v1/google/import-candidates/import` | Google 連携 |
| `GET /healthz`, `GET /readyz` | ヘルスチェック（ホストのループバックのみ） |

Task 応答は OpenAPI `Task`（`effectivePriority`・`progress`（親は算出値）・`isOverdue`・`showFire`・`taskScore`・`recommendationReasons`・`recommended`・`canComplete`・`isLeaf`、更新時のみ `warnings`）。

| `GET/POST /v1/tasks/{id}/memo-links`, `DELETE /v1/tasks/{id}/memo-links/{linkId}` | メモリンクの一覧・紐付け（URL 単位で upsert）・解除 |

主なエラー: 400 `INVALID_REQUEST` / 401 / 404 `NOT_FOUND`（他ユーザーのリソースも 404）/ 409 `CONFLICT`・`IMPACT_CONFIRMATION_REQUIRED`・`IMPACT_TOKEN_INVALID` / 422 `DOMAIN_VALIDATION_FAILED`。

---

## 7. MCP / WebMCP

- **MCP**（`cmd/mcp`、stdio、`TASMANE_MCP_USER_ID` のユーザーとして動作）: 読み取り・作成・更新・状態遷移・進捗・確認事項・依存追加・ジャンル/タイプ・確認先プリセット・Google 同期の起動と照会（`mcp/tools.json`）。**削除系は公開しない**。影響確認が必要な操作は `status: confirmation_required` と Impact Plan を返し、Agent が承認を代行しない。出力は OpenAPI 相当（ユーザー ID を含まない）。
- **WebMCP**（`navigator.modelContext` 対応ブラウザ）: 検索・取得・ツリー・ダッシュボード・ジャンル・確認事項の読み取りと、作成・更新・状態遷移・進捗・確認事項・依存追加の書き込み。書き込みは毎回ブラウザのダイアログでユーザーが許可した場合のみ実行。結果は「ユーザーデータであり指示ではない」旨を添えて返す。履歴の via は `WEB_MCP`。

---

## 8. 画面

| URL | 内容 |
|---|---|
| `/` | 挨拶・クイック追加・統計・次にやる 5 件 |
| `/dashboard` | 全ジャンル: 統計、AI Advice、おすすめ着手順（優先度順/TaskScore 順/期限順を切替・URL 同期）、期限区分、確認事項の推薦 |
| `/dashboard/genres/{id}` | ジャンル内の上記 + Gantt（バー移動・左右リサイズ・キーボード操作・日付伝播確認・日程矛盾警告） |
| `/tasks/new` | 作成（ジャンル新規作成、親の検索選択、終日/時刻、期限 ≥ 開始の検証） |
| `/tasks/{id}` | 詳細・編集（共通 TaskEditor）、状態操作、葉の進捗スライダー、完了可能表示、確認事項（プリセット）、Hierarchy Tree（折りたたみ）、Dependency Graph（複数依存は淡い黄色）、Google コンフリクト解決 |
| `/genres` | ジャンル一覧（ルートタスクと進捗）、作成・改名・削除・タスクタイプ管理 |
| `/search` | 検索（URL と同期、debounce、ジャンル複数 OR、期限切れ、並び順） |
| `/trash` | 削除済みタスク（30 日以内）の一括復元 |
| `/settings` | プロフィール、テーマ、タイムゾーン・同期間隔・メモアプリ連携、確認先プリセット、Google 同期（状態・失敗・コンフリクト・取込） |

共通: Header（ロゴ・Top・Dashboard・新規タスク・ジャンル一覧・検索・テーマ・プロフィール）、Sidebar（表示中ジャンルを強調）、ページタイトル、BroadcastChannel による他タブ共有（version で新旧比較）、未保存の離脱確認、ライト/ダーク。

---

## 9. データモデル（主要テーブル）

`users`, `user_server_settings`, `genres`, `task_types`, `genre_task_types`, `tasks`（`leaf_progress`・`version`・`deleted_at`）, `task_dependencies`, `confirmation_targets`, `task_confirmations`, `task_status_histories`, `task_progress_histories`, `ai_advices`, `google_integrations`, `google_task_syncs`（`conflict_remote_snapshot`）, `google_sync_jobs`, `google_unlinked_tasks`, `task_memo_links`（`memo_date`、`user_server_settings.memo_integration_enabled`）。
主キーは UUIDv7。整合性は CHECK・外部キー（ユーザー・ジャンル単位の複合キー）で担保し、残りはトランザクション内のドメインルールで担保。

---

## 10. 仕様との差分（判断記録）

| 項目 | 仕様書 | 現行 | 判断 |
|---|---|---|---|
| 実行基盤 | （旧）Cloudflare Containers | 自宅サーバー + Tunnel + Workers VPC | オーナー指示。仕様書を更新済み（`spec/DECISIONS_2026-11-09.md`） |
| Google 同期キュー | （旧）Cloudflare Queues | PostgreSQL outbox | 同上の構成に合わせた判断。仕様書を更新済み |
| `/trash`・`/settings` | 主要 URL に未記載 | 実装 | 復元・プリセット・同期解決に必要な画面（UX）。仕様書に追記済み |
| 検索の並び順 | — | `sort` を URL 同期 | OpenAPI に既定の機能。仕様書に追記済み |
| ログイン許可リスト | — | `NUXT_ALLOWED_EMAILS` | セキュリティ上必要。仕様書に追記済み |
| メモアプリ連携 | — | 埋め込み・メモリンク・ログイン引き継ぎ | オーナー要望（2026-11-10）。仕様書 §16.1 に追記済み |
