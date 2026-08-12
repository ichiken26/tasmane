package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestOD001GenreMoveIncludesSubtreeAndBoundaryDependencies(t *testing.T) {
	tasks := []GraphTask{
		{ID: "root", UserID: "u", GenreID: "old"},
		{ID: "move", UserID: "u", GenreID: "old", ParentID: "root"},
		{ID: "child", UserID: "u", GenreID: "old", ParentID: "move"},
		{ID: "outside", UserID: "u", GenreID: "old"},
	}
	dependencies := []Dependency{
		{TaskID: "move", DependsOnTaskID: "child"},
		{TaskID: "outside", DependsOnTaskID: "child"},
	}
	plan, err := PlanGenreMove(tasks, dependencies, "move", "new")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.MovedTaskIDs, []string{"child", "move"}) || plan.DetachedParentID != "root" {
		t.Fatalf("unexpected move plan: %+v", plan)
	}
	if !reflect.DeepEqual(plan.RemovedDependencies, []Dependency{{TaskID: "outside", DependsOnTaskID: "child"}}) {
		t.Fatalf("unexpected boundary edges: %+v", plan.RemovedDependencies)
	}
}

func TestOD002GoogleDueRejectsAncestorPropagation(t *testing.T) {
	d5, d10, d12 := day(5), day(10), day(12)
	tasks := []DateTask{
		{ID: "parent", EndAt: &d10},
		{ID: "child", ParentID: "parent", EndAt: &d5},
	}
	if _, err := PlanGoogleDueChange(tasks, "child", &d12); !errors.Is(err, ErrGoogleHierarchyConflict) {
		t.Fatalf("got %v", err)
	}
	changes, err := PlanGoogleDueChange(tasks, "child", &d5)
	if err != nil || len(changes) != 0 {
		t.Fatalf("no-propagation due change failed: changes=%+v err=%v", changes, err)
	}
}

func TestOD002GoogleDueRejectsMalformedHierarchyWithoutLooping(t *testing.T) {
	d5 := day(5)
	tasks := []DateTask{
		{ID: "a", ParentID: "b", EndAt: &d5},
		{ID: "b", ParentID: "a", EndAt: &d5},
	}
	if _, err := PlanGoogleDueChange(tasks, "a", &d5); !errors.Is(err, ErrInvalidDatePropagation) {
		t.Fatalf("got %v", err)
	}
}

func TestOD003DedicatedGoogleTaskListCreateAndReuse(t *testing.T) {
	create := PlanDedicatedGoogleTaskList("", false)
	if create.Action != CreateGoogleTaskList || create.Name != DedicatedGoogleTaskListName {
		t.Fatalf("unexpected create plan: %+v", create)
	}
	reuse := PlanDedicatedGoogleTaskList("list-1", true)
	if reuse.Action != ReuseGoogleTaskList || reuse.TaskListID != "list-1" {
		t.Fatalf("unexpected reuse plan: %+v", reuse)
	}
	recover := PlanDedicatedGoogleTaskList("deleted-list", false)
	if recover.Action != CreateGoogleTaskList || recover.TaskListID != "" {
		t.Fatalf("missing stored list must be recreated: %+v", recover)
	}
}

func TestOD006LeafToParentRequiresConfirmationAndClearsProgress(t *testing.T) {
	leaf := Task{ID: "leaf", Status: InProgress, Progress: intPointer(40)}
	plan, err := PlanFirstChildAddition(leaf)
	if err != nil || !plan.RequiresConfirmation || !plan.ClearLeafProgress {
		t.Fatalf("unexpected leaf conversion plan: %+v err=%v", plan, err)
	}
	if _, err := PlanFirstChildAddition(Task{Status: Completed}); !errors.Is(err, ErrNotCompletable) {
		t.Fatalf("completed leaf accepted child: %v", err)
	}
}

func TestOD007DeleteImpactIncludesDescendantsAndRecursiveDependents(t *testing.T) {
	tasks := []GraphTask{
		{ID: "target", UserID: "u", GenreID: "g"},
		{ID: "child", UserID: "u", GenreID: "g", ParentID: "target"},
		{ID: "dependent", UserID: "u", GenreID: "g"},
		{ID: "dependent-child", UserID: "u", GenreID: "g", ParentID: "dependent"},
		{ID: "transitive", UserID: "u", GenreID: "g"},
	}
	dependencies := []Dependency{
		{TaskID: "dependent", DependsOnTaskID: "target"},
		{TaskID: "transitive", DependsOnTaskID: "dependent"},
		{TaskID: "transitive", DependsOnTaskID: "child"},
	}
	got, err := PlanTaskDeletion(tasks, dependencies, "target")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"child", "dependent", "dependent-child", "target", "transitive"}
	if !reflect.DeepEqual(got.TaskIDs, want) {
		t.Fatalf("got %v want %v", got.TaskIDs, want)
	}
}
