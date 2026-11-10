BEGIN;
-- Google 同期コンフリクト解決用の列と連携解除テーブルの追加

-- コンフリクト時点の Google 側値（UI で双方の値を提示するため）
ALTER TABLE google_task_syncs ADD COLUMN conflict_remote_snapshot jsonb;

-- 「タスマネ保持し連携解除」したタスク（以後 Google 同期の対象外）
CREATE TABLE google_unlinked_tasks (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, task_id),
  FOREIGN KEY (task_id, user_id) REFERENCES tasks(id, user_id) ON DELETE CASCADE
);

COMMIT;
