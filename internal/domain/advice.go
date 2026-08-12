package domain

import "time"

type Advice struct {
	ID, UserID, Content, SourceHash string
	GenreID                         *string
	GeneratedAt, ExpiresAt          time.Time
}
type AdviceUser struct {
	UserID, Timezone string
	LastGeneratedAt  *time.Time
}

func ShouldGenerateAdvice(now time.Time, user AdviceUser) bool {
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
func NextDailyRun(now time.Time, timezone string) (time.Time, error) {
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
