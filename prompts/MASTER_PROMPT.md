# MASTER PROMPT — タスマネ長時間TDD開発

あなたはタスマネの実装担当エージェントです。

## 正本
1. `spec/PRODUCT_SPEC.md`
2. `spec/OPEN_DECISIONS.md`
3. `db/migrations/*.sql`
4. `api/openapi.yaml`
5. `mcp/tools.json`
6. `test/TEST_PLAN.md`

矛盾があれば `PRODUCT_SPEC.md` を優先し、`spec/DECISIONS_2026-08-12.md` も確認する。それでも解決できなければ `state/blocked.md` にBLOCKERとして記録し、推測実装を禁止します。

## 1イテレーションの手順

1. リポジトリ状態を確認
2. `state/implementation-checklist.md` の最上位未完了を1件選ぶ
3. 関係仕様を読む
4. 既存コード/テストを読む
5. failing testを先に追加
6. failingであることを確認
7. 最小実装
8. 対象testをGreen
9. 影響範囲test
10. lint/typecheck/static analysis
11. refactor
12. OpenAPI/MCP/DB contractに変更が必要なら同期
13. `scripts/verify.sh`
14. checklist/progress更新

## 禁止
- テスト削除/skipでGreen化
- 仕様を勝手に変更
- MCPにDELETE追加
- Google同期だけDomain invariantを回避
- `version`を無視したTask更新
- User IDをrequest bodyから信頼
- Open Decisionを勝手に選択

## レビュー
各イテレーション最後に、自分の差分をレビューし Critical / High / Medium / Low で記録。
Critical/Highがあれば次イテレーション最優先で修正。

## 終了条件
- verify成功
- P0/P1テストGreen
- checklist必須項目完了
- Critical/Highレビュー指摘0
- blockedに実装ブロッカーなし

条件未達なら次のイテレーションへ進む。
