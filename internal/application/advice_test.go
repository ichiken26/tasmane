package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"testing"
	"time"
)

// 助言プロバイダのテスト用スタブ
type providerStub struct{ calls int }

// 呼出回数記録付き助言返却
func (p *providerStub) Generate(context.Context, string) (string, error) {
	p.calls++
	return "advice", nil
}

// 助言リポジトリのテスト用スタブ
type adviceRepoStub struct{ saved *domain.Advice }

// 同一ハッシュ助言の返却
func (r *adviceRepoStub) FindAdvice(_ context.Context, _ string, _ *string, hash string) (domain.Advice, error) {
	if r.saved != nil && r.saved.SourceHash == hash {
		return *r.saved, nil
	}
	return domain.Advice{}, domain.ErrForbidden
}

// 保存済み最新助言の返却
func (r *adviceRepoStub) LatestAdvice(_ context.Context, _ string, _ *string) (domain.Advice, error) {
	if r.saved == nil {
		return domain.Advice{}, domain.ErrForbidden
	}
	return *r.saved, nil
}

// スタブ助言の上書き保存
func (r *adviceRepoStub) SaveAdvice(_ context.Context, a domain.Advice) (domain.Advice, error) {
	r.saved = &a
	return a, nil
}

// テナント単位の最新助言読み出し確認
func TestCurrentAdviceUsesTenantScopedLatestRead(t *testing.T) {
	repo := &adviceRepoStub{saved: &domain.Advice{UserID: "u", Content: "focus"}}
	// 保存済み助言の取得確認
	got, err := (Advices{Repo: repo}).Current(context.Background(), "u", nil)
	if err != nil || got.Content != "focus" {
		t.Fatalf("current=%+v err=%v", got, err)
	}
	// 未認証の拒否確認
	if _, err := (Advices{Repo: repo}).Current(context.Background(), "", nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("missing identity error=%v", err)
	}
}

// 期限切れ削除の固定件数返却
func (r *adviceRepoStub) DeleteExpiredAdvice(context.Context, time.Time) (int64, error) {
	return 1, nil
}

// ソースハッシュ重複排除と30日期限の確認
func TestAdviceSourceHashDeduplicatesAndExpiresInThirtyDays(t *testing.T) {
	now := time.Date(2026, 8, 13, 4, 0, 0, 0, time.UTC)
	provider := &providerStub{}
	repo := &adviceRepoStub{}
	service := Advices{Provider: provider, Repo: repo, Now: func() time.Time { return now }}
	// 初回生成の実行
	first, err := service.Generate(context.Background(), "u", nil, map[string]any{"tasks": []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	// 同一ソースの再利用と期限確認
	second, err := service.Generate(context.Background(), "u", nil, map[string]any{"tasks": []string{"a"}})
	if err != nil || provider.calls != 1 || first.SourceHash != second.SourceHash || !first.ExpiresAt.Equal(now.AddDate(0, 0, 30)) {
		t.Fatal(first, second, provider.calls, err)
	}
}
