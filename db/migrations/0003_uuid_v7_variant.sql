BEGIN;

DO $$
DECLARE
  table_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'users', 'genres', 'task_types', 'tasks', 'confirmation_targets',
    'task_confirmations', 'task_status_histories', 'task_progress_histories',
    'ai_advices', 'google_integrations', 'google_task_syncs'
  ]
  LOOP
    EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', table_name, table_name || '_id_uuid_v7');
    EXECUTE format(
      'ALTER TABLE %I ADD CONSTRAINT %I CHECK (substring(id::text, 15, 1) = ''7'' AND substring(id::text, 20, 1) IN (''8'', ''9'', ''a'', ''b''))',
      table_name,
      table_name || '_id_uuid_v7'
    );
  END LOOP;
END;
$$;

COMMIT;
