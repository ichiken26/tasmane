package domain

import (
	"testing"
	"time"
)

// 親進捗は子の実効優先度（manual ?? calculated）で加重する
func TestProjectTasksWeightsParentProgressByChildEffectivePriority(t *testing.T) {
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	past := now.Add(-48 * time.Hour)
	parentID := "parent"
	fifty, zero := 50, 0
	tasks := []Task{
		{ID: parentID, Title: "親", Status: InProgress, Version: 1, HasChildren: true},
		// 期限切れ・手動優先度なし → calculated HIGH（重み5）
		{ID: "overdue", Title: "期限切れ子", ParentTaskID: &parentID, Status: InProgress, Progress: &zero, EndAt: &past, Version: 1},
		// 期限なし → calculated MEDIUM（重み3）
		{ID: "plain", Title: "通常子", ParentTaskID: &parentID, Status: InProgress, Progress: &fifty, Version: 1},
	}
	projected, err := ProjectTasks(ProjectionInput{Tasks: tasks, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]TaskProjection{}
	for _, value := range projected {
		byID[value.Task.ID] = value
	}
	if byID["overdue"].EffectivePriority != HighPriority || byID["plain"].EffectivePriority != MediumPriority {
		t.Fatalf("child priorities: %s %s", byID["overdue"].EffectivePriority, byID["plain"].EffectivePriority)
	}
	// (0*5 + 50*3) / (5+3) = 18
	if got := byID[parentID].Progress; got == nil || *got != 18 {
		t.Fatalf("parent progress=%v want 18", got)
	}
	if !byID[parentID].ShowFire || byID[parentID].IsOverdue {
		t.Fatalf("parent fire must propagate from overdue child: %+v", byID[parentID])
	}
}

// 完了済み葉の進捗は100、未着手はNULL
func TestProjectTasksNormalizesLeafProgressByStatus(t *testing.T) {
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	projected, err := ProjectTasks(ProjectionInput{Now: now, Tasks: []Task{
		{ID: "done", Title: "完了", Status: Completed, Version: 1},
		{ID: "todo", Title: "未着手", Status: NotStarted, Version: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range projected {
		switch value.Task.ID {
		case "done":
			if value.Progress == nil || *value.Progress != 100 || value.Recommended {
				t.Fatalf("completed leaf: %+v", value)
			}
		case "todo":
			if value.Progress != nil {
				t.Fatalf("not started leaf progress must be NULL: %+v", value)
			}
		}
	}
}

// 投影範囲外の親を持つタスク（検索の部分集合）でも炎表示を失わない
func TestProjectTasksToleratesParentsOutsideTheProjectedSet(t *testing.T) {
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	missing := "outside"
	projected, err := ProjectTasks(ProjectionInput{Now: now, Tasks: []Task{
		{ID: "child", Title: "子", ParentTaskID: &missing, Status: InProgress, EndAt: &past, Version: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !projected[0].IsOverdue || !projected[0].ShowFire {
		t.Fatalf("overdue state lost: %+v", projected[0])
	}
}

// 完了可能: 進行中かつ未完了子孫・未確認確認事項なし
func TestProjectTasksMarksCompletableTasks(t *testing.T) {
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	parentID := "parent"
	projected, err := ProjectTasks(ProjectionInput{Now: now, Tasks: []Task{
		{ID: parentID, Title: "親", Status: InProgress, Version: 1, HasChildren: true},
		{ID: "child", Title: "子", ParentTaskID: &parentID, Status: Completed, Version: 1},
		{ID: "pending", Title: "確認待ち", Status: InProgress, Version: 1},
	}, Confirmations: map[string][]Confirmation{"pending": {{Status: ConfirmationChecking}}}})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, value := range projected {
		got[value.Task.ID] = value.CanComplete
	}
	if !got[parentID] || got["pending"] || got["child"] {
		t.Fatalf("canComplete=%v", got)
	}
}
