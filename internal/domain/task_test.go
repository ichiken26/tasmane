package domain

import (
	"errors"
	"testing"
)

func TestTransitionFSMAndCompletionInvariant(t *testing.T) {
	base := Task{Status: NotStarted}
	active, err := base.Transition(InProgress)
	if err != nil || *active.Progress != 0 {
		t.Fatal(active, err)
	}
	blocked := active
	blocked.UnresolvedConfirmations = 1
	if _, err = blocked.Transition(Completed); !errors.Is(err, ErrNotCompletable) {
		t.Fatal(err)
	}
	done, err := active.Transition(Completed)
	if err != nil || *done.Progress != 100 {
		t.Fatal(done, err)
	}
	reopened, err := done.Transition(InProgress)
	if err != nil || *reopened.Progress != 95 {
		t.Fatal(reopened, err)
	}
	if _, err = base.Transition(Completed); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal(err)
	}
}

func TestParentTransitionNeverPersistsDerivedProgress(t *testing.T) {
	parent := Task{ID: "parent", Status: NotStarted, Progress: intPointer(75), HasChildren: true}
	active, err := parent.Transition(InProgress)
	if err != nil {
		t.Fatal(err)
	}
	persisted := active.ForPersistence()
	if persisted.Status != InProgress || persisted.Progress != nil {
		t.Fatalf("parent persistence must keep status and clear leaf progress: %+v", persisted)
	}

	done := Task{ID: "parent", Status: Completed, Progress: intPointer(100), HasChildren: true}
	reopened, err := done.Transition(InProgress)
	if err != nil {
		t.Fatal(err)
	}
	if persisted = reopened.ForPersistence(); persisted.Status != InProgress || persisted.Progress != nil {
		t.Fatalf("reopened parent persisted derived progress: %+v", persisted)
	}

	leaf := Task{ID: "leaf", Status: InProgress, Progress: intPointer(40)}.ForPersistence()
	if leaf.Progress == nil || *leaf.Progress != 40 {
		t.Fatalf("leaf progress was cleared: %+v", leaf)
	}
}

func intPointer(value int) *int { return &value }

func TestParentProgressUsesEffectivePriorityWeights(t *testing.T) {
	children := []ProgressContribution{
		{Progress: intPointer(100), EffectivePriority: HighPriority},
		{Progress: intPointer(50), EffectivePriority: MediumPriority},
		{Progress: nil, EffectivePriority: LowPriority},
	}
	progress, err := ParentProgress(InProgress, children)
	if err != nil || progress != 72 { // floor((100*5 + 50*3 + 0*1) / 9)
		t.Fatalf("progress=%d err=%v", progress, err)
	}
	progress, err = ParentProgress(InProgress, []ProgressContribution{{Progress: intPointer(100), EffectivePriority: HighPriority}})
	if err != nil || progress != 95 {
		t.Fatalf("in-progress cap: progress=%d err=%v", progress, err)
	}
	progress, err = ParentProgress(Completed, children)
	if err != nil || progress != 100 {
		t.Fatalf("completed: progress=%d err=%v", progress, err)
	}
}

func TestParentProgressRejectsMissingChildrenAndInvalidInputs(t *testing.T) {
	if _, err := ParentProgress(InProgress, nil); !errors.Is(err, ErrParentNeedsChildren) {
		t.Fatal(err)
	}
	if _, err := ParentProgress(NotStarted, []ProgressContribution{{EffectivePriority: MediumPriority}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ParentProgress(InProgress, []ProgressContribution{{Progress: intPointer(101), EffectivePriority: LowPriority}}); !errors.Is(err, ErrInvalidProgress) {
		t.Fatal(err)
	}
	if _, err := ParentProgress(InProgress, []ProgressContribution{{Progress: intPointer(50), EffectivePriority: Priority("URGENT")}}); !errors.Is(err, ErrInvalidPriority) {
		t.Fatal(err)
	}
}

func TestRollbackPropagation(t *testing.T) {
	ancestors := []Task{
		{ID: "parent", Status: Completed, Progress: intPointer(100)},
		{ID: "root", Status: Completed, Progress: intPointer(100)},
	}
	rolledBack := RollbackCompletedAncestors(ancestors)
	for _, task := range rolledBack {
		if task.Status != InProgress || task.Progress != nil {
			t.Fatalf("ancestor was not reopened: %+v", task)
		}
	}
	if ancestors[0].Status != Completed {
		t.Fatal("input mutated")
	}

	descendants := []Task{
		{ID: "active", Status: InProgress, Progress: intPointer(40)},
		{ID: "done", Status: Completed, Progress: intPointer(100)},
		{ID: "idle", Status: NotStarted},
	}
	rolledBack = RollbackInProgressDescendants(descendants)
	if rolledBack[0].Status != NotStarted || rolledBack[0].Progress != nil {
		t.Fatalf("active descendant was not reset: %+v", rolledBack[0])
	}
	if rolledBack[1].Status != Completed || rolledBack[1].Progress == nil || *rolledBack[1].Progress != 100 {
		t.Fatalf("completed descendant changed: %+v", rolledBack[1])
	}
}
