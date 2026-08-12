# AGENTS.md — タスマネ開発ルール

## 最優先原則

1. `spec/PRODUCT_SPEC.md` を正本とする。
2. TDDを必須とする。仕様変更をコード先行で行わない。
3. 1ループで扱う変更を小さく保つ。
4. Red → Green → Refactor の順を守る。
5. テストを削除・skip・弱体化してGreenにしない。
6. 未確定事項を推測で確定しない。`state/blocked.md`へ記録する。
7. DB migrationは追加方式。既存migrationを書き換えない。
8. Domain ruleはHTTP/MCP/Google同期などTransportごとに複製せず、共通UseCase/Domainへ集約する。
9. Google同期でもDomain ruleをバイパスしない。
10. MCP/WebMCPからDELETEを公開しない。

## 作業順序

1. `state/implementation-checklist.md` を読む
2. 最上位の未完了タスクを1つ選ぶ
3. 関係する仕様を読む
4. failing testを書く
5. 最小実装
6. unit test
7. integration test
8. lint/typecheck
9. refactor
10. checklist更新
11. `state/progress.md`へ記録

## 完了禁止条件

以下のいずれかなら「完了」とみなさない。

- failing testあり
- TODO/FIXMEで必須仕様を先送り
- Domain ruleがTransport層だけに存在
- DB制約で担保可能な整合性が無制約
- Google同期が子タスク/Confirmation完了制約を迂回
- OpenAPI/MCP schemaと実装不一致
