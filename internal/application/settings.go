package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"time"
)

var ErrInvalidSettings = errors.New("invalid settings")

type SettingsRepository interface {
	GetUserSettings(context.Context, string) (domain.UserSettings, error)
	UpdateUserSettings(context.Context, string, domain.UserSettings) (domain.UserSettings, error)
}

type Settings struct{ Repo SettingsRepository }

func (u Settings) Get(ctx context.Context, userID string) (domain.UserSettings, error) {
	if userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	return u.Repo.GetUserSettings(ctx, userID)
}

func (u Settings) Update(ctx context.Context, userID string, next domain.UserSettings) (domain.UserSettings, error) {
	if userID == "" {
		return domain.UserSettings{}, domain.ErrForbidden
	}
	if next.Timezone == "" {
		return domain.UserSettings{}, ErrInvalidSettings
	}
	if _, err := time.LoadLocation(next.Timezone); err != nil {
		return domain.UserSettings{}, ErrInvalidSettings
	}
	if next.GoogleSyncIntervalMinutes < 1 || next.GoogleSyncIntervalMinutes > 1440 {
		return domain.UserSettings{}, ErrInvalidSettings
	}
	return u.Repo.UpdateUserSettings(ctx, userID, next)
}
