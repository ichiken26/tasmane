package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"testing"
)

type settingsRepoStub struct {
	value domain.UserSettings
}

func (s *settingsRepoStub) GetUserSettings(context.Context, string) (domain.UserSettings, error) {
	return s.value, nil
}
func (s *settingsRepoStub) UpdateUserSettings(_ context.Context, _ string, next domain.UserSettings) (domain.UserSettings, error) {
	s.value = next
	return next, nil
}

func TestSettingsRejectInvalidTimezoneAndInterval(t *testing.T) {
	u := Settings{Repo: &settingsRepoStub{value: domain.UserSettings{Timezone: "Asia/Tokyo", GoogleSyncIntervalMinutes: 10}}}
	if _, err := u.Update(context.Background(), "u", domain.UserSettings{Timezone: "Not/AZone", GoogleSyncIntervalMinutes: 10}); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("timezone error = %v", err)
	}
	if _, err := u.Update(context.Background(), "u", domain.UserSettings{Timezone: "Asia/Tokyo", GoogleSyncIntervalMinutes: 0}); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("interval error = %v", err)
	}
	saved, err := u.Update(context.Background(), "u", domain.UserSettings{Timezone: "UTC", GoogleSyncIntervalMinutes: 15})
	if err != nil || saved.Timezone != "UTC" || saved.GoogleSyncIntervalMinutes != 15 {
		t.Fatalf("saved=%v err=%v", saved, err)
	}
}
