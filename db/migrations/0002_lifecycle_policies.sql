BEGIN;
-- 更新日時トリガと UUIDv7 制約の追加

-- updated_at 自動更新関数の定義
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = clock_timestamp();
  RETURN NEW;
END;
$$;

-- 対象テーブルへの更新トリガ付与
DO $$
DECLARE
  table_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'users', 'user_server_settings', 'genres', 'task_types', 'tasks',
    'confirmation_targets', 'task_confirmations', 'google_integrations',
    'google_task_syncs'
  ]
  LOOP
    EXECUTE format(
      'CREATE TRIGGER %I_set_updated_at BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
      table_name, table_name
    );
  END LOOP;
END;
$$;

-- 主キーの UUIDv7 版ビット制約
ALTER TABLE users ADD CONSTRAINT users_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE genres ADD CONSTRAINT genres_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE task_types ADD CONSTRAINT task_types_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE tasks ADD CONSTRAINT tasks_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE confirmation_targets ADD CONSTRAINT confirmation_targets_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE task_confirmations ADD CONSTRAINT task_confirmations_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE task_status_histories ADD CONSTRAINT task_status_histories_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE task_progress_histories ADD CONSTRAINT task_progress_histories_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE ai_advices ADD CONSTRAINT ai_advices_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE google_integrations ADD CONSTRAINT google_integrations_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');
ALTER TABLE google_task_syncs ADD CONSTRAINT google_task_syncs_id_uuid_v7 CHECK (substring(id::text, 15, 1) = '7');

COMMIT;
