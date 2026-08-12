package application

import (
	"context"
	"tasmane/internal/domain"
	"testing"
	"time"
)

type providerStub struct{ calls int }

func (p *providerStub) Generate(context.Context, string) (string, error) {
	p.calls++
	return "advice", nil
}

type adviceRepoStub struct{ saved *domain.Advice }

func (r *adviceRepoStub) FindAdvice(_ context.Context, _ string, _ *string, hash string) (domain.Advice, error) {
	if r.saved != nil && r.saved.SourceHash == hash {
		return *r.saved, nil
	}
	return domain.Advice{}, domain.ErrForbidden
}
func (r *adviceRepoStub) SaveAdvice(_ context.Context, a domain.Advice) (domain.Advice, error) {
	r.saved = &a
	return a, nil
}
func (r *adviceRepoStub) DeleteExpiredAdvice(context.Context, time.Time) (int64, error) {
	return 1, nil
}
func TestAdviceSourceHashDeduplicatesAndExpiresInThirtyDays(t *testing.T) {
	now := time.Date(2026, 8, 13, 4, 0, 0, 0, time.UTC)
	provider := &providerStub{}
	repo := &adviceRepoStub{}
	service := Advices{Provider: provider, Repo: repo, Now: func() time.Time { return now }}
	first, err := service.Generate(context.Background(), "u", nil, map[string]any{"tasks": []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Generate(context.Background(), "u", nil, map[string]any{"tasks": []string{"a"}})
	if err != nil || provider.calls != 1 || first.SourceHash != second.SourceHash || !first.ExpiresAt.Equal(now.AddDate(0, 0, 30)) {
		t.Fatal(first, second, provider.calls, err)
	}
}
