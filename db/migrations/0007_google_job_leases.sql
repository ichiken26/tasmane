BEGIN;
-- Google 同期ジョブのリース列と未クレーム索引の追加

-- クレーム列の追加
ALTER TABLE google_sync_jobs
  ADD COLUMN claimed_at timestamptz,
  ADD COLUMN claimed_by text;

-- 未クレームジョブ用部分インデックスの定義
CREATE INDEX idx_google_sync_jobs_claimable
  ON google_sync_jobs(available_at, created_at)
  WHERE claimed_at IS NULL;

COMMIT;
