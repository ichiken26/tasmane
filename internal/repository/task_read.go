package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tasmane/internal/domain"
)

const taskReadColumns = `task.id, task.user_id, task.genre_id, task.task_type_id,
  task.parent_task_id, task.title, task.body, task.status, task.leaf_progress,
  task.start_at, task.end_at, task.is_all_day, task.manual_priority, task.version, task.created_at,
  EXISTS (SELECT 1 FROM tasks child WHERE child.parent_task_id=task.id
          AND child.user_id=task.user_id AND child.deleted_at IS NULL) AS has_children`

// タスク検索
func (p *Postgres) Search(ctx context.Context, userID string, search domain.TaskSearch) ([]domain.Task, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, domain.ErrForbidden
	}
	query := `SELECT ` + taskReadColumns + ` FROM tasks task WHERE task.user_id=$1 AND task.deleted_at IS NULL`
	args := []any{userID}
	if value := strings.TrimSpace(search.Query); value != "" {
		args = append(args, "%"+value+"%")
		query += fmt.Sprintf(` AND (task.title ILIKE $%d OR COALESCE(task.body,'') ILIKE $%d)`, len(args), len(args))
	}
	if len(search.GenreIDs) > 0 {
		args = append(args, search.GenreIDs)
		query += fmt.Sprintf(` AND task.genre_id = ANY($%d::uuid[])`, len(args))
	}
	if search.Priority != nil {
		args = append(args, *search.Priority)
		query += fmt.Sprintf(` AND COALESCE(task.manual_priority, 'MEDIUM'::priority_level)=$%d`, len(args))
	}
	if search.Deadline != "" {
		boundary, ok := deadlineBoundary(search.Deadline, time.Now().UTC())
		if !ok {
			return nil, domain.ErrInvalidTaskInput
		}
		args = append(args, boundary)
		switch search.Deadline {
		case "later":
			query += fmt.Sprintf(` AND task.end_at >= $%d`, len(args))
		case "overdue":
			// 期限切れは未完了タスクのみ（isOverdueと同じ定義）
			query += fmt.Sprintf(` AND task.end_at < $%d AND task.status <> 'COMPLETED'::task_status`, len(args))
		default:
			query += fmt.Sprintf(` AND task.end_at < $%d`, len(args))
		}
	}
	query += ` ORDER BY task.end_at NULLS LAST, task.id`
	// 検索SQLの実行
	rows, err := p.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search tasks: %w", err)
	}
	defer rows.Close()
	return scanTaskRows(rows)
}

// タスク木の取得
func (p *Postgres) Tree(ctx context.Context, userID, taskID string) ([]domain.Task, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	// 祖先・子孫ツリーSQLの実行
	rows, err := p.database.QueryContext(ctx, `WITH RECURSIVE ancestors AS (
    SELECT task.id, task.parent_task_id FROM tasks task
    WHERE task.id=$1 AND task.user_id=$2 AND task.deleted_at IS NULL
    UNION ALL
    SELECT parent.id, parent.parent_task_id FROM tasks parent
    JOIN ancestors child ON child.parent_task_id=parent.id
    WHERE parent.user_id=$2 AND parent.deleted_at IS NULL
  ), root AS (
    SELECT id FROM ancestors WHERE parent_task_id IS NULL
  ), tree AS (
    SELECT task.id FROM tasks task JOIN root ON task.id=root.id
    UNION ALL
    SELECT child.id FROM tasks child JOIN tree parent ON child.parent_task_id=parent.id
    WHERE child.user_id=$2 AND child.deleted_at IS NULL
  )
  SELECT `+taskReadColumns+` FROM tasks task JOIN tree ON tree.id=task.id
  WHERE task.user_id=$2 ORDER BY task.id`, taskID, userID)
	if err != nil {
		return nil, fmt.Errorf("get task tree: %w", err)
	}
	defer rows.Close()
	tasks, err := scanTaskRows(rows)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, domain.ErrForbidden
	}
	return tasks, nil
}

// タスク行のスキャン
func scanTaskRows(rows *sql.Rows) ([]domain.Task, error) {
	tasks := []domain.Task{}
	for rows.Next() {
		var task domain.Task
		var taskTypeID, parentTaskID, body, priority sql.NullString
		var status string
		var progress sql.NullInt64
		var startAt, endAt sql.NullTime
		// 1行のスキャン
		if err := rows.Scan(&task.ID, &task.UserID, &task.GenreID, &taskTypeID, &parentTaskID,
			&task.Title, &body, &status, &progress, &startAt, &endAt, &task.IsAllDay,
			&priority, &task.Version, &task.CreatedAt, &task.HasChildren); err != nil {
			return nil, fmt.Errorf("scan task read model: %w", err)
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
		if priority.Valid {
			value := domain.Priority(priority.String)
			task.ManualPriority = &value
		}
		tasks = append(tasks, task.ForPersistence())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read task rows: %w", err)
	}
	return tasks, nil
}

// 期限境界の算出
func deadlineBoundary(value string, now time.Time) (time.Time, bool) {
	switch value {
	case "overdue":
		return now, true
	case "today":
		return now.Add(24 * time.Hour), true
	case "tomorrow":
		return now.Add(48 * time.Hour), true
	case "week":
		return now.Add(7 * 24 * time.Hour), true
	case "month":
		return now.Add(30 * 24 * time.Hour), true
	case "later":
		return now.Add(30 * 24 * time.Hour), true
	default:
		return time.Time{}, false
	}
}
