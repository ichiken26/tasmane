package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
)

// タスク削除計画の適用
func (p *Postgres) DeleteTask(ctx context.Context, userID, taskID string, expected int64, confirmed bool) (domain.TaskDeletionPlan, error) {
	return p.deleteTask(ctx, userID, taskID, expected, confirmed, nil)
}

// 削除計画の算出と（承認集合一致時の）適用
func (p *Postgres) deleteTask(ctx context.Context, userID, taskID string, expected int64, confirmed bool, approved []string) (domain.TaskDeletionPlan, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" || expected < 1 {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	}
	// トランザクション開始
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("begin task delete: %w", err)
	}
	defer tx.Rollback()
	// ユーザーグラフのロック取得
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return domain.TaskDeletionPlan{}, err
	}
	var version int64
	// 削除対象行のロック取得
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
	// 承認時に提示した削除集合からの変化検出
	if approved != nil && !sameIDSet(approved, plan.TaskIDs) {
		return plan, domain.ErrImpactPlanChanged
	}
	// ソフト削除の実行
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
	// 削除で子を失った親の葉進捗正規化
	deleted := map[string]bool{}
	for _, id := range plan.TaskIDs {
		deleted[id] = true
	}
	parents := []string{}
	for _, task := range tasks {
		if deleted[task.ID] && task.ParentID != "" && !deleted[task.ParentID] {
			parents = append(parents, task.ParentID)
		}
	}
	if err := normalizeChildlessParentsTx(ctx, tx, userID, parents); err != nil {
		return domain.TaskDeletionPlan{}, err
	}
	for _, deletedTaskID := range plan.TaskIDs {
		if err := p.enqueueGoogleTaskTx(ctx, tx, userID, deletedTaskID, domain.GoogleDeleteTask); err != nil {
			return domain.TaskDeletionPlan{}, err
		}
	}
	// 削除の確定
	if err := tx.Commit(); err != nil {
		return domain.TaskDeletionPlan{}, fmt.Errorf("commit task delete: %w", err)
	}
	return plan, nil
}

// ID集合の一致判定
func sameIDSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[string]bool, len(left))
	for _, id := range left {
		set[id] = true
	}
	for _, id := range right {
		if !set[id] {
			return false
		}
	}
	return true
}
