package domain

import "testing"

// 葉進捗は進行中の葉だけが0..95の5刻みで設定できる
func TestValidateLeafProgress(t *testing.T) {
	leaf := Task{ID: "t", Status: InProgress}
	for _, tc := range []struct {
		task     Task
		progress int
		ok       bool
	}{
		{leaf, 0, true}, {leaf, 50, true}, {leaf, 95, true},
		{leaf, 100, false}, {leaf, 33, false}, {leaf, -5, false},
		{Task{ID: "t", Status: NotStarted}, 10, false},
		{Task{ID: "t", Status: Completed}, 95, false},
		{Task{ID: "t", Status: InProgress, HasChildren: true}, 10, false},
	} {
		if err := ValidateLeafProgress(tc.task, tc.progress); (err == nil) != tc.ok {
			t.Fatalf("status=%s children=%v progress=%d err=%v", tc.task.Status, tc.task.HasChildren, tc.progress, err)
		}
	}
}

// 子を失った親は状態に整合する葉進捗へ正規化
func TestNormalizedLeafProgress(t *testing.T) {
	if NormalizedLeafProgress(NotStarted) != nil {
		t.Fatal("NOT_STARTED must be NULL")
	}
	if got := NormalizedLeafProgress(InProgress); got == nil || *got != 0 {
		t.Fatalf("IN_PROGRESS=%v", got)
	}
	if got := NormalizedLeafProgress(Completed); got == nil || *got != 100 {
		t.Fatalf("COMPLETED=%v", got)
	}
}
