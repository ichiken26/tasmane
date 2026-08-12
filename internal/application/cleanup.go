package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const hardDeleteSoftDeletedTasksSQL = `DELETE FROM tasks WHERE deleted_at IS NOT NULL AND deleted_at < $1`

type Cleanup struct {
	DB        *sql.DB
	Retention time.Duration
}

func (c Cleanup) HardDeleteExpiredTasks(ctx context.Context, now time.Time) (int64, error) {
	if c.DB == nil {
		return 0, fmt.Errorf("database is required")
	}
	retention := c.Retention
	if retention == 0 {
		retention = 30 * 24 * time.Hour
	}
	if retention < 0 {
		return 0, fmt.Errorf("retention must not be negative")
	}
	result, err := c.DB.ExecContext(ctx, hardDeleteSoftDeletedTasksSQL, now.Add(-retention))
	if err != nil {
		return 0, fmt.Errorf("hard delete expired tasks: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count hard deleted tasks: %w", err)
	}
	return count, nil
}
