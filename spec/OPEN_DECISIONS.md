# OPEN DECISIONS

2026-08-12の追加ヒアリングにより、OD-001 / OD-002 / OD-003 / OD-005 / OD-006 / OD-007 / OD-008は解決済み。

## Resolved

- OD-001: Genre移動時、境界DependencyをWarning後に解除。Subtree全体のGenreを自動変更。
- OD-002: Google due変更がParent期限伝播を必要とする場合はCONFLICT。
- OD-003: タスマネ専用Google Task Listを自動作成。
- OD-005: Score係数・関数用定数を専用モジュールへ分離し、後から定数調整可能にする。
- OD-006: 最初のChild追加時に進捗再計算確認。COMPLETED TaskへのChild追加は禁止。
- OD-007: Dependency削除影響範囲を一覧確認し、承認後に影響TaskをSoft Delete。
- OD-008: Worker -> Go ContainerはCloudflare内部経路/Tunnel + Binding。Containerを直接外部公開しない。

## Non-blocking implementation note

### all-day精度

初期実装:
- `is_all_day=true`: start/endとも日付単位
- `is_all_day=false`: start/endとも日時単位

将来、start/end個別precisionが必要になった場合はmigrationで拡張する。
