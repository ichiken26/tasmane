package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
)

func (p *Postgres) DeleteTask(ctx context.Context, userID, taskID string, expected int64, confirmed bool) (domain.TaskDeletionPlan, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" || expected < 1 {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("begin task delete: %w", err)
	}
	defer tx.Rollback()
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return domain.TaskDeletionPlan{}, err
	}
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, taskID, userID).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	} else if err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("lock delete target: %w", err)
	}
	if version != expected {
		return domain.TaskDeletionPlan{}, domain.ErrVersionConflict
	}
	tasks, deps, err := loadGraph(ctx, tx, userID)
	if err != nil {
		return domain.TaskDeletionPlan{}, err
	}
	plan, err := domain.PlanTaskDeletion(tasks, deps, taskID)
	if err != nil {
		return domain.TaskDeletionPlan{}, err
	}
	if !confirmed {
		return plan, domain.ErrImpactConfirmationRequired
	}
	result, err := tx.ExecContext(ctx, `UPDATE tasks SET deleted_at=now(),version=version+1 WHERE user_id=$1 AND id=ANY($2::uuid[]) AND deleted_at IS NULL`, userID, plan.TaskIDs)
	if err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("soft delete task impact: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("soft delete task result: %w", err)
	}
	if count != int64(len(plan.TaskIDs)) {
		return domain.TaskDeletionPlan{}, domain.ErrVersionConflict
	}
	if err := tx.Commit(); err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("commit task delete: %w", err)
	}
	return plan, nil
}
