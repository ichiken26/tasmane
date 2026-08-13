package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tasmane/internal/domain"
)

func (p *Postgres) Create(ctx context.Context, userID string, draft domain.TaskDraft, authorization domain.TaskCreateAuthorization) (domain.Task, error) {
	if p == nil || p.database == nil || p.ids == nil || userID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return domain.Task{}, err
	}
	id, err := p.ids.New()
	if err != nil {
		return domain.Task{}, fmt.Errorf("generate task id: %w", err)
	}
	task, err := domain.NewTask(id, userID, draft)
	if err != nil {
		return domain.Task{}, err
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin task create: %w", err)
	}
	defer tx.Rollback()
	var genreID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM genres
    WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR SHARE`, draft.GenreID, userID).Scan(&genreID); errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrForbidden
	} else if err != nil {
		return domain.Task{}, fmt.Errorf("lock task genre: %w", err)
	}
	if draft.TaskTypeID != nil {
		var allowed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
      SELECT 1 FROM genre_task_types link
      JOIN task_types task_type ON task_type.id=link.task_type_id AND task_type.user_id=link.user_id
      WHERE link.user_id=$1 AND link.genre_id=$2 AND link.task_type_id=$3 AND task_type.deleted_at IS NULL
    )`, userID, draft.GenreID, *draft.TaskTypeID).Scan(&allowed); err != nil {
			return domain.Task{}, fmt.Errorf("check task type assignment: %w", err)
		}
		if !allowed {
			return domain.Task{}, domain.ErrGraphGenre
		}
	}
	if draft.ParentTaskID != nil {
		if err := lockUserGraph(ctx, tx, userID); err != nil {
			return domain.Task{}, err
		}
		var parentStatus string
		var parentGenre string
		var parentVersion int64
		var hasChildren bool
		err := tx.QueryRowContext(ctx, `SELECT parent.status, parent.genre_id, parent.version,
      EXISTS (SELECT 1 FROM tasks child WHERE child.parent_task_id=parent.id AND child.user_id=parent.user_id AND child.deleted_at IS NULL)
      FROM tasks parent WHERE parent.id=$1 AND parent.user_id=$2 AND parent.deleted_at IS NULL FOR UPDATE OF parent`,
			*draft.ParentTaskID, userID).Scan(&parentStatus, &parentGenre, &parentVersion, &hasChildren)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrForbidden
		}
		if err != nil {
			return domain.Task{}, fmt.Errorf("lock parent for child creation: %w", err)
		}
		if parentGenre != draft.GenreID {
			return domain.Task{}, domain.ErrGraphGenre
		}
		if domain.Status(parentStatus) == domain.Completed {
			return domain.Task{}, domain.ErrNotCompletable
		}
		if !hasChildren {
			if !authorization.AllowLeafConversion {
				return domain.Task{}, domain.ErrImpactConfirmationRequired
			}
			if authorization.ParentVersion != parentVersion {
				return domain.Task{}, domain.ErrVersionConflict
			}
			if _, err := tx.ExecContext(ctx, `UPDATE tasks SET leaf_progress=NULL, version=version+1
        WHERE id=$1 AND user_id=$2 AND version=$3 AND deleted_at IS NULL`, *draft.ParentTaskID, userID, parentVersion); err != nil {
				return domain.Task{}, fmt.Errorf("convert leaf parent: %w", err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tasks
    (id, user_id, genre_id, task_type_id, parent_task_id, title, body, status,
     start_at, end_at, is_all_day, leaf_progress, manual_priority, version)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULL,$12,1)`,
		task.ID, task.UserID, task.GenreID, task.TaskTypeID, task.ParentTaskID,
		task.Title, task.Body, task.Status, task.StartAt, task.EndAt, task.IsAllDay, task.ManualPriority); err != nil {
		return domain.Task{}, fmt.Errorf("insert task: %w", err)
	}
	if err := p.enqueueGoogleTaskTx(ctx, tx, userID, task.ID, domain.GoogleSyncTask); err != nil {
		return domain.Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit task create: %w", err)
	}
	return task, nil
}
