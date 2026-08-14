package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
)

func (p *Postgres) GetUserSettings(ctx context.Context, userID string) (domain.UserSettings, error) {
	if p == nil || p.database == nil || userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	var settings domain.UserSettings
	err := p.database.QueryRowContext(ctx, `
SELECT users.timezone, settings.google_sync_interval_minutes
FROM users
JOIN user_server_settings settings ON settings.user_id = users.id
WHERE users.id = $1`, userID).Scan(&settings.Timezone, &settings.GoogleSyncIntervalMinutes)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("get user settings: %w", err)
	}
	return settings, nil
}

func (p *Postgres) UpdateUserSettings(ctx context.Context, userID string, next domain.UserSettings) (domain.UserSettings, error) {
	if p == nil || p.database == nil || userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("begin settings update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE users SET timezone=$2 WHERE id=$1`, userID, next.Timezone)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("update timezone: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	result, err = tx.ExecContext(ctx, `
UPDATE user_server_settings SET google_sync_interval_minutes=$2
WHERE user_id=$1`, userID, next.GoogleSyncIntervalMinutes)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("update sync interval: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	if err := tx.Commit(); err != nil {
		return domain.UserSettings{}, fmt.Errorf("commit settings update: %w", err)
	}
	return next, nil
}
