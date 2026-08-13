package domain

import "testing"

func TestPlanChildUncompleteReopensCompletedAncestors(t *testing.T) {
	plan, err := PlanStatusTransition(
		Task{ID: "leaf", Status: Completed, Progress: statusInt(100), Version: 4},
		[]Task{{ID: "parent", Status: Completed, HasChildren: true, Version: 8}, {ID: "root", Status: Completed, HasChildren: true, Version: 3}}, nil, InProgress,
	)
	if err != nil || !plan.RequiresConfirmation || len(plan.Changes) != 3 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if plan.Changes[0].ToProgress == nil || *plan.Changes[0].ToProgress != 95 {
		t.Fatalf("leaf must reopen at 95: %+v", plan.Changes[0])
	}
	for _, change := range plan.Changes[1:] {
		if change.ToStatus != InProgress || change.ToProgress != nil {
			t.Fatalf("parent progress must remain derived: %+v", change)
		}
	}
}

func TestPlanParentResetOnlyResetsInProgressDescendants(t *testing.T) {
	plan, err := PlanStatusTransition(
		Task{ID: "parent", Status: InProgress, HasChildren: true, Version: 2}, nil,
		[]Task{
			{ID: "active-leaf", Status: InProgress, Progress: statusInt(40), Version: 3},
			{ID: "active-parent", Status: InProgress, HasChildren: true, Version: 5},
			{ID: "done", Status: Completed, Progress: statusInt(100), Version: 7},
			{ID: "idle", Status: NotStarted, Version: 9},
		}, NotStarted,
	)
	if err != nil || !plan.RequiresConfirmation || len(plan.Changes) != 3 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	for _, change := range plan.Changes {
		if change.ToStatus != NotStarted || change.ToProgress != nil {
			t.Fatalf("reset must clear progress: %+v", change)
		}
	}
}

func TestPlanParentCompletedRollbackPreservesCompletedDescendantsAndAncestors(t *testing.T) {
	plan, err := PlanStatusTransition(
		Task{ID: "parent", Status: Completed, HasChildren: true, Version: 2},
		[]Task{{ID: "root", Status: Completed, HasChildren: true, Version: 1}},
		[]Task{{ID: "child", Status: Completed, Progress: statusInt(100), Version: 1}}, InProgress,
	)
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].TaskID != "parent" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func statusInt(value int) *int { return &value }
