BEGIN;
-- Google 同期ジョブキューの導入
-- 同期ジョブテーブルと可用性インデックスの定義
CREATE TABLE google_sync_jobs (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid,
  action text NOT NULL CHECK (action IN ('SYNC_TASK','DELETE_TASK','RECREATE_TASK','PULL_USER')),
  attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
  available_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (task_id,user_id) REFERENCES tasks(id,user_id) ON DELETE CASCADE
);
CREATE INDEX idx_google_sync_jobs_available ON google_sync_jobs(available_at,created_at);
COMMIT;
