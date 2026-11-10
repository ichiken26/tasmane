package domain

import (
	"errors"
	"testing"
	"time"
)

// 期限超過子孫からの炎伝播検証
func TestProjectDeadlineStatePropagatesFireFromOverdueDescendants(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Second), now.Add(time.Second)
	tasks := []DeadlineTask{
		{ID: "root", Status: InProgress, EndAt: &future},
		{ID: "child", ParentID: "root", Status: InProgress, EndAt: &future},
		{ID: "late", ParentID: "child", Status: NotStarted, EndAt: &past},
		{ID: "done-late", ParentID: "root", Status: Completed, EndAt: &past},
		{ID: "root-2", Status: InProgress, EndAt: &future},
		{ID: "done-branch", ParentID: "root-2", Status: Completed, EndAt: &future},
		{ID: "late-below-done", ParentID: "done-branch", Status: InProgress, EndAt: &past},
		{ID: "boundary", Status: InProgress, EndAt: &now},
	}

	got, err := ProjectDeadlineState(tasks, now)
	if err != nil {
		t.Fatal(err)
	}
	assertDeadlineState(t, got, "late", true, true)
	assertDeadlineState(t, got, "child", false, true)
	assertDeadlineState(t, got, "root", false, true)
	assertDeadlineState(t, got, "done-late", false, false)
	assertDeadlineState(t, got, "root-2", false, true)
	assertDeadlineState(t, got, "done-branch", false, false)
	assertDeadlineState(t, got, "late-below-done", true, true)
	assertDeadlineState(t, got, "boundary", false, false)
}

// 不正階層の期限投影拒否検証
func TestProjectDeadlineStateRejectsInvalidHierarchy(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		tasks []DeadlineTask
	}{
		{"empty id", []DeadlineTask{{Status: InProgress}}},
		{"duplicate id", []DeadlineTask{{ID: "a"}, {ID: "a"}}},
		{"missing parent", []DeadlineTask{{ID: "a", ParentID: "missing"}}},
		{"cycle", []DeadlineTask{{ID: "a", ParentID: "b"}, {ID: "b", ParentID: "a"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ProjectDeadlineState(tc.tasks, now); !errors.Is(err, ErrInvalidDeadlineProjection) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

// ゼロ基準時刻の拒否検証
func TestProjectDeadlineStateRejectsZeroNow(t *testing.T) {
	if _, err := ProjectDeadlineState(nil, time.Time{}); !errors.Is(err, ErrInvalidDeadlineProjection) {
		t.Fatalf("got %v", err)
	}
}

// 期限状態の期待値照合
func assertDeadlineState(t *testing.T, states map[string]DeadlineState, id string, overdue, fire bool) {
	t.Helper()
	got, ok := states[id]
	if !ok || got.IsOverdue != overdue || got.ShowFire != fire {
		t.Fatalf("%s: got %+v, present=%v; want overdue=%v fire=%v", id, got, ok, overdue, fire)
	}
}
