# Historical Review — superseded

> この文書はv2作成時点の履歴。未確定事項一覧はすべて後続ヒアリングで解決済み。 現在の正本は `spec/PRODUCT_SPEC.md` と `spec/DECISIONS_2026-08-12.md`。

# タスマネ TDD Harness — 2nd Review

## 結論

初版ハーネスは方向性は正しいが、「全要件を完全収録」「終了条件を機械的に保証」という観点では不足があった。
このv2では、確認できた実装矛盾・抜け・完了ゲート不備を修正した。

## Critical 修正

1. `leaf_progress` DB CHECKがParentにもprogress永続化を強制していた
   - Parent statusは保存
   - Parent progressは常に算出
   - Parent `leaf_progress`はNULL
   - Leaf/Parent判別はchild存在に依存するため残りはDomain invariant

2. TDD loopが最終レビューPASSを実際には終了条件として検査していなかった
   - `state/review.md`
   - `REVIEW_STATUS: PASS`
   - Critical/High=0
   を機械的Completion Gateに追加

3. 既知Open DecisionがCompletion Gateを止めていなかった
   - 実装をブロックするODを`BLOCKER`として明示

## High 修正

4. 初期ハーネスでUI/Application仕様の一部が落ちていた
   - Dashboard期限区分
   - Genre Dashboard
   - `/genres`
   - TaskEditor
   - Parent picker
   - Header/Sidebar/Logo
   - Search URL state
   - Page title
   - Nuxt Layers
   - Critical Fetch / Prefetch / BroadcastChannel
   をPRODUCT_SPECへ復元

5. Hierarchy Tree / Dependency Graphの用語を明確に分離

6. 認証Context完全性、Dependency付き削除、Leaf<->Parent変換、Score具体式をOpen Decisionとして追加

## まだ未確定（ユーザー判断が必要）

- OD-001 Genre移動と境界Dependency
- OD-002 Google due変更とParent期限伝播
- OD-003 Google Task List方式
- OD-005 TaskScore/calculatedPriorityの具体関数
- OD-006 Leaf<->Parent化時のstatus/progress
- OD-007 Dependencyを持つTaskの削除
- OD-008 Worker->Go認証Contextの具体保証方式

OD-004 all-day精度は初期ルールで実装可能なため非ブロッキング。

## 注意

OpenAPIとMCP Tool Schemaは「完全な最終契約」ではなく、実装開始用の骨格。
全画面/全UseCaseが固まった後にEndpoint/Schema completeness reviewを必須とする。
