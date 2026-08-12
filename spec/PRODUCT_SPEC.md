# タスマネ 実装仕様 正本

## 1. アーキテクチャ

```text
Browser
  ↓
Cloudflare Workers / Nuxt
  - Google OAuth
  - 認証済みUser Context注入
  - WebMCP側Permission/Injection
  ↓ /api/*
Cloudflare Containers / Go + chi
  - HTTP Adapter
  - MCP Adapter
  - Application UseCases
  - Domain
  - Repository
  ↓
Neon PostgreSQL
```

非同期:
- Cloudflare Queues: Google Tasks同期
- Scheduler/Job: ユーザーTimezone基準04:00のAI Advice
- Gemini API: AI Advice生成

Root: `https://tasmane.kokage-studio.com`
API: `https://tasmane.kokage-studio.com/api/*`

## 2. 保存原則

事実・ユーザー入力・外部同期状態は保存する。
保存値から再現可能な値は原則算出する。

保存:
- Task status（Leaf/Parent両方）
- Leaf progress（Leafである間のみ。ParentではNULL）
- manualPriority
- startDate/endDate
- isAllDay
- parentTaskId
- genreId
- taskTypeId?
- Dependency
- Confirmation
- AI Advice
- Google sync state
- history
- optimistic lock version
- users.timezone
- server-side settings（Google sync interval等）

算出:
- Parent progress
- calculatedPriority
- effectivePriority
- TaskScore
- recommended order/reasons
- Genre progress/score/order
- depth/isRoot/isLeaf/counts
- daysUntilDeadline/isOverdue/deadlineCategory/duration
- hasPendingConfirmation
- needsSync

## 3. Task

Status:
- NOT_STARTED
- IN_PROGRESS
- COMPLETED

許可:
- NOT_STARTED -> IN_PROGRESS
- IN_PROGRESS -> COMPLETED
- IN_PROGRESS -> NOT_STARTED（確認Modal）
- COMPLETED -> IN_PROGRESS（確認Modal）

禁止:
- NOT_STARTED -> COMPLETED
- COMPLETED -> NOT_STARTED

Progress:
- NOT_STARTED: NULL
- IN_PROGRESS: 0..95, 5刻み
- COMPLETED: 100

COMPLETED -> IN_PROGRESS: 95へ戻す。
IN_PROGRESS -> NOT_STARTED: NULLへ戻す。

## 4. Genre / TaskType

TaskはGenreを必ず1つ持つ。
Tree全体でGenreを統一。
TaskTypeは任意・最大1つ。

TaskType定義はユーザー共通。
Genre <-> TaskTypeはMany-to-Many。
Taskに設定できるTaskTypeはそのGenreに関連付け済みのものだけ。

Genre/TaskType名:
- 同一ユーザーでcase-insensitive unique

使用中TaskType:
- Genreから解除禁止
- TaskType自体の削除禁止

Taskが存在するGenre:
- 削除禁止

## 5. Task Hierarchy

`parent_task_id`による自己参照。
固定深度上限なし。
同一user・同一Genreのみ親子可。
自己参照/循環禁止。

Parent削除:
- 子孫を含めSoft Delete可能
- UIは2段階確認
- Soft Delete 30日後Hard Delete

### Parent Progress

```text
raw =
Σ(childProgress × effectivePriorityWeight)
/
Σ(effectivePriorityWeight)

HIGH=5
MEDIUM=3
LOW=1
```

ParentがCOMPLETEDでなければ表示/実効progressは最大95。
ParentがCOMPLETEDなら100。

```text
parentProgress =
if status == COMPLETED then 100
else min(raw, 95)
```

未完了Child/DescendantがあるParentはCOMPLETED不可。
未確認ConfirmationがあるParentも通常Taskと同様COMPLETED不可。

全子孫COMPLETED:
- Parentは自動完了しない
- IN_PROGRESS / 95
- 「完了可能」表示
- ユーザー明示操作でCOMPLETED / 100

Child COMPLETED -> IN_PROGRESS:
- COMPLETED済みancestorをRootまでIN_PROGRESS / 95へ戻す
- UI操作なら影響範囲を確認表示

Parent IN_PROGRESS -> NOT_STARTED:
- IN_PROGRESS子孫をNOT_STARTED / progress=NULLへ
- 影響範囲確認

Parent自身 COMPLETED -> IN_PROGRESS:
- Parentだけ95へ戻す
- COMPLETED子孫はそのまま

### Leaf -> Parent変換

Leaf Taskへ最初の子Taskを追加するとき:
- 対象Taskが`COMPLETED`なら子Task追加は禁止
- 未完了Taskなら、追加前に確認Modalを表示する
- Modal文言: `進捗率は自動で再計算されます。子タスクを追加しますか？`
- 承認後に子Taskを追加し、対象TaskはParentとして扱う
- 以後のParent progressは子Taskから自動算出する
- Parentでは`leaf_progress`を永続値として使用しないためNULLへクリアする
- キャンセル時は何も変更しない

最後のChildがなくなりParent -> Leafへ戻るケースは、削除処理の結果として発生しうる。
その場合のLeaf progressは現在statusに整合する値へ正規化する:
- NOT_STARTED -> NULL
- IN_PROGRESS -> 0
- COMPLETED -> 100

## 6. Dependency

Hierarchyとは別概念。

```text
task_dependencies(task_id, depends_on_task_id)
```

- 同一user・同一Genre内のみ
- Many-to-Many
- 自己依存/循環禁止
- 依存先未完了でも着手可。ただしWarning確認
- 複数Taskに依存しているTaskをUIで淡い黄色等で強調
- Dependency表示では同一Taskを複数箇所に参照表示してよい
- DBにTaskを複製しない

TaskScore:
- 直接ブロックを強く評価
- 間接ブロックも距離で逓減
- ブロック件数加点も逓減し上限あり

### Dependencyを持つTaskの削除

削除対象Taskが他TaskのDependencyとして参照されている場合:
1. 削除によって影響を受けるTaskを再帰的に算出する
2. Hierarchy上の削除対象子孫も影響範囲へ含める
3. Warning Modalに削除対象Taskを一覧表示する
4. `これらのタスクも削除されます。よろしいですか？` と確認する
5. 承認後、影響範囲全体を同一TransactionでSoft Deleteする
6. キャンセル時は何も変更しない

Dependencyの下流にさらに依存Taskが存在する場合も影響範囲へ再帰的に含める。
同一Taskが複数経路から到達しても削除対象は一意化する。
Hard Deleteは既定の30日保持後に行う。

### Genre移動

SubtreeのGenre変更:
- 対象Task + 全子孫を新Genreへ移動
- 元ParentとのHierarchyを解除
- 対象Taskが新GenreのRootになる
- 子孫のGenreも移動先Genreへ自動変更する

移動対象外TaskとのDependencyがGenre境界を跨ぐ場合:
1. 解除対象Dependencyを事前計算
2. Warning Modalで一覧表示
3. 「Genre移動により以下の依存関係が解除されます」旨を確認
4. ユーザー承認後、境界Dependencyを解除
5. Subtree全体を移動先Genreへ変更

ユーザーがキャンセルした場合、Genre変更もDependency解除も行わない。
処理は同一Transactionで行う。

## 7. 日時 / Gantt

`start_at`, `end_at`: TIMESTAMPTZ nullable。
`is_all_day`: boolean。

時刻指定なし:
- is_all_day = true

時刻指定あり:
- is_all_day = false

両方ある場合 start_at <= end_at。

Gantt:
- バー全体Drag: start/endを同量移動
- 左Resize: startのみ
- 右Resize: endのみ

日付伝播:
- Child.end > Parent.end: Warning後、必要なancestor endをRootまで延長
- Parent.end短縮: Warning後、超過するdescendant endも短縮
- Parent.start後ろ倒し: Warning後、それより前のdescendant startも後ろ倒し
- Child.start < Parent.start: Warning後、必要なancestor startをRootまで前倒し
- 影響範囲を一覧表示し一括承認

Dependency schedule矛盾:
- Warningする
- 保存は許可

## 8. Overdue / Fire

`isOverdue`は算出。

未完了Taskでendを超えたら期限切れ。
COMPLETEDは🔥対象外。

🔥表示:
- 自身が期限切れ
- または未完了子孫に期限切れTaskが1件以上ある
- Tree collapse中でもParentの🔥を表示

期限切れ:
- calculatedPriorityをHIGH方向へ強く補正
- TaskScoreを非常に強く加点

CHECKING + 期限切れ:
- 通常CHECKINGよりTaskScore減点を弱める
- ConfirmationScore側でも浮上させる

## 9. Priority

manualPriority:
- NULL/HIGH/MEDIUM/LOW
- 未指定はNULL

effectivePriority:
`manualPriority ?? calculatedPriority`

calculatedPriority:
- deadline
- progress
- dependency
から内部priorityScore 0..100

閾値:
- 0..34 LOW
- 35..64 MEDIUM
- 65..100 HIGH

期限なしTask:
- calculatedPriority = MEDIUM

ParentにendDateがない:
- 子孫で最も近いendを仮想期限として計算

ParentにstartDateがない:
- 子孫で最も早いstartを仮想startとして表示/計算用途に使用

## 10. TaskScore

0..100の内部値。
UIに数値を表示しない。

基本Weight:
- deadline 30%
- effectivePriority 25%
- dependency 20%
- progress 15%
- taskAge 5%
- confirmation 5%

同点:
- 期限が近い順
- 期限なしは後ろ

おすすめUI:
- 順位
- 決定論的な理由
- AIでランキングを生成しない

要素:
- deadline: 期限なしは中立50
- progress: 低いほど高い。NOT_STARTED(NULL)は0%相当
- effectivePriority: 強く反映
- dependency: 逓減/上限
- taskAge: createdAt基準、古いほど加点、上限
- confirmation:
  - CHECKING中はTaskScoreを大きく減点
  - ただし期限切れ🔥 + CHECKINGでは減点を弱める

未完了DependencyがあるTask:
- TaskScoreを大きく減点
- おすすめ一覧には残す
- 「前提タスクXが未完了」を理由表示

COMPLETED:
- TaskScore対象外
- おすすめ対象外

### Score tuning policy

TaskScore / calculatedPriorityの具体関数・係数・閾値は、Business LogicへMagic Numberとして埋め込まない。

専用の定数/設定モジュールへ分離する。例:

```text
domain/scoring/constants
- taskScoreWeights
- deadlineUrgencyThresholds
- progressUrgencyConstants
- dependencyDecayFactor
- dependencyScoreCap
- taskAgeGrowthRate
- taskAgeScoreCap
- confirmationPenalty
- overdueCheckingPenaltyRelaxation
- calculatedPriorityWeights
- priorityThresholds
```

初期値は既決のWeight・方向性・閾値を満たす妥当な値で実装し、テーブル駆動Testで境界を固定する。
後から定数ファイルだけで調整可能な構造とし、Score計算ロジック自体の書き換えを不要にする。

## 11. Confirmation

Task statusとは別FSM。

Fields:
- content
- target_name
- status
- due_at?
- confirmed_at?

Confirmation FSM:
- UNCONFIRMED <-> CHECKING <-> CONFIRMED
- UNCONFIRMED <-> CONFIRMED 直接禁止

未確認Confirmationが1つでもあるTask:
- COMPLETED禁止

確認先:
- 自由入力
- ユーザー個人のプリセット登録可能
- ユーザー間連携なし

ConfirmationScore:
- due urgency 50%
- status 20%
- parent TaskScore 30%
- CONFIRMEDは対象外
- 期限なし:
  - UNCONFIRMED 固定Score
  - CHECKING はUNCONFIRMEDより高い固定Score
- 期限ありでは期限Urgencyを強く反映

## 12. History

Task status history:
- 永久保持
- via: WEB/MCP/WEB_MCP/SYSTEM/GOOGLE_TASKS

Progress history:
- 専用テーブル
- Leaf progress変更を保存

## 13. Search

初期:
- PostgreSQL ILIKE + pg_trgm

検索対象:
- title
- body
- genre
- priority
- deadline

Search URL state:
- query parameterと同期
- debounce可
- direct URL復元
- browser back/forward対応

## 14. AI Advice

Gemini API。
ランキング決定には使用しない。
PM的Advice生成のみ。

毎日ユーザーTimezone基準04:00頃。
- user全体Advice
- Genre別Advice
を生成。

DB保存。
30日保持。
source_hashで入力状態識別。

## 15. User server settings

DB保存が必要:
- timezone (IANA; 例 Asia/Tokyo)
- google_sync_interval_minutes (default 10)

localStorage:
- UI表示設定等、サーバーのバックグラウンド挙動に影響しないもの

## 16. Google OAuth / Google Tasks

Cloudflare Workers側:
- OAuth
- 認証
- User Context注入

Worker -> Go Container:
- Cloudflare Tunnel / Cloudflare内部経路を利用し、Go ContainerをInternetへ直接公開しない
- WorkerからContainerへの接続はBindingを利用する
- Go APIはBinding経由の内部通信のみを受け付ける構成とする
- Browser/外部ClientからContainerへ直接到達できるPublic Endpointを作らない

Go API:
- Workerが注入した認証済みUser Contextを利用
- 公開Clientが送信した任意User ID/Headerを信頼しない
- User Contextが存在しない/内部経路として検証できないRequestは拒否する
- cross-user resource accessをRepository/UseCase境界でも必ず検証する

Refresh token:
- Neonに暗号化保存
- Encryption keyはCloudflare Secrets
- Browserへ返さない

直接連携:
- Google Tasksのみ
- Calendar APIは直接利用しない

同期:
- title <-> title
- body <-> notes
- endDateの日付部分 <-> due
- completed <-> status

同期しない:
- startDate
- endDateのhh:mm
- genre
- taskType
- hierarchy
- dependency
- arbitrary progress
- priority
- confirmation
- TaskScore

Google Tasks APIの時刻制約:
- タスマネはhh:mm保持
- Googleへはdueの日付のみ
- Googleからdue日付変更時、タスマネの既存hh:mmは保持
- 将来API対応時に時刻同期拡張可能にする

Sync status:
- SYNCED/PENDING/SYNCING/FAILED/CONFLICT

Tasmane -> Google:
- DB保存後即レスポンス
- Cloudflare Queuesで非同期同期

Google -> Tasmane:
- `tasmane.kokage-studio.com/*`アクセス時
- 最終同期から設定間隔以上なら非同期同期
- default 10 min
- user単位lock

Retry:
- 429/5xx: exponential backoff
- 401/403: refresh/permission確認後retry
- permanent 400: no retry -> FAILED
- CONFLICT: no retry
- 最終FAILEDはUI表示 + 手動再同期

Conflict:
- フィールド単位
- 別フィールド変更なら自動merge
- 同一フィールド双方変更ならCONFLICT

Google complete:
- 必ず通常CompleteTask Domain UseCaseを通す
- Domain制約を満たす場合:
  - NOT_STARTEDなら内部的にNOT_STARTED -> IN_PROGRESS -> COMPLETED
  - 履歴2件 / via GOOGLE_TASKS
- 未完了子孫や未確認Confirmation等で完了不可:
  - CONFLICT
  - Domain ruleをバイパスしない

Google uncomplete:
- COMPLETED -> IN_PROGRESS / 95
- 通常Domain ruleを通す
- 必要なCOMPLETED ancestorもRootまでIN_PROGRESS / 95へ戻す

Google delete:
- 自動削除せずCONFLICT
- UIで「タスマネ削除 / タスマネ保持し連携解除 / Google再作成」を選択

Tasmane delete:
- Soft Delete
- Google側も削除
- 30日以内復元ならGoogle側も新規作成し新IDで再紐付け

Google-only Task:
- 自動確定登録しない
- import candidate
- userがGenre選択後に取り込み

Google due変更がHierarchyのendDate伝播を必要とする場合:
- 自動でancestor期限を変更しない
- `CONFLICT`とする
- UIで影響範囲とGoogle側/タスマネ側の値を表示し、ユーザー判断を要求する

Google Task List:
- Google連携開始時にタスマネ専用Task Listを自動作成する
- タスマネが作成・同期するTaskは原則この専用Listへ格納する
- 専用List IDをGoogle integration設定として永続化する
- 再認証時に既存専用Listが利用可能なら再利用する
- 専用ListがGoogle側で削除・利用不能なら同期をFAILED/CONFLICTとして復旧フローへ送る


## 17. MCP / WebMCP

MCP serverを用意。
Cloudflare側にTool登録。

許可:
- read
- create
- update
- status transition
- complete

禁止:
- delete

MCP/REST/WebMCPは同じApplication UseCase/Domainを呼ぶ。
MCPからのcompleteも子孫/Confirmation等Domain ruleを通す。

## 18. UI / Route

主要URL:
```text
/
├ /dashboard
│ └ /dashboard/genres/{genreId}
├ /tasks/new
├ /tasks/{taskId}
├ /genres
└ /search
```

Layout:
- Header
- Sidebar
- Body

Header:
- タスマネLogo
- Top
- Dashboard
- 新規タスク
- ジャンル一覧
- 検索
- Google profile

Sidebar:
- Dashboard
- Genres
- current genre highlight

TaskEditor:
- create/editで共通化

Task詳細:
- 所属Hierarchy全体を取得可能
- collapse可能

別タブ:
- Taskリンクは別タブ表示対応
- GET済み情報はBroadcastChannelで共有
- versionで新旧比較

Fetch:
1. 画面表示に必要なCritical data取得
2. 描画
3. 遷移可能性の高いデータをPrefetch
4. BroadcastChannelで他タブ共有

## 19. Optimistic Lock

`tasks.version BIGINT NOT NULL DEFAULT 1`

更新:
```sql
UPDATE tasks
SET ..., version = version + 1
WHERE id = :id
  AND version = :expected_version;
```

0 rowsならversion conflict。

## 20. 仕様未確定事項の扱い

現時点で既知の仕様BLOCKERはない。
今後実装中に新しい矛盾・未確定事項を発見した場合のみ`state/blocked.md`へ`BLOCKER`として記録し、推測で仕様を確定しない。


# 21. UI / Application 詳細要件

## 21.1 主要画面

```text
/
├── /dashboard
│   └── /dashboard/genres/{genreId}
├── /tasks/new
├── /tasks/{taskId}
├── /genres
└── /search
```

Top `/`:
- Dashboardほど詳細な分析画面にしない
- 日常的にすぐ確認したい情報を中心にする
- 「大量のタスクから次に何をすべきか」を短時間で把握できることを優先する

Dashboard `/dashboard`:
- 全Genre横断
- AI Advice
- おすすめ着手順
- 確認事項 / ConfirmationScore順
- 期限区分テーブル
  - 期限切れ
  - 今日まで
  - 明日まで
  - 1週間以内
  - 1か月以内
- ユーザー操作で優先度順 / TaskScore順 / 期限順へ切替可能

Genre Dashboard `/dashboard/genres/{genreId}`:
- Genre内Task
- Progress
- Deadline
- Priority
- TaskScoreに基づくおすすめ着手順
- Gantt
- AI Advice
- Confirmation一覧

Genre一覧 `/genres`:
- 詳細分析は行わない
- Genre名
- Genreに属するRoot Task
- 各Root Taskの進捗率
を簡潔に表示

Task作成 `/tasks/new`:
- title必須
- Genre必須（1つ）
- body/start/end/manualPriority/parent/taskType等は任意
- Genre選択後、そのGenreのHierarchy TreeからParentを選択可能
- Parent未選択ならRoot
- Genreは検索・新規作成可能
- TaskTypeは当該Genreで利用可能なものから任意で1つ

Task詳細 `/tasks/{taskId}`:
- create/editで同じTaskEditor componentを再利用
- Taskが属するHierarchy Tree全体を取得・表示可能
- 子Task collapse
- Dependency Graph/参照も区別して表示

## 21.2 Header / Sidebar / Layout

共通Layout:
```text
┌─────────────────────────────┐
│ Header                      │
├──────────┬──────────────────┤
│ Sidebar  │ Body             │
└──────────┴──────────────────┘
```

Header左:
- タスマネ SVG Logo
  - 少しポップ
  - 親しみやすい
  - 業務利用でも違和感がない
  - 子供っぽすぎない
  - シンプル

Header右:
- Top
- Dashboard
- 新規タスク
- ジャンル一覧
- 検索
- Google profile icon

Google profile menu:
- ユーザー設定
- ログアウト

Sidebar:
- Dashboard
- Genres
  - Genre一覧
- 現在表示中Genreを必ずhighlight

## 21.3 Search

`/search`

フリーワード:
- title
- body

Genre:
- UI上はGenre filter
- Task自体は1 Genreのみのため複数Genre filter指定時は「いずれかに一致(OR)」として扱う
- 旧仕様の複数Genre ANDはTask=1 Genre化により成立しないため廃止

Priority:
- HIGH / MEDIUM / LOW

Deadline:
- 今日まで
- 明日まで
- 1週間以内
- 1か月以内
- それ以上

検索条件変更でリアルタイム更新。
フリーワードはdebounce可。

URL Query Parameterと同期:
```text
/search?q=google&genres=dev,infra&priority=high&deadline=week
```

直接アクセス / Bookmark / URL共有 / browser back-forwardで状態復元。

検索結果はtable。
Taskリンクは`/tasks/{taskId}`を別タブで開ける。

## 21.4 Page Title

画面ごとにBrowser titleを設定。

例:
```text
トップ | タスマネ
Dashboard | タスマネ
新規タスク | タスマネ
検索 | タスマネ
Task A | タスマネ
Genre A | タスマネ
```

Nuxt SEO/Meta機能を利用。

## 21.5 Nuxt構成

Nuxt Layersを積極利用し責務分離する。

最低限の境界:
- core/domain-facing client
- task
- genre
- dashboard
- search
- google integration
- shared UI

TaskScore等のBusiness LogicはUI componentへ直接実装しない。

## 21.6 Critical Fetch / Prefetch / BroadcastChannel

画面遷移時:
1. 現在画面の表示に必要なCritical dataを最優先取得
2. 取得できた範囲で即描画
3. その画面から遷移しうる可能性の高いデータを後続Prefetch
4. GET済みentityをBroadcastChannelで他タブへ共有
5. `tasks.version`で新旧比較し、古いpayloadで新しいstateを上書きしない

## 21.7 Hierarchy Tree と Dependency Graph の用語

混同禁止。

- `Hierarchy Tree`: `parent_task_id`による分解/包含構造
- `Dependency Graph`: `task_dependencies`による作業前提関係

Dependency GraphをTree風に表示しても、DB上のHierarchyとは別物。

複数依存を持つTask:
- 淡い黄色等でhighlight
- 同一Taskを複数箇所に参照表示可能
- entityは複製しない


# 22. Impact Preview / Confirm Contract

UIでWarning/確認Modalが必要な操作は、Frontendだけで影響範囲を推測しない。
Domain/Application層がImpact Planを返す。

対象:
- Parent/Dependencyを含むTask削除
- Genre移動による境界Dependency解除
- 日付変更によるancestor/descendant日付伝播
- Leaf -> Parent変換
- status出戻りによるancestor/descendant伝播

共通フロー:

```text
1. Preview/通常更新Request
2. Domainが影響なし
   -> そのまま成功

3. Domainが確認必要
   -> HTTP 409 IMPACT_CONFIRMATION_REQUIRED
   -> impactToken + affectedEntities + warnings を返す

4. FrontendがModal表示

5. User承認
   -> 同じ操作を impactToken 付きで再送
   -> Server側でVersion/対象状態を再検証
   -> 同一Transactionで適用

6. Userキャンセル
   -> 再送しない
```

`impactToken`は短寿命かつ操作内容・User・対象Versionに紐付ける。
古いimpactTokenで状態が変化していた場合は再Previewを要求する。

MCP/WebMCP:
- 削除は禁止のためDelete Impactは利用不可
- 削除以外でWarningが必要な更新は、Tool結果として`confirmation_required`とImpact Planを返す
- Agentが勝手にUser承認を代行しない
