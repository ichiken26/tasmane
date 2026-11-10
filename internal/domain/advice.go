package domain

import "time"

// 日次アドバイス
type Advice struct {
	ID, UserID, Content, SourceHash string
	GenreID                         *string
	GeneratedAt, ExpiresAt          time.Time
}

// アドバイス生成対象ユーザー
type AdviceUser struct {
	UserID, Timezone string
	LastGeneratedAt  *time.Time
}

// ユーザー現地4時以降の1日1回生成判定
func ShouldGenerateAdvice(now time.Time, user AdviceUser) bool {
	// タイムゾーン読込と現地日付の算出
	location, err := time.LoadLocation(user.Timezone)
	if err != nil {
		return false
	}
	local := now.In(location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 4, 0, 0, 0, location)
	if local.Before(today) {
		return false
	}
	return user.LastGeneratedAt == nil || user.LastGeneratedAt.In(location).Before(today)
}

// 次の現地4時実行時刻の算出
func NextDailyRun(now time.Time, timezone string) (time.Time, error) {
	// タイムゾーン読込と当日4時の算出
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day(), 4, 0, 0, 0, location)
	if !local.Before(next) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}
