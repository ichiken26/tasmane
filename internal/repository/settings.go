package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
)

// ユーザー設定の取得
func (p *Postgres) GetUserSettings(ctx context.Context, userID string) (domain.UserSettings, error) {
	if p == nil || p.database == nil || userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	var settings domain.UserSettings
	// 設定行のスキャン
	err := p.database.QueryRowContext(ctx, `
SELECT users.timezone, settings.google_sync_interval_minutes, settings.memo_integration_enabled
FROM users
JOIN user_server_settings settings ON settings.user_id = users.id
WHERE users.id = $1`, userID).Scan(&settings.Timezone, &settings.GoogleSyncIntervalMinutes, &settings.MemoIntegrationEnabled)
	// 未存在時の禁止エラー変換
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("get user settings: %w", err)
	}
	return settings, nil
}

// ユーザー設定の更新
func (p *Postgres) UpdateUserSettings(ctx context.Context, userID string, next domain.UserSettings) (domain.UserSettings, error) {
	if p == nil || p.database == nil || userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	// トランザクション開始
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("begin settings update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// タイムゾーン更新の実行
	result, err := tx.ExecContext(ctx, `UPDATE users SET timezone=$2 WHERE id=$1`, userID, next.Timezone)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("update timezone: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	// 同期間隔更新の実行
	result, err = tx.ExecContext(ctx, `
UPDATE user_server_settings SET google_sync_interval_minutes=$2, memo_integration_enabled=$3, updated_at=now()
WHERE user_id=$1`, userID, next.GoogleSyncIntervalMinutes, next.MemoIntegrationEnabled)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("update sync interval: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	// 設定更新の確定
	if err := tx.Commit(); err != nil {
		return domain.UserSettings{}, fmt.Errorf("commit settings update: %w", err)
	}
	return next, nil
}
