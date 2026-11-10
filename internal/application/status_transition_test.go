package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
	"time"
)

// 束縛済み影響トークン必須と計画変更拒否の確認
func TestRollbackRequiresBoundImpactTokenAndRejectsChangedPlan(t *testing.T) {
	parentID := "parent"
	store := repository.NewMemory(
		domain.Task{ID: parentID, UserID: "u", Status: domain.Completed, HasChildren: true, Version: 2},
		domain.Task{ID: "leaf", UserID: "u", ParentTaskID: &parentID, Status: domain.Completed, Progress: appInt(100), Version: 4},
	)
	useCase := Tasks{Repo: store}
	now := time.Unix(1700000000, 0)
	tokens := ImpactTokens{Secret: []byte("12345678901234567890123456789012"), Now: func() time.Time { return now }}
	// 確認必須計画の返却確認
	_, plan, err := useCase.TransitionWithImpact(context.Background(), "u", "leaf", 4, domain.InProgress, "WEB", "", tokens)
	if !errors.Is(err, domain.ErrImpactConfirmationRequired) || len(plan.Changes) != 2 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	// 計画束縛トークンの発行
	token, err := tokens.Issue(ImpactClaims{UserID: "u", Operation: StatusTransitionImpactOperation(plan), ResourceID: "leaf", Version: 4})
	if err != nil {
		t.Fatal(err)
	}
	// 祖先変更による承認済み計画の陳腐化
	changed := store.Tasks[parentID]
	changed.Status, changed.Version = domain.InProgress, 3
	store.Tasks[parentID] = changed
	if _, _, err = useCase.TransitionWithImpact(context.Background(), "u", "leaf", 4, domain.InProgress, "WEB", token, tokens); !errors.Is(err, ErrInvalidImpactToken) {
		t.Fatalf("stale plan token accepted: %v", err)
	}
}

// 確認済み巻き戻しの対象と祖先適用確認
func TestConfirmedRollbackAppliesTargetAndAncestor(t *testing.T) {
	parentID := "parent"
	store := repository.NewMemory(
		domain.Task{ID: parentID, UserID: "u", Status: domain.Completed, HasChildren: true, Version: 2},
		domain.Task{ID: "leaf", UserID: "u", ParentTaskID: &parentID, Status: domain.Completed, Progress: appInt(100), Version: 4},
	)
	useCase := Tasks{Repo: store}
	tokens := ImpactTokens{Secret: []byte("12345678901234567890123456789012")}
	// 遷移計画の事前算出
	plan, err := useCase.PreviewTransition(context.Background(), "u", "leaf", 4, domain.InProgress)
	if err != nil {
		t.Fatal(err)
	}
	// 計画束縛トークンの発行
	token, _ := tokens.Issue(ImpactClaims{UserID: "u", Operation: StatusTransitionImpactOperation(plan), ResourceID: "leaf", Version: 4})
	// 対象と祖先への巻き戻し適用確認
	got, _, err := useCase.TransitionWithImpact(context.Background(), "u", "leaf", 4, domain.InProgress, "MCP", token, tokens)
	if err != nil || got.Status != domain.InProgress || got.Progress == nil || *got.Progress != 95 || store.Tasks[parentID].Status != domain.InProgress {
		t.Fatalf("got=%+v parent=%+v err=%v", got, store.Tasks[parentID], err)
	}
}

// テスト用整数ポインタの生成
func appInt(value int) *int { return &value }
