package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
	"time"
)

func TestRollbackRequiresBoundImpactTokenAndRejectsChangedPlan(t *testing.T) {
	parentID := "parent"
	store := repository.NewMemory(
		domain.Task{ID: parentID, UserID: "u", Status: domain.Completed, HasChildren: true, Version: 2},
		domain.Task{ID: "leaf", UserID: "u", ParentTaskID: &parentID, Status: domain.Completed, Progress: appInt(100), Version: 4},
	)
	useCase := Tasks{Repo: store}
	now := time.Unix(1700000000, 0)
	tokens := ImpactTokens{Secret: []byte("12345678901234567890123456789012"), Now: func() time.Time { return now }}
	_, plan, err := useCase.TransitionWithImpact(context.Background(), "u", "leaf", 4, domain.InProgress, "WEB", "", tokens)
	if !errors.Is(err, domain.ErrImpactConfirmationRequired) || len(plan.Changes) != 2 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	token, err := tokens.Issue(ImpactClaims{UserID: "u", Operation: StatusTransitionImpactOperation(plan), ResourceID: "leaf", Version: 4})
	if err != nil {
		t.Fatal(err)
	}
	// A changed ancestor makes the approved impact set stale even though the
	// target version did not change.
	changed := store.Tasks[parentID]
	changed.Status, changed.Version = domain.InProgress, 3
	store.Tasks[parentID] = changed
	if _, _, err = useCase.TransitionWithImpact(context.Background(), "u", "leaf", 4, domain.InProgress, "WEB", token, tokens); !errors.Is(err, ErrInvalidImpactToken) {
		t.Fatalf("stale plan token accepted: %v", err)
	}
}

func TestConfirmedRollbackAppliesTargetAndAncestor(t *testing.T) {
	parentID := "parent"
	store := repository.NewMemory(
		domain.Task{ID: parentID, UserID: "u", Status: domain.Completed, HasChildren: true, Version: 2},
		domain.Task{ID: "leaf", UserID: "u", ParentTaskID: &parentID, Status: domain.Completed, Progress: appInt(100), Version: 4},
	)
	useCase := Tasks{Repo: store}
	tokens := ImpactTokens{Secret: []byte("12345678901234567890123456789012")}
	plan, err := useCase.PreviewTransition(context.Background(), "u", "leaf", 4, domain.InProgress)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := tokens.Issue(ImpactClaims{UserID: "u", Operation: StatusTransitionImpactOperation(plan), ResourceID: "leaf", Version: 4})
	got, _, err := useCase.TransitionWithImpact(context.Background(), "u", "leaf", 4, domain.InProgress, "MCP", token, tokens)
	if err != nil || got.Status != domain.InProgress || got.Progress == nil || *got.Progress != 95 || store.Tasks[parentID].Status != domain.InProgress {
		t.Fatalf("got=%+v parent=%+v err=%v", got, store.Tasks[parentID], err)
	}
}

func appInt(value int) *int { return &value }
