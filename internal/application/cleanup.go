package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const hardDeleteSoftDeletedTasksSQL = `DELETE FROM tasks WHERE deleted_at IS NOT NULL AND deleted_at < $1`

// 論理削除タスクの物理削除ジョブ
type Cleanup struct {
	DB        *sql.DB
	Retention time.Duration
}

// 保持期間超過タスクの物理削除
func (c Cleanup) HardDeleteExpiredTasks(ctx context.Context, now time.Time) (int64, error) {
	// DB必須チェック
	if c.DB == nil {
		return 0, fmt.Errorf("database is required")
	}
	// 保持期間の既定値適用
	retention := c.Retention
	if retention == 0 {
		retention = 30 * 24 * time.Hour
	}
	// 負の保持期間の拒否
	if retention < 0 {
		return 0, fmt.Errorf("retention must not be negative")
	}
	// 期限切れ論理削除タスクの物理削除
	result, err := c.DB.ExecContext(ctx, hardDeleteSoftDeletedTasksSQL, now.Add(-retention))
	if err != nil {
		return 0, fmt.Errorf("hard delete expired tasks: %w", err)
	}
	// 削除件数の取得
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count hard deleted tasks: %w", err)
	}
	return count, nil
}
