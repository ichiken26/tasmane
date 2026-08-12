package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tasmane/internal/domain"
	"tasmane/internal/identifier"
)

type IDGenerator interface{ New() (string, error) }

type Postgres struct {
	database *sql.DB
	ids      IDGenerator
}

func NewPostgres(database *sql.DB, ids IDGenerator) *Postgres {
	if ids == nil {
		ids = identifier.NewUUIDv7Generator()
	}
	return &Postgres{database: database, ids: ids}
}

const taskGetSQL = `WITH RECURSIVE descendants AS (
  SELECT child.id, child.status
  FROM tasks child
  WHERE child.parent_task_id = $1 AND child.user_id = $2 AND child.deleted_at IS NULL
  UNION ALL
  SELECT child.id, child.status
  FROM tasks child
  JOIN descendants parent ON child.parent_task_id = parent.id
  WHERE child.user_id = $2 AND child.deleted_at IS NULL
)
SELECT t.id, t.user_id, t.genre_id, t.task_type_id, t.parent_task_id,
       t.title, t.body, t.status, t.leaf_progress, t.start_at, t.end_at,
       t.is_all_day, t.manual_priority, t.version,
       EXISTS (
         SELECT 1 FROM tasks child
         WHERE child.parent_task_id = t.id AND child.user_id = t.user_id AND child.deleted_at IS NULL
       ) AS has_children,
       (SELECT count(*) FROM descendants WHERE status <> 'COMPLETED') AS incomplete_descendants,
       (SELECT count(*) FROM task_confirmations confirmation
        WHERE confirmation.task_id = t.id AND confirmation.user_id = t.user_id
          AND confirmation.status <> 'CONFIRMED') AS unresolved_confirmations
FROM tasks t
WHERE t.id = $1 AND t.user_id = $2 AND t.deleted_at IS NULL`

func (p *Postgres) Get(ctx context.Context, userID, taskID string) (domain.Task, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	var task domain.Task
	var status string
	var progress sql.NullInt64
	var taskTypeID, parentTaskID, body, manualPriority sql.NullString
	var startAt, endAt sql.NullTime
	err := p.database.QueryRowContext(ctx, taskGetSQL, taskID, userID).Scan(
		&task.ID, &task.UserID, &task.GenreID, &taskTypeID, &parentTaskID,
		&task.Title, &body, &status, &progress, &startAt, &endAt,
		&task.IsAllDay, &manualPriority, &task.Version, &task.HasChildren,
		&task.IncompleteDescendants, &task.UnresolvedConfirmations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("get task: %w", err)
	}
	task.Status = domain.Status(status)
	if taskTypeID.Valid {
		task.TaskTypeID = &taskTypeID.String
	}
	if parentTaskID.Valid {
		task.ParentTaskID = &parentTaskID.String
	}
	if body.Valid {
		task.Body = &body.String
	}
	if progress.Valid {
		value := int(progress.Int64)
		task.Progress = &value
	}
	if startAt.Valid {
		task.StartAt = &startAt.Time
	}
	if endAt.Valid {
		task.EndAt = &endAt.Time
	}
	if manualPriority.Valid {
		value := domain.Priority(manualPriority.String)
		task.ManualPriority = &value
	}
	return task.ForPersistence(), nil
}

const taskForUpdateSQL = `SELECT t.status, t.leaf_progress, t.version,
       EXISTS (
         SELECT 1 FROM tasks child
         WHERE child.parent_task_id = t.id AND child.user_id = t.user_id AND child.deleted_at IS NULL
       ) AS has_children
FROM tasks t
WHERE t.id = $1 AND t.user_id = $2 AND t.deleted_at IS NULL
FOR UPDATE OF t`

const taskUpdateSQL = `UPDATE tasks
SET status = $1, leaf_progress = $2, version = version + 1
WHERE id = $3 AND user_id = $4 AND version = $5 AND deleted_at IS NULL
RETURNING version`

func (p *Postgres) UpdateStatus(ctx context.Context, task domain.Task, expected int64, via string) (domain.Task, error) {
	if p == nil || p.database == nil || p.ids == nil || task.ID == "" || task.UserID == "" || expected < 1 || !validChangeVia(via) {
		return domain.Task{}, domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin task update: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	var currentProgress sql.NullInt64
	var currentVersion int64
	var hasChildren bool
	err = tx.QueryRowContext(ctx, taskForUpdateSQL, task.ID, task.UserID).
		Scan(&currentStatus, &currentProgress, &currentVersion, &hasChildren)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("lock task: %w", err)
	}
	if currentVersion != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}

	task.HasChildren = hasChildren
	task = task.ForPersistence()
	var progress any
	if task.Progress != nil {
		progress = *task.Progress
	}
	if err := tx.QueryRowContext(ctx, taskUpdateSQL, task.Status, progress, task.ID, task.UserID, expected).Scan(&task.Version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrVersionConflict
		}
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	if domain.Status(currentStatus) != task.Status {
		if err := p.insertStatusHistory(ctx, tx, task, domain.Status(currentStatus), via); err != nil {
			return domain.Task{}, err
		}
	}
	if !sameNullableProgress(currentProgress, task.Progress) {
		if err := p.insertProgressHistory(ctx, tx, task, currentProgress, via); err != nil {
			return domain.Task{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit task update: %w", err)
	}
	return task, nil
}

func (p *Postgres) insertStatusHistory(ctx context.Context, tx *sql.Tx, task domain.Task, from domain.Status, via string) error {
	id, err := p.ids.New()
	if err != nil {
		return fmt.Errorf("generate status history id: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO task_status_histories
    (id, user_id, task_id, from_status, to_status, changed_by_user_id, changed_via)
    VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, task.UserID, task.ID, from, task.Status, task.UserID, via)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}

func (p *Postgres) insertProgressHistory(ctx context.Context, tx *sql.Tx, task domain.Task, from sql.NullInt64, via string) error {
	id, err := p.ids.New()
	if err != nil {
		return fmt.Errorf("generate progress history id: %w", err)
	}
	var fromValue, toValue any
	if from.Valid {
		fromValue = int(from.Int64)
	}
	if task.Progress != nil {
		toValue = *task.Progress
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO task_progress_histories
    (id, user_id, task_id, from_progress, to_progress, changed_by_user_id, changed_via)
    VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, task.UserID, task.ID, fromValue, toValue, task.UserID, via)
	if err != nil {
		return fmt.Errorf("insert progress history: %w", err)
	}
	return nil
}

func sameNullableProgress(current sql.NullInt64, next *int) bool {
	if !current.Valid || next == nil {
		return !current.Valid && next == nil
	}
	return int(current.Int64) == *next
}

func validChangeVia(value string) bool {
	switch value {
	case "WEB", "MCP", "WEB_MCP", "SYSTEM", "GOOGLE_TASKS":
		return true
	default:
		return false
	}
}
