package domain

import (
	"testing"
	"time"
)

// 前提タスクの期限前に開始・後続タスクの開始後に期限を警告（保存は許可）
func TestDependencyScheduleWarnings(t *testing.T) {
	day := func(n int) *time.Time { value := time.Date(2026, 8, n, 0, 0, 0, 0, time.UTC); return &value }
	task := Task{ID: "task", Title: "本体", Status: NotStarted, StartAt: day(10), EndAt: day(20)}
	tasks := []Task{
		task,
		{ID: "pre", Title: "前提", Status: InProgress, EndAt: day(12)},
		{ID: "next", Title: "後続", Status: NotStarted, StartAt: day(15)},
		{ID: "done", Title: "完了済み前提", Status: Completed, EndAt: day(30)},
	}
	deps := []Dependency{{TaskID: "task", DependsOnTaskID: "pre"}, {TaskID: "next", DependsOnTaskID: "task"}, {TaskID: "task", DependsOnTaskID: "done"}}
	warnings := DependencyScheduleWarnings(task, tasks, deps)
	if len(warnings) != 2 {
		t.Fatalf("warnings=%v", warnings)
	}
	if got := UnmetPrerequisites("task", tasks, deps); len(got) != 1 || got[0].ID != "pre" {
		t.Fatalf("unmet=%+v", got)
	}
}
