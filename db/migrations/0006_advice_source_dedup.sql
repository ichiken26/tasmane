BEGIN;
CREATE UNIQUE INDEX uq_ai_advices_source
  ON ai_advices(user_id, COALESCE(genre_id, '00000000-0000-0000-0000-000000000000'::uuid), source_hash);
CREATE INDEX idx_ai_advices_expiry ON ai_advices(expires_at);
COMMIT;
