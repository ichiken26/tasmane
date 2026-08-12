package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"tasmane/internal/domain"
	"time"
)

type AdviceProvider interface {
	Generate(context.Context, string) (string, error)
}
type AdviceRepository interface {
	FindAdvice(context.Context, string, *string, string) (domain.Advice, error)
	SaveAdvice(context.Context, domain.Advice) (domain.Advice, error)
	DeleteExpiredAdvice(context.Context, time.Time) (int64, error)
}
type Advices struct {
	Provider AdviceProvider
	Repo     AdviceRepository
	Now      func() time.Time
}

func (a Advices) Generate(ctx context.Context, userID string, genreID *string, source any) (domain.Advice, error) {
	if userID == "" {
		return domain.Advice{}, domain.ErrForbidden
	}
	canonical, err := json.Marshal(source)
	if err != nil {
		return domain.Advice{}, err
	}
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	if existing, err := a.Repo.FindAdvice(ctx, userID, genreID, hash); err == nil {
		return existing, nil
	}
	text, err := a.Provider.Generate(ctx, "タスク状況から、簡潔で実行可能な日本語の助言を作成してください。\n"+string(canonical))
	if err != nil {
		return domain.Advice{}, err
	}
	now := time.Now().UTC()
	if a.Now != nil {
		now = a.Now()
	}
	return a.Repo.SaveAdvice(ctx, domain.Advice{UserID: userID, GenreID: genreID, Content: text, SourceHash: hash, GeneratedAt: now, ExpiresAt: now.AddDate(0, 0, 30)})
}
func (a Advices) Cleanup(ctx context.Context, now time.Time) (int64, error) {
	return a.Repo.DeleteExpiredAdvice(ctx, now)
}

var _ = errors.Is
