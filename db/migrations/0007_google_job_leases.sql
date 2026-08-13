BEGIN;

ALTER TABLE google_sync_jobs
  ADD COLUMN claimed_at timestamptz,
  ADD COLUMN claimed_by text;

CREATE INDEX idx_google_sync_jobs_claimable
  ON google_sync_jobs(available_at, created_at)
  WHERE claimed_at IS NULL;

COMMIT;
