BEGIN;
-- Google 未紐付けインポート候補のための task_id 任意化

-- google_task_syncs.task_id の NOT NULL 解除
ALTER TABLE google_task_syncs ALTER COLUMN task_id DROP NOT NULL;

COMMIT;
