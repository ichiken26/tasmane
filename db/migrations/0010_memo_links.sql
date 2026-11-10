BEGIN;
-- メモアプリ連携: ユーザーごとの連携有無とタスクに紐づくメモリンク

ALTER TABLE user_server_settings ADD COLUMN memo_integration_enabled boolean NOT NULL DEFAULT false;

CREATE TABLE task_memo_links (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
  title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (task_id, url)
);

CREATE INDEX task_memo_links_user_task_idx ON task_memo_links (user_id, task_id, created_at);

COMMIT;
