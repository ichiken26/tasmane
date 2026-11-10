package repository

import (
	"context"
	"fmt"
	"tasmane/internal/domain"
)

// Google起点の完了取消
func (p *Postgres) GoogleUncomplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	// トランザクション開始
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, err
	}
	defer tx.Rollback()
	// ユーザーグラフのロック取得
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return domain.Task{}, err
	}
	// 祖先行の取得
	rows, err := tx.QueryContext(ctx, `WITH RECURSIVE ancestors AS (
    SELECT id,parent_task_id,status,version FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
    UNION ALL SELECT parent.id,parent.parent_task_id,parent.status,parent.version FROM tasks parent
    JOIN ancestors child ON child.parent_task_id=parent.id WHERE parent.user_id=$2 AND parent.deleted_at IS NULL
  ) SELECT id,status,version FROM ancestors`, taskID, userID)
	if err != nil {
		return domain.Task{}, err
	}
	type item struct {
		id      string
		status  domain.Status
		version int64
	}
	items := []item{}
	for rows.Next() {
		var v item
		// 祖先行のスキャン
		if err := rows.Scan(&v.id, &v.status, &v.version); err != nil {
			rows.Close()
			return domain.Task{}, err
		}
		items = append(items, v)
	}
	rows.Close()
	if len(items) == 0 {
		return domain.Task{}, domain.ErrForbidden
	}
	if items[0].version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	if items[0].status != domain.Completed {
		return domain.Task{}, domain.ErrInvalidTransition
	}
	for _, v := range items {
		if v.status != domain.Completed {
			continue
		}
		// 完了取消の実行
		result, err := tx.ExecContext(ctx, `UPDATE tasks SET status='IN_PROGRESS',leaf_progress=NULL,version=version+1 WHERE id=$1 AND user_id=$2 AND version=$3`, v.id, userID, v.version)
		if err != nil {
			return domain.Task{}, err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return domain.Task{}, domain.ErrVersionConflict
		}
		historyID, err := p.ids.New()
		if err != nil {
			return domain.Task{}, err
		}
		// 状態履歴の挿入
		if _, err = tx.ExecContext(ctx, `INSERT INTO task_status_histories(id,user_id,task_id,from_status,to_status,changed_by_user_id,changed_via) VALUES($1,$2,$3,'COMPLETED','IN_PROGRESS',$2,'GOOGLE_TASKS')`, historyID, userID, v.id); err != nil {
			return domain.Task{}, err
		}
	}
	// 取消の確定
	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit google uncomplete: %w", err)
	}
	return p.Get(ctx, userID, taskID)
}
