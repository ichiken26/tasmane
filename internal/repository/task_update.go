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
	// Every hierarchy/dependency-sensitive update is serialized per user. This
	// also makes the impact plan recomputation below authoritative at commit time.
	if err := lockUserGraph(ctx, tx, task.UserID); err != nil {
		return domain.Task{}, err
	}
	tasks, dependencies, err := loadGraph(ctx, tx, task.UserID)
	if err != nil {
		return domain.Task{}, err
	}
	var currentGraph domain.GraphTask
	found := false
	for _, graphTask := range tasks {
		if graphTask.ID == task.ID {
			currentGraph, found = graphTask, true
			break
		}
	}
	if !found {
		return domain.Task{}, domain.ErrForbidden
	}
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, task.ID, task.UserID).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrForbidden
	} else if err != nil {
		return domain.Task{}, fmt.Errorf("lock task patch: %w", err)
	}
	if version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	genreChanged := currentGraph.GenreID != task.GenreID
	if genreChanged {
		if update.ApprovedGenreMove == nil {
			return domain.Task{}, domain.ErrImpactConfirmationRequired
		}
		var targetExists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM genres WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, task.GenreID, task.UserID).Scan(&targetExists); err != nil {
			return domain.Task{}, fmt.Errorf("validate target genre: %w", err)
		}
		if !targetExists {
			return domain.Task{}, domain.ErrForbidden
		}
		if task.TaskTypeID != nil {
			var attached bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM genre_task_types
          WHERE genre_id=$1 AND task_type_id=$2 AND user_id=$3)`, task.GenreID, *task.TaskTypeID, task.UserID).Scan(&attached); err != nil {
				return domain.Task{}, fmt.Errorf("validate moved task type: %w", err)
			}
			if !attached {
				task.TaskTypeID = nil
			}
		}
		plan, planErr := domain.PlanGenreMove(tasks, dependencies, task.ID, task.GenreID)
		if planErr != nil {
			return domain.Task{}, planErr
		}
		if !sameGenreMovePlan(plan, *update.ApprovedGenreMove) {
			return domain.Task{}, domain.ErrImpactPlanChanged
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_dependencies
      WHERE user_id=$1 AND ((task_id=ANY($2::uuid[]) AND NOT depends_on_task_id=ANY($2::uuid[]))
	        OR (depends_on_task_id=ANY($2::uuid[]) AND NOT task_id=ANY($2::uuid[])))`, task.UserID, plan.MovedTaskIDs); err != nil {
			return domain.Task{}, fmt.Errorf("remove genre boundary dependencies: %w", err)
		}
		// Update the complete subtree in one statement so the composite hierarchy
		// foreign key never observes a mixed-Genre tree. Incompatible TaskTypes are
		// cleared because a TaskType may only be used by an attached Genre.
		if _, err := tx.ExecContext(ctx, `UPDATE tasks moved SET
        genre_id=$1,
        parent_task_id=CASE WHEN moved.id=$2 THEN NULL ELSE moved.parent_task_id END,
        task_type_id=CASE WHEN moved.task_type_id IS NULL OR EXISTS(
          SELECT 1 FROM genre_task_types link WHERE link.genre_id=$1
            AND link.task_type_id=moved.task_type_id AND link.user_id=$3
        ) THEN moved.task_type_id ELSE NULL END,
        version=CASE WHEN moved.id=$2 THEN moved.version ELSE moved.version+1 END
	      WHERE moved.user_id=$3 AND moved.id=ANY($4::uuid[]) AND moved.deleted_at IS NULL`, task.GenreID, task.ID, task.UserID, plan.MovedTaskIDs); err != nil {
			return domain.Task{}, fmt.Errorf("move task subtree genre: %w", err)
		}
		task.ParentTaskID = nil
	} else if currentGraph.ParentID != nullableString(task.ParentTaskID) {
		if err := domain.ValidateHierarchyAssignment(tasks, task.ID, nullableString(task.ParentTaskID)); err != nil {
			return domain.Task{}, err
		}
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
		if !isGoogleInbound(ctx) {
			if err := p.enqueueGoogleTaskTx(ctx, tx, task.UserID, change.TaskID, domain.GoogleSyncTask); err != nil {
				return domain.Task{}, err
			}
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
	if !isGoogleInbound(ctx) {
		if err := p.enqueueGoogleTaskTx(ctx, tx, task.UserID, task.ID, domain.GoogleSyncTask); err != nil {
			return domain.Task{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit task patch: %w", err)
	}
	return task.ForPersistence(), nil
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sameGenreMovePlan(left, right domain.GenreMovePlan) bool {
	if left.TaskID != right.TaskID || left.NewGenreID != right.NewGenreID || left.DetachedParentID != right.DetachedParentID ||
		len(left.MovedTaskIDs) != len(right.MovedTaskIDs) || len(left.RemovedDependencies) != len(right.RemovedDependencies) {
		return false
	}
	for index := range left.MovedTaskIDs {
		if left.MovedTaskIDs[index] != right.MovedTaskIDs[index] {
			return false
		}
	}
	for index := range left.RemovedDependencies {
		if left.RemovedDependencies[index] != right.RemovedDependencies[index] {
			return false
		}
	}
	return true
}
