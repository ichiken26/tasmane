package domain

import (
	"testing"
	"time"
)

func TestNextDailyRunUsesUserTimezoneAtFour(t *testing.T) {
	now := time.Date(2026, 8, 12, 20, 0, 0, 0, time.UTC)
	got, err := NextDailyRun(now, "Asia/Tokyo")
	if err != nil || got.Hour() != 4 || got.Day() != 14 {
		t.Fatal(got, err)
	}
}
func TestShouldGenerateAdviceOnceAfterLocalFour(t *testing.T) {
	now := time.Date(2026, 8, 12, 18, 30, 0, 0, time.UTC)
	user := AdviceUser{Timezone: "Asia/Tokyo"}
	if ShouldGenerateAdvice(now, user) {
		t.Fatal("03:30 must not run")
	}
	now = now.Add(time.Hour)
	if !ShouldGenerateAdvice(now, user) {
		t.Fatal("04:30 must run")
	}
	last := now
	user.LastGeneratedAt = &last
	if ShouldGenerateAdvice(now.Add(time.Hour), user) {
		t.Fatal("must run once")
	}
}
