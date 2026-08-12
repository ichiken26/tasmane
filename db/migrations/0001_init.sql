BEGIN;

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE task_status AS ENUM ('NOT_STARTED','IN_PROGRESS','COMPLETED');
CREATE TYPE priority_level AS ENUM ('LOW','MEDIUM','HIGH');
CREATE TYPE confirmation_status AS ENUM ('UNCONFIRMED','CHECKING','CONFIRMED');
CREATE TYPE sync_status AS ENUM ('SYNCED','PENDING','SYNCING','FAILED','CONFLICT');
CREATE TYPE change_via AS ENUM ('WEB','MCP','WEB_MCP','SYSTEM','GOOGLE_TASKS');

CREATE TABLE users (
  id uuid PRIMARY KEY,
  google_subject text NOT NULL UNIQUE,
  email citext NOT NULL,
  display_name text,
  avatar_url text,
  timezone text NOT NULL DEFAULT 'Asia/Tokyo',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_server_settings (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  google_sync_interval_minutes integer NOT NULL DEFAULT 10
    CHECK (google_sync_interval_minutes >= 1 AND google_sync_interval_minutes <= 1440),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE genres (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text GENERATED ALWAYS AS (lower(btrim(name))) STORED,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE (id, user_id),
  UNIQUE (user_id, normalized_name)
);

CREATE TABLE task_types (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text GENERATED ALWAYS AS (lower(btrim(name))) STORED,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE (id, user_id),
  UNIQUE (user_id, normalized_name)
);

CREATE TABLE genre_task_types (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  genre_id uuid NOT NULL,
  task_type_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (genre_id, task_type_id),
  FOREIGN KEY (genre_id, user_id) REFERENCES genres(id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (task_type_id, user_id) REFERENCES task_types(id, user_id) ON DELETE CASCADE,
  UNIQUE (genre_id, task_type_id, user_id)
);

CREATE TABLE tasks (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  genre_id uuid NOT NULL,
  task_type_id uuid,
  parent_task_id uuid,
  title text NOT NULL CHECK (length(btrim(title)) > 0),
  body text,
  status task_status NOT NULL DEFAULT 'NOT_STARTED',
  start_at timestamptz,
  end_at timestamptz,
  is_all_day boolean NOT NULL DEFAULT true,
  leaf_progress smallint,
  manual_priority priority_level,
  version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  UNIQUE (id, user_id),
  UNIQUE (id, genre_id, user_id),

  FOREIGN KEY (genre_id, user_id)
    REFERENCES genres(id, user_id) ON DELETE RESTRICT,

  FOREIGN KEY (parent_task_id, genre_id, user_id)
    REFERENCES tasks(id, genre_id, user_id) ON DELETE RESTRICT,

  FOREIGN KEY (genre_id, task_type_id, user_id)
    REFERENCES genre_task_types(genre_id, task_type_id, user_id)
    ON DELETE RESTRICT,

  CHECK (parent_task_id IS NULL OR parent_task_id <> id),
  CHECK (start_at IS NULL OR end_at IS NULL OR start_at <= end_at),

  CHECK (
    (status = 'NOT_STARTED' AND leaf_progress IS NULL)
    OR
    (status = 'IN_PROGRESS' AND (
      leaf_progress IS NULL
      OR (leaf_progress BETWEEN 0 AND 95 AND leaf_progress % 5 = 0)
    ))
    OR
    (status = 'COMPLETED' AND (
      leaf_progress IS NULL
      OR leaf_progress = 100
    ))
  )
);

-- leaf_progress is authoritative ONLY while a task is a Leaf.
-- Parent progress is always derived and leaf_progress must be NULL for Parent tasks.
-- Whether the task is currently a Leaf depends on child existence, therefore the
-- remaining Leaf-vs-Parent invariant is enforced transactionally in the Domain layer.

-- NOTE:
-- Parent progress is calculated, but tasks.leaf_progress must not be treated
-- as authoritative for parent nodes by application code. Parent status is persisted.
-- If desired, a future migration can split leaf_progress from a generic task table.

CREATE TABLE task_dependencies (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  genre_id uuid NOT NULL,
  task_id uuid NOT NULL,
  depends_on_task_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (task_id, depends_on_task_id),
  FOREIGN KEY (task_id, genre_id, user_id)
    REFERENCES tasks(id, genre_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (depends_on_task_id, genre_id, user_id)
    REFERENCES tasks(id, genre_id, user_id) ON DELETE RESTRICT,
  CHECK (task_id <> depends_on_task_id)
);

CREATE TABLE confirmation_targets (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text GENERATED ALWAYS AS (lower(btrim(name))) STORED,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, normalized_name)
);

CREATE TABLE task_confirmations (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid NOT NULL,
  content text NOT NULL CHECK (length(btrim(content)) > 0),
  target_name text NOT NULL CHECK (length(btrim(target_name)) > 0),
  status confirmation_status NOT NULL DEFAULT 'UNCONFIRMED',
  due_at timestamptz,
  confirmed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (task_id, user_id)
    REFERENCES tasks(id, user_id) ON DELETE CASCADE,
  CHECK (
    (status = 'CONFIRMED' AND confirmed_at IS NOT NULL)
    OR
    (status <> 'CONFIRMED' AND confirmed_at IS NULL)
  )
);

CREATE TABLE task_status_histories (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid NOT NULL,
  from_status task_status,
  to_status task_status NOT NULL,
  changed_by_user_id uuid,
  changed_via change_via NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (task_id, user_id)
    REFERENCES tasks(id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (changed_by_user_id)
    REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE task_progress_histories (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid NOT NULL,
  from_progress smallint,
  to_progress smallint,
  changed_by_user_id uuid,
  changed_via change_via NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (task_id, user_id)
    REFERENCES tasks(id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (changed_by_user_id)
    REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE ai_advices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  genre_id uuid,
  content text NOT NULL,
  source_hash text NOT NULL,
  generated_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (genre_id, user_id)
    REFERENCES genres(id, user_id) ON DELETE CASCADE
);

CREATE TABLE google_integrations (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  google_subject text NOT NULL,
  encrypted_refresh_token text NOT NULL,
  google_task_list_id text,
  token_expires_at timestamptz,
  scopes text[] NOT NULL DEFAULT '{}',
  last_sync_started_at timestamptz,
  last_sync_finished_at timestamptz,
  sync_lock_until timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE google_task_syncs (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id uuid NOT NULL,
  google_task_list_id text NOT NULL,
  google_task_id text NOT NULL,
  last_synced_at timestamptz,
  google_updated_at timestamptz,
  status sync_status NOT NULL DEFAULT 'PENDING',
  last_synced_snapshot jsonb,
  last_error_code text,
  last_error_message text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, task_id),
  UNIQUE (user_id, google_task_list_id, google_task_id),
  FOREIGN KEY (task_id, user_id)
    REFERENCES tasks(id, user_id) ON DELETE CASCADE
);

CREATE INDEX idx_tasks_user_status ON tasks(user_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_user_end ON tasks(user_id, end_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_parent ON tasks(parent_task_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_genre ON tasks(user_id, genre_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_title_trgm ON tasks USING gin (title gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX idx_confirmations_task_status ON task_confirmations(task_id, status);
CREATE INDEX idx_status_hist_task_created ON task_status_histories(task_id, created_at DESC);
CREATE INDEX idx_progress_hist_task_created ON task_progress_histories(task_id, created_at DESC);
CREATE INDEX idx_google_sync_status ON google_task_syncs(user_id, status);

COMMIT;
