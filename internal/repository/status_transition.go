package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"tasmane/internal/domain"
)

const statusTransitionSnapshotSQL = `WITH RECURSIVE
ancestors AS (
  SELECT parent.id,parent.parent_task_id,parent.status,parent.leaf_progress,parent.version,
         EXISTS (SELECT 1 FROM tasks c WHERE c.parent_task_id=parent.id AND c.user_id=parent.user_id AND c.deleted_at IS NULL) has_children, 1 depth
  FROM tasks target JOIN tasks parent ON parent.id=target.parent_task_id
  WHERE target.id=$1 AND target.user_id=$2 AND target.deleted_at IS NULL AND parent.user_id=$2 AND parent.deleted_at IS NULL
  UNION ALL
  SELECT parent.id,parent.parent_task_id,parent.status,parent.leaf_progress,parent.version,
         true, child.depth+1
  FROM ancestors child JOIN tasks parent ON parent.id=child.parent_task_id
  WHERE parent.user_id=$2 AND parent.deleted_at IS NULL
),
descendants AS (
  SELECT child.id,child.parent_task_id,child.status,child.leaf_progress,child.version,
         EXISTS (SELECT 1 FROM tasks c WHERE c.parent_task_id=child.id AND c.user_id=child.user_id AND c.deleted_at IS NULL) has_children, 1 depth
  FROM tasks child WHERE child.parent_task_id=$1 AND child.user_id=$2 AND child.deleted_at IS NULL
  UNION ALL
  SELECT child.id,child.parent_task_id,child.status,child.leaf_progress,child.version,
         EXISTS (SELECT 1 FROM tasks c WHERE c.parent_task_id=child.id AND c.user_id=child.user_id AND c.deleted_at IS NULL), parent.depth+1
  FROM descendants parent JOIN tasks child ON child.parent_task_id=parent.id
  WHERE child.user_id=$2 AND child.deleted_at IS NULL
)
SELECT 'target',t.id,t.parent_task_id,t.status,t.leaf_progress,t.version,
       EXISTS (SELECT 1 FROM tasks c WHERE c.parent_task_id=t.id AND c.user_id=t.user_id AND c.deleted_at IS NULL),0
FROM tasks t WHERE t.id=$1 AND t.user_id=$2 AND t.deleted_at IS NULL
UNION ALL SELECT 'ancestor',id,parent_task_id,status,leaf_progress,version,has_children,depth FROM ancestors
UNION ALL SELECT 'descendant',id,parent_task_id,status,leaf_progress,version,has_children,depth FROM descendants
ORDER BY 1 DESC,8,2`

// 状態遷移計画のプレビュー
func (p *Postgres) PreviewStatusTransition(ctx context.Context, userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" || expected < 1 {
		return domain.StatusTransitionPlan{}, domain.ErrForbidden
	}
	return readStatusTransitionPlan(ctx, p.database, userID, taskID, expected, to)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// 状態遷移計画の読取
func readStatusTransitionPlan(ctx context.Context, query queryer, userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	// スナップショットSQLの実行
	rows, err := query.QueryContext(ctx, statusTransitionSnapshotSQL, taskID, userID)
	if err != nil {
		return domain.StatusTransitionPlan{}, fmt.Errorf("read status impact: %w", err)
	}
	defer rows.Close()
	var target domain.Task
	ancestors, descendants := []domain.Task{}, []domain.Task{}
	for rows.Next() {
		var kind, id string
		var parent sql.NullString
		var status domain.Status
		var progress sql.NullInt64
		var version int64
		var hasChildren bool
		var depth int
		// 影響行のスキャン
		if err := rows.Scan(&kind, &id, &parent, &status, &progress, &version, &hasChildren, &depth); err != nil {
			return domain.StatusTransitionPlan{}, err
		}
		task := domain.Task{ID: id, UserID: userID, Status: status, Version: version, HasChildren: hasChildren}
		if parent.Valid {
			task.ParentTaskID = &parent.String
		}
		if progress.Valid {
			value := int(progress.Int64)
			task.Progress = &value
		}
		switch kind {
		case "target":
			target = task
		case "ancestor":
			ancestors = append(ancestors, task)
		case "descendant":
			descendants = append(descendants, task)
		}
	}
	if err := rows.Err(); err != nil {
		return domain.StatusTransitionPlan{}, err
	}
	if target.ID == "" {
		return domain.StatusTransitionPlan{}, domain.ErrForbidden
	}
	if target.Version != expected {
		return domain.StatusTransitionPlan{}, domain.ErrVersionConflict
	}
	return domain.PlanStatusTransition(target, ancestors, descendants, to)
}

// 状態遷移計画の適用
func (p *Postgres) ApplyStatusTransition(ctx context.Context, userID, taskID string, expected int64, to domain.Status, via string, approved domain.StatusTransitionPlan) (domain.Task, error) {
	if p == nil || p.database == nil || p.ids == nil || userID == "" || taskID == "" || expected < 1 || !validChangeVia(via) {
		return domain.Task{}, domain.ErrForbidden
	}
	// トランザクション開始
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin status transition: %w", err)
	}
	defer tx.Rollback()
	// ユーザーグラフのロック取得
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return domain.Task{}, err
	}
	current, err := readStatusTransitionPlan(ctx, tx, userID, taskID, expected, to)
	if err != nil {
		return domain.Task{}, err
	}
	if !equalStatusTransitionPlan(current, approved) {
		return domain.Task{}, domain.ErrStatusImpactPlanChanged
	}

	// 行ロック順序固定とスナップショット保護
	changes := append([]domain.StatusTransitionChange(nil), current.Changes...)
	sort.Slice(changes, func(i, j int) bool { return changes[i].TaskID < changes[j].TaskID })
	for _, change := range changes {
		var progress any
		if change.ToProgress != nil {
			progress = *change.ToProgress
		}
		// 状態更新の実行
		result, updateErr := tx.ExecContext(ctx, `UPDATE tasks SET status=$1,leaf_progress=$2,version=version+1,updated_at=now() WHERE id=$3 AND user_id=$4 AND version=$5 AND status=$6 AND deleted_at IS NULL`, change.ToStatus, progress, change.TaskID, userID, change.ExpectedVersion, change.FromStatus)
		if updateErr != nil {
			return domain.Task{}, fmt.Errorf("apply status impact: %w", updateErr)
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return domain.Task{}, domain.ErrVersionConflict
		}
		historyTask := domain.Task{ID: change.TaskID, UserID: userID, Status: change.ToStatus, Progress: change.ToProgress}
		if err := p.insertStatusHistory(ctx, tx, historyTask, change.FromStatus, via); err != nil {
			return domain.Task{}, err
		}
		if !sameIntPointers(change.FromProgress, change.ToProgress) {
			from := sql.NullInt64{}
			if change.FromProgress != nil {
				from.Valid = true
				from.Int64 = int64(*change.FromProgress)
			}
			if err := p.insertProgressHistory(ctx, tx, historyTask, from, via); err != nil {
				return domain.Task{}, err
			}
		}
		if via != "GOOGLE_TASKS" {
			if err := p.enqueueGoogleTaskTx(ctx, tx, userID, change.TaskID, domain.GoogleSyncTask); err != nil {
				return domain.Task{}, err
			}
		}
	}
	// 遷移の確定
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit status transition: %w", err)
	}
	return p.Get(ctx, userID, taskID)
}

// 状態遷移計画の一致判定
func equalStatusTransitionPlan(a, b domain.StatusTransitionPlan) bool {
	if a.TaskID != b.TaskID || a.ToStatus != b.ToStatus || a.ExpectedVersion != b.ExpectedVersion || a.RequiresConfirmation != b.RequiresConfirmation || len(a.Changes) != len(b.Changes) {
		return false
	}
	for i := range a.Changes {
		x, y := a.Changes[i], b.Changes[i]
		if x.TaskID != y.TaskID || x.FromStatus != y.FromStatus || x.ToStatus != y.ToStatus || x.ExpectedVersion != y.ExpectedVersion || !sameIntPointers(x.FromProgress, y.FromProgress) || !sameIntPointers(x.ToProgress, y.ToProgress) {
			return false
		}
	}
	return true
}

// 整数ポインタの一致判定
func sameIntPointers(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
