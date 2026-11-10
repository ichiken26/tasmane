BEGIN;
-- 専用 Task List が Google 側で削除・利用不能になった時刻（復旧フロー表示用）

ALTER TABLE google_integrations ADD COLUMN task_list_missing_at timestamptz;

COMMIT;
