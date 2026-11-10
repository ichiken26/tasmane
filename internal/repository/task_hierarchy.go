package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"tasmane/internal/domain"
)

// 承認済み日付伝播の適用（変更前値が一致する行のみ更新）
func (p *Postgres) applyDateChangesTx(ctx context.Context, tx *sql.Tx, userID, skipTaskID string, changes []domain.DateChange) error {
	for _, change := range changes {
		if change.TaskID == skipTaskID {
			continue
		}
		result, err := tx.ExecContext(ctx, `UPDATE tasks SET start_at=$1,end_at=$2,version=version+1
      WHERE id=$3 AND user_id=$4 AND deleted_at IS NULL
        AND start_at IS NOT DISTINCT FROM $5 AND end_at IS NOT DISTINCT FROM $6`,
			change.After.StartAt, change.After.EndAt, change.TaskID, userID, change.Before.StartAt, change.Before.EndAt)
		if err != nil {
			return fmt.Errorf("propagate task dates: %w", err)
		}
		if count, _ := result.RowsAffected(); count != 1 {
			return domain.ErrVersionConflict
		}
		if !isGoogleInbound(ctx) {
			if err := p.enqueueGoogleTaskTx(ctx, tx, userID, change.TaskID, domain.GoogleSyncTask); err != nil {
				return err
			}
		}
	}
	return nil
}

// 子を失って葉へ戻ったタスクの進捗正規化（NOT_STARTED→NULL, IN_PROGRESS→0, COMPLETED→100）
func normalizeChildlessParentsTx(ctx context.Context, tx *sql.Tx, userID string, candidateIDs []string) error {
	if len(candidateIDs) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE tasks task SET
      leaf_progress=CASE task.status WHEN 'IN_PROGRESS' THEN 0 WHEN 'COMPLETED' THEN 100 ELSE NULL END,
      version=task.version+1
    WHERE task.user_id=$1 AND task.id=ANY($2::uuid[]) AND task.deleted_at IS NULL AND task.leaf_progress IS NULL
      AND task.status <> 'NOT_STARTED'
      AND NOT EXISTS (SELECT 1 FROM tasks child WHERE child.parent_task_id=task.id AND child.user_id=task.user_id AND child.deleted_at IS NULL)`,
		userID, candidateIDs)
	if err != nil {
		return fmt.Errorf("normalize childless parents: %w", err)
	}
	return nil
}

// 付け替え先親の検証と葉→親変換（完了済み親は禁止、葉は承認必須）
func assignParentTx(ctx context.Context, tx *sql.Tx, userID, genreID, parentID string, approvedVersion *int64) error {
	var status, parentGenre string
	var version int64
	var hasChildren bool
	err := tx.QueryRowContext(ctx, `SELECT parent.status, parent.genre_id, parent.version,
      EXISTS (SELECT 1 FROM tasks child WHERE child.parent_task_id=parent.id AND child.user_id=parent.user_id AND child.deleted_at IS NULL)
      FROM tasks parent WHERE parent.id=$1 AND parent.user_id=$2 AND parent.deleted_at IS NULL FOR UPDATE OF parent`,
		parentID, userID).Scan(&status, &parentGenre, &version, &hasChildren)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("lock new parent: %w", err)
	}
	if parentGenre != genreID {
		return domain.ErrGraphGenre
	}
	if domain.Status(status) == domain.Completed {
		return domain.ErrCompletedParent
	}
	if hasChildren {
		return nil
	}
	if approvedVersion == nil {
		return domain.ErrImpactConfirmationRequired
	}
	if *approvedVersion != version {
		return domain.ErrImpactPlanChanged
	}
	// 葉の親化（leaf_progressは権威値でなくなるためNULL）
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET leaf_progress=NULL, version=version+1
    WHERE id=$1 AND user_id=$2 AND version=$3 AND deleted_at IS NULL`, parentID, userID, version); err != nil {
		return fmt.Errorf("convert leaf parent: %w", err)
	}
	return nil
}

// 葉タスク進捗の更新（履歴付き・Google非同期対象外）
func (p *Postgres) UpdateLeafProgress(ctx context.Context, userID, taskID string, expected int64, progress int, via string) (domain.Task, error) {
	if p == nil || p.database == nil || p.ids == nil || userID == "" || taskID == "" || expected < 1 || !validChangeVia(via) {
		return domain.Task{}, domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin progress update: %w", err)
	}
	defer tx.Rollback()
	var status string
	var current sql.NullInt64
	var version int64
	var hasChildren bool
	// 対象行のロック取得
	err = tx.QueryRowContext(ctx, taskForUpdateSQL, taskID, userID).Scan(&status, &current, &version, &hasChildren)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("lock task progress: %w", err)
	}
	if version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	if err := domain.ValidateLeafProgress(domain.Task{ID: taskID, Status: domain.Status(status), HasChildren: hasChildren}, progress); err != nil {
		return domain.Task{}, err
	}
	task := domain.Task{ID: taskID, UserID: userID, Status: domain.Status(status), Progress: &progress}
	if err := tx.QueryRowContext(ctx, `UPDATE tasks SET leaf_progress=$1, version=version+1
    WHERE id=$2 AND user_id=$3 AND version=$4 AND deleted_at IS NULL RETURNING version`, progress, taskID, userID, expected).Scan(&task.Version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrVersionConflict
		}
		return domain.Task{}, fmt.Errorf("update leaf progress: %w", err)
	}
	if !sameNullableProgress(current, task.Progress) {
		if err := p.insertProgressHistory(ctx, tx, task, current, via); err != nil {
			return domain.Task{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit progress update: %w", err)
	}
	return p.Get(ctx, userID, taskID)
}

// 承認済み削除計画の適用（計画が変化していれば拒否）
func (p *Postgres) DeleteTaskApproved(ctx context.Context, userID, taskID string, expected int64, approved []string) (domain.TaskDeletionPlan, error) {
	return p.deleteTask(ctx, userID, taskID, expected, true, approved)
}

// 復元可能（30日以内）な削除済みタスク一覧
func (p *Postgres) ListDeletedTasks(ctx context.Context, userID string) ([]domain.DeletedTask, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, domain.ErrForbidden
	}
	rows, err := p.database.QueryContext(ctx, `SELECT task.id, task.genre_id, task.parent_task_id, task.title, task.status, task.end_at, task.deleted_at
    FROM tasks task WHERE task.user_id=$1 AND task.deleted_at IS NOT NULL AND task.deleted_at > now() - interval '30 days'
    ORDER BY task.deleted_at DESC, task.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list deleted tasks: %w", err)
	}
	defer rows.Close()
	result := []domain.DeletedTask{}
	for rows.Next() {
		var value domain.DeletedTask
		var parent sql.NullString
		var endAt sql.NullTime
		var status string
		if err := rows.Scan(&value.ID, &value.GenreID, &parent, &value.Title, &status, &endAt, &value.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan deleted task: %w", err)
		}
		value.Status = domain.Status(status)
		if parent.Valid {
			value.ParentTaskID = &parent.String
		}
		if endAt.Valid {
			value.EndAt = &endAt.Time
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

// 削除単位（同一削除操作で削除されたタスク群）の復元とGoogle再作成
func (p *Postgres) RestoreTask(ctx context.Context, userID, taskID string) ([]string, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin restore: %w", err)
	}
	defer tx.Rollback()
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return nil, err
	}
	var deletedAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT deleted_at FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NOT NULL
    AND deleted_at > now() - interval '30 days' FOR UPDATE`, taskID, userID).Scan(&deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrForbidden
	}
	if err != nil {
		return nil, fmt.Errorf("lock restore target: %w", err)
	}
	// 同一削除操作の集合（同一トランザクションのdeleted_atは一致）
	rows, err := tx.QueryContext(ctx, `SELECT id, parent_task_id, genre_id FROM tasks WHERE user_id=$1 AND deleted_at=$2 FOR UPDATE`, userID, deletedAt)
	if err != nil {
		return nil, fmt.Errorf("load restore batch: %w", err)
	}
	type member struct{ id, parent, genre string }
	members := []member{}
	inBatch := map[string]bool{}
	for rows.Next() {
		var value member
		var parent sql.NullString
		if err := rows.Scan(&value.id, &parent, &value.genre); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan restore batch: %w", err)
		}
		value.parent = parent.String
		members = append(members, value)
		inBatch[value.id] = true
	}
	rows.Close()
	ids := make([]string, 0, len(members))
	for _, value := range members {
		ids = append(ids, value.id)
		// ジャンル削除済みなら復元不可
		var genreAlive bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM genres WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, value.genre, userID).Scan(&genreAlive); err != nil {
			return nil, fmt.Errorf("check restore genre: %w", err)
		}
		if !genreAlive {
			return nil, domain.ErrConflict
		}
		if value.parent == "" || inBatch[value.parent] {
			continue
		}
		// 集合外の親: 削除済みなら根として復元、存在すれば親子規則を再検証
		var parentDeleted bool
		if err := tx.QueryRowContext(ctx, `SELECT deleted_at IS NOT NULL FROM tasks WHERE id=$1 AND user_id=$2`, value.parent, userID).Scan(&parentDeleted); err != nil {
			return nil, fmt.Errorf("check restore parent: %w", err)
		}
		if parentDeleted {
			if _, err := tx.ExecContext(ctx, `UPDATE tasks SET parent_task_id=NULL WHERE id=$1 AND user_id=$2`, value.id, userID); err != nil {
				return nil, fmt.Errorf("detach restore root: %w", err)
			}
			continue
		}
		var parentStatus string
		var parentLeaf bool
		if err := tx.QueryRowContext(ctx, `SELECT status, NOT EXISTS (SELECT 1 FROM tasks child WHERE child.parent_task_id=parent.id AND child.user_id=parent.user_id AND child.deleted_at IS NULL)
      FROM tasks parent WHERE id=$1 AND user_id=$2 FOR UPDATE`, value.parent, userID).Scan(&parentStatus, &parentLeaf); err != nil {
			return nil, fmt.Errorf("lock restore parent: %w", err)
		}
		if domain.Status(parentStatus) == domain.Completed {
			return nil, domain.ErrCompletedParent
		}
		if parentLeaf {
			if _, err := tx.ExecContext(ctx, `UPDATE tasks SET leaf_progress=NULL, version=version+1 WHERE id=$1 AND user_id=$2`, value.parent, userID); err != nil {
				return nil, fmt.Errorf("convert restore parent: %w", err)
			}
		}
	}
	sort.Strings(ids)
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET deleted_at=NULL, version=version+1 WHERE user_id=$1 AND id=ANY($2::uuid[])`, userID, ids); err != nil {
		return nil, fmt.Errorf("restore tasks: %w", err)
	}
	// Google側は新規作成し新IDで再紐付け
	for _, id := range ids {
		if err := p.enqueueGoogleTaskTx(ctx, tx, userID, id, domain.GoogleRecreateTask); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit restore: %w", err)
	}
	return ids, nil
}
