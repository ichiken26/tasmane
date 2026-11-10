package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"tasmane/internal/domain"
	"tasmane/internal/repository"
)

// 依存関係の固定スタブ
type fixedRelations struct{ deps []domain.Dependency }

func (r fixedRelations) LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error) {
	return nil, r.deps, nil
}
func (r fixedRelations) ListTaskDependencies(context.Context, string, string) ([]domain.Dependency, error) {
	return r.deps, nil
}
func (fixedRelations) SetParent(context.Context, string, string, string) error { return nil }
func (fixedRelations) AddDependency(context.Context, string, domain.Dependency, int64) error {
	return nil
}
func (fixedRelations) RemoveDependency(context.Context, string, domain.Dependency) error { return nil }

var testTokens = ImpactTokens{Secret: []byte("01234567890123456789012345678901")}

// 影響確認エラーの取り出し
func requireImpact(t *testing.T, err error) *ImpactRequiredError {
	t.Helper()
	var impact *ImpactRequiredError
	if !errors.As(err, &impact) || impact.ImpactToken == "" {
		t.Fatalf("expected impact confirmation, got %v", err)
	}
	if !errors.Is(err, domain.ErrImpactConfirmationRequired) {
		t.Fatal("impact error must match ErrImpactConfirmationRequired")
	}
	return impact
}

// 未完了の前提タスクがあるタスクへの着手は警告確認後に許可
func TestTransitionWarnsBeforeStartingWithUnmetPrerequisites(t *testing.T) {
	store := repository.NewMemory(
		domain.Task{ID: "pre", UserID: "u", GenreID: "g", Title: "前提", Status: domain.NotStarted, Version: 1},
		domain.Task{ID: "task", UserID: "u", GenreID: "g", Title: "本体", Status: domain.NotStarted, Version: 1},
	)
	commands := TaskCommands{Tasks: Tasks{Repo: store}, Relations: Relations{Repo: fixedRelations{deps: []domain.Dependency{{TaskID: "task", DependsOnTaskID: "pre"}}}}, Tokens: testTokens}
	_, err := commands.Transition(context.Background(), "u", "task", 1, domain.InProgress, "WEB", "")
	impact := requireImpact(t, err)
	if len(impact.AffectedEntities) != 1 || impact.AffectedEntities[0].ID != "pre" || *impact.AffectedEntities[0].Reason != "UNMET_DEPENDENCY" {
		t.Fatalf("entities=%+v", impact.AffectedEntities)
	}
	started, err := commands.Transition(context.Background(), "u", "task", 1, domain.InProgress, "WEB", impact.ImpactToken)
	if err != nil || started.Status != domain.InProgress {
		t.Fatalf("started=%+v err=%v", started, err)
	}
}

// 完了済みタスクへの付け替えは禁止、葉への付け替えはLeaf→Parent確認
func TestUpdateReparentRules(t *testing.T) {
	store := repository.NewMemory(
		domain.Task{ID: "done", UserID: "u", GenreID: "g", Title: "完了", Status: domain.Completed, Version: 1},
		domain.Task{ID: "leaf", UserID: "u", GenreID: "g", Title: "葉", Status: domain.InProgress, Version: 3},
		domain.Task{ID: "task", UserID: "u", GenreID: "g", Title: "移動", Status: domain.NotStarted, IsAllDay: true, Version: 1},
	)
	commands := TaskCommands{Tasks: Tasks{Repo: store}, Tokens: testTokens}
	reparent := func(parent string) TaskMutation {
		return func(task domain.Task) (domain.Task, error) { task.ParentTaskID = &parent; return task, nil }
	}
	if _, err := commands.Update(context.Background(), "u", "task", 1, reparent("done"), ""); !errors.Is(err, domain.ErrCompletedParent) {
		t.Fatalf("completed parent err=%v", err)
	}
	_, err := commands.Update(context.Background(), "u", "task", 1, reparent("leaf"), "")
	impact := requireImpact(t, err)
	if impact.Warnings[len(impact.Warnings)-1] != "進捗率は自動で再計算されます。子タスクを追加しますか？" {
		t.Fatalf("warnings=%v", impact.Warnings)
	}
	result, err := commands.Update(context.Background(), "u", "task", 1, reparent("leaf"), impact.ImpactToken)
	if err != nil || result.Task.ParentTaskID == nil || *result.Task.ParentTaskID != "leaf" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

// 子の期限が親の期限を超える作成は祖先日付伝播の確認を要求
func TestCreateChildBeyondParentEndRequiresDatePropagationImpact(t *testing.T) {
	parentEnd := time.Date(2026, 8, 10, 23, 59, 0, 0, time.UTC)
	childEnd := parentEnd.Add(72 * time.Hour)
	store := repository.NewMemory(
		domain.Task{ID: "parent", UserID: "u", GenreID: "g", Title: "親", Status: domain.InProgress, EndAt: &parentEnd, Version: 2},
		domain.Task{ID: "sibling", UserID: "u", GenreID: "g", Title: "既存子", ParentTaskID: stringRef("parent"), Status: domain.NotStarted, Version: 1},
	)
	commands := TaskCommands{Tasks: Tasks{Repo: store}, Tokens: testTokens}
	draft := domain.TaskDraft{GenreID: "g", ParentTaskID: stringRef("parent"), Title: "新しい子", EndAt: &childEnd, IsAllDay: true}
	_, err := commands.Create(context.Background(), "u", draft, "")
	impact := requireImpact(t, err)
	if impact.AffectedEntities[0].ID != "parent" || *impact.AffectedEntities[0].Reason != "DATE_PROPAGATION" || *impact.AffectedEntities[0].Title != "親" {
		t.Fatalf("entities=%+v", impact.AffectedEntities)
	}
	if _, err := commands.Create(context.Background(), "u", draft, impact.ImpactToken); err != nil {
		t.Fatalf("confirmed create err=%v", err)
	}
}

// 葉進捗は進行中の葉だけ0..95・5刻みで更新
func TestSetProgressValidatesLeafRules(t *testing.T) {
	store := repository.NewMemory(domain.Task{ID: "leaf", UserID: "u", GenreID: "g", Title: "葉", Status: domain.InProgress, Version: 1})
	commands := TaskCommands{Tasks: Tasks{Repo: store}}
	if _, err := commands.SetProgress(context.Background(), "u", "leaf", 1, 33, "WEB"); !errors.Is(err, domain.ErrInvalidProgress) {
		t.Fatalf("err=%v", err)
	}
	updated, err := commands.SetProgress(context.Background(), "u", "leaf", 1, 45, "WEB")
	if err != nil || updated.Progress == nil || *updated.Progress != 45 || updated.Version != 2 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
}

// 削除トークンは提示した削除集合に束縛される
func TestDeleteTokenIsBoundToDeletionSet(t *testing.T) {
	store := repository.NewMemory(domain.Task{ID: "task", UserID: "u", GenreID: "g", Title: "削除", Status: domain.NotStarted, Version: 1})
	commands := TaskCommands{Tasks: Tasks{Repo: store}, Tokens: testTokens}
	err := commands.Delete(context.Background(), "u", "task", 1, "")
	impact := requireImpact(t, err)
	if impact.Warnings[0] != "これらのタスクも削除されます。よろしいですか？" {
		t.Fatalf("warnings=%v", impact.Warnings)
	}
	forged, _ := testTokens.Issue(ImpactClaims{UserID: "u", Operation: "DELETE_TASK", ResourceID: "task", Version: 1})
	if err := commands.Delete(context.Background(), "u", "task", 1, forged); !errors.Is(err, ErrInvalidImpactToken) {
		t.Fatalf("legacy unbound token must be rejected: %v", err)
	}
	if err := commands.Delete(context.Background(), "u", "task", 1, impact.ImpactToken); err != nil {
		t.Fatalf("confirmed delete err=%v", err)
	}
}
