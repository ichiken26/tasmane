package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tasmane/internal/domain"
)

func (p *Postgres) Update(ctx context.Context, update domain.TaskUpdate, expected int64) (domain.Task, error) {
	task := update.Task
	if p == nil || p.database == nil || task.ID == "" || task.UserID == "" || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin task patch: %w", err)
	}
	defer tx.Rollback()
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, task.ID, task.UserID).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrForbidden
	} else if err != nil {
		return domain.Task{}, fmt.Errorf("lock task patch: %w", err)
	}
	if version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	for _, change := range update.DateChanges {
		if change.TaskID == task.ID {
			continue
		}
		result, err := tx.ExecContext(ctx, `UPDATE tasks SET start_at=$1,end_at=$2,version=version+1
      WHERE id=$3 AND user_id=$4 AND deleted_at IS NULL
        AND start_at IS NOT DISTINCT FROM $5 AND end_at IS NOT DISTINCT FROM $6`,
			change.After.StartAt, change.After.EndAt, change.TaskID, task.UserID, change.Before.StartAt, change.Before.EndAt)
		if err != nil {
			return domain.Task{}, fmt.Errorf("propagate task dates: %w", err)
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return domain.Task{}, domain.ErrVersionConflict
		}
	}
	var taskType, parent, body, priority any
	if task.TaskTypeID != nil {
		taskType = *task.TaskTypeID
	}
	if task.ParentTaskID != nil {
		parent = *task.ParentTaskID
	}
	if task.Body != nil {
		body = *task.Body
	}
	if task.ManualPriority != nil {
		priority = *task.ManualPriority
	}
	err = tx.QueryRowContext(ctx, `UPDATE tasks SET genre_id=$1,task_type_id=$2,parent_task_id=$3,title=$4,body=$5,start_at=$6,end_at=$7,is_all_day=$8,manual_priority=$9,version=version+1 WHERE id=$10 AND user_id=$11 AND version=$12 AND deleted_at IS NULL RETURNING version`,
		task.GenreID, taskType, parent, task.Title, body, task.StartAt, task.EndAt, task.IsAllDay, priority, task.ID, task.UserID, expected).Scan(&task.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrVersionConflict
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("patch task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit task patch: %w", err)
	}
	return task.ForPersistence(), nil
}
