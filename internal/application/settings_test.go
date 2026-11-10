package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"testing"
)

// ユーザー設定リポジトリのテスト用スタブ
type settingsRepoStub struct {
	value domain.UserSettings
}

// スタブ設定の返却
func (s *settingsRepoStub) GetUserSettings(context.Context, string) (domain.UserSettings, error) {
	return s.value, nil
}

// スタブ設定の上書き保存
func (s *settingsRepoStub) UpdateUserSettings(_ context.Context, _ string, next domain.UserSettings) (domain.UserSettings, error) {
	s.value = next
	return next, nil
}

// 不正タイムゾーンと間隔の拒否および正当値の保存確認
func TestSettingsRejectInvalidTimezoneAndInterval(t *testing.T) {
	u := Settings{Repo: &settingsRepoStub{value: domain.UserSettings{Timezone: "Asia/Tokyo", GoogleSyncIntervalMinutes: 10}}}
	// 不正タイムゾーンの拒否確認
	if _, err := u.Update(context.Background(), "u", domain.UserSettings{Timezone: "Not/AZone", GoogleSyncIntervalMinutes: 10}); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("timezone error = %v", err)
	}
	// 下限未満同期間隔の拒否確認
	if _, err := u.Update(context.Background(), "u", domain.UserSettings{Timezone: "Asia/Tokyo", GoogleSyncIntervalMinutes: 0}); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("interval error = %v", err)
	}
	// 正当値の保存確認
	saved, err := u.Update(context.Background(), "u", domain.UserSettings{Timezone: "UTC", GoogleSyncIntervalMinutes: 15})
	if err != nil || saved.Timezone != "UTC" || saved.GoogleSyncIntervalMinutes != 15 {
		t.Fatalf("saved=%v err=%v", saved, err)
	}
}
