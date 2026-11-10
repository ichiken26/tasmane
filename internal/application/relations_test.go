package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"testing"
	"time"
)

// 階層・依存関係リポジトリのテスト用スタブ
type relationRepoStub struct {
	tasks  []domain.GraphTask
	edges  []domain.Dependency
	writes int
}

// スタブグラフの返却
func (r *relationRepoStub) LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error) {
	return r.tasks, r.edges, nil
}

// スタブ依存関係の返却
func (r *relationRepoStub) ListTaskDependencies(context.Context, string, string) ([]domain.Dependency, error) {
	return r.edges, nil
}

// 親割当書き込み回数の記録
func (r *relationRepoStub) SetParent(context.Context, string, string, string) error {
	r.writes++
	return nil
}

// 依存追加書き込み回数の記録
func (r *relationRepoStub) AddDependency(context.Context, string, domain.Dependency, int64) error {
	r.writes++
	return nil
}

// 依存削除書き込み回数の記録
func (r *relationRepoStub) RemoveDependency(context.Context, string, domain.Dependency) error {
	r.writes++
	return nil
}

// 永続化前の循環検証と正当書き込みの確認
func TestRelationsValidateBeforePersistence(t *testing.T) {
	tasks := []domain.GraphTask{
		{ID: "a", UserID: "u", GenreID: "g", ParentID: "b"},
		{ID: "b", UserID: "u", GenreID: "g"},
	}
	r := &relationRepoStub{tasks: tasks, edges: []domain.Dependency{{TaskID: "a", DependsOnTaskID: "b"}}}
	u := Relations{Repo: r}
	// 階層循環の拒否確認
	if err := u.AssignParent(context.Background(), "u", "b", "a"); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatal(err)
	}
	// 依存循環の拒否確認
	if err := u.AddDependency(context.Background(), "u", "b", "a", 1); !errors.Is(err, domain.ErrDependencyCycle) {
		t.Fatal(err)
	}
	// 不正関係の非永続化確認
	if r.writes != 0 {
		t.Fatalf("invalid relations persisted: %d", r.writes)
	}
	// 正当な親割当の成功確認
	if err := u.AssignParent(context.Background(), "u", "a", "b"); err != nil {
		t.Fatal(err)
	}
	// 正当な依存追加の成功確認
	if err := u.AddDependency(context.Background(), "u", "a", "b", 1); err != nil {
		t.Fatal(err)
	}
	if r.writes != 2 {
		t.Fatalf("valid writes=%d", r.writes)
	}
}

// 認証所有者外グラフの書き込み拒否確認
func TestRelationsRejectRepositoryGraphOutsideAuthenticatedOwner(t *testing.T) {
	r := &relationRepoStub{tasks: []domain.GraphTask{
		{ID: "a", UserID: "other", GenreID: "g"},
		{ID: "b", UserID: "other", GenreID: "g"},
	}}
	u := Relations{Repo: r}
	// 親割当の所有権拒否確認
	if err := u.AssignParent(context.Background(), "authenticated", "a", "b"); !errors.Is(err, domain.ErrGraphOwnership) {
		t.Fatalf("AssignParent error = %v", err)
	}
	// 依存追加の所有権拒否確認
	if err := u.AddDependency(context.Background(), "authenticated", "a", "b", 1); !errors.Is(err, domain.ErrGraphOwnership) {
		t.Fatalf("AddDependency error = %v", err)
	}
	// 依存削除の所有権拒否確認
	if err := u.RemoveDependency(context.Background(), "authenticated", "a", "b"); !errors.Is(err, domain.ErrGraphOwnership) {
		t.Fatalf("RemoveDependency error = %v", err)
	}
	if r.writes != 0 {
		t.Fatalf("foreign graph writes = %d", r.writes)
	}
}

// 既存エッジ必須の依存削除確認
func TestRelationsRemoveDependencyRequiresExistingEdge(t *testing.T) {
	r := &relationRepoStub{
		tasks: []domain.GraphTask{{ID: "a", UserID: "u", GenreID: "g"}, {ID: "b", UserID: "u", GenreID: "g"}},
		edges: []domain.Dependency{{TaskID: "a", DependsOnTaskID: "b"}},
	}
	u := Relations{Repo: r}
	// 未存在エッジの拒否確認
	if err := u.RemoveDependency(context.Background(), "u", "a", "missing"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("missing edge error = %v", err)
	}
	// 既存エッジの削除成功確認
	if err := u.RemoveDependency(context.Background(), "u", "a", "b"); err != nil {
		t.Fatal(err)
	}
	if r.writes != 1 {
		t.Fatalf("writes=%d", r.writes)
	}
}

// 確認事項リポジトリのテスト用スタブ
type confirmationRepoStub struct {
	value  domain.Confirmation
	writes int
}

// スタブ確認事項の返却
func (r *confirmationRepoStub) CreateConfirmation(context.Context, string, string, string, string, *time.Time) (domain.Confirmation, error) {
	return r.value, nil
}

// スタブ確認事項の単件返却
func (r *confirmationRepoStub) GetConfirmation(context.Context, string, string) (domain.Confirmation, error) {
	return r.value, nil
}

// スタブ確認事項の一覧返却
func (r *confirmationRepoStub) ListConfirmations(context.Context, string, string) ([]domain.Confirmation, error) {
	return []domain.Confirmation{r.value}, nil
}

// スタブ確認事項の上書き保存
func (r *confirmationRepoStub) UpdateConfirmation(_ context.Context, _, _ string, value domain.Confirmation) error {
	r.value = value
	r.writes++
	return nil
}

// 不正遷移の非永続化と正当遷移の保存確認
func TestConfirmationsValidateTransitionBeforePersistence(t *testing.T) {
	r := &confirmationRepoStub{value: domain.Confirmation{Status: domain.ConfirmationUnconfirmed}}
	u := Confirmations{Repo: r}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	// 未確認から確認済みへの不正遷移拒否
	if _, err := u.Transition(context.Background(), "u", "c", domain.ConfirmationConfirmed, now); !errors.Is(err, domain.ErrInvalidConfirmationTransition) {
		t.Fatal(err)
	}
	if r.writes != 0 {
		t.Fatal("invalid transition persisted")
	}
	// 未確認から確認中への正当遷移確認
	got, err := u.Transition(context.Background(), "u", "c", domain.ConfirmationChecking, now)
	if err != nil || got.Status != domain.ConfirmationChecking || r.writes != 1 {
		t.Fatal(got, err, r.writes)
	}
}
