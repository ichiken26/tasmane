package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"time"
)

var ErrInvalidSettings = errors.New("invalid settings")

// ユーザー設定の永続化境界
type SettingsRepository interface {
	GetUserSettings(context.Context, string) (domain.UserSettings, error)
	UpdateUserSettings(context.Context, string, domain.UserSettings) (domain.UserSettings, error)
}

// ユーザー設定の取得・更新ユースケース
type Settings struct{ Repo SettingsRepository }

// 認証済みユーザーの設定取得
func (u Settings) Get(ctx context.Context, userID string) (domain.UserSettings, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	// リポジトリからの設定読み出し
	return u.Repo.GetUserSettings(ctx, userID)
}

// タイムゾーンと同期間隔の妥当性検証付き設定更新
func (u Settings) Update(ctx context.Context, userID string, next domain.UserSettings) (domain.UserSettings, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	// 空タイムゾーンの拒否
	if next.Timezone == "" {
		return domain.UserSettings{}, ErrInvalidSettings
	}
	// IANAタイムゾーンの実在確認
	if _, err := time.LoadLocation(next.Timezone); err != nil {
		return domain.UserSettings{}, ErrInvalidSettings
	}
	// Google同期間隔の1〜1440分制約
	if next.GoogleSyncIntervalMinutes < 1 || next.GoogleSyncIntervalMinutes > 1440 {
		return domain.UserSettings{}, ErrInvalidSettings
	}
	// 検証済み設定の永続化
	return u.Repo.UpdateUserSettings(ctx, userID, next)
}
