package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// 優先度境界と仮想期限の検証
func TestCalculatedPriorityBoundariesAndVirtualDeadline(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		score    int
		priority Priority
	}{{34, LowPriority}, {35, MediumPriority}, {64, MediumPriority}, {65, HighPriority}} {
		if got := priorityForScore(tc.score); got != tc.priority {
			t.Fatalf("score %d: got %s want %s", tc.score, got, tc.priority)
		}
	}
	progress := 0
	week := now.Add(7 * 24 * time.Hour)
	got, err := CalculatePriority(PriorityInput{Now: now, Deadline: &week, Progress: &progress, DirectDependents: 1})
	if err != nil || got.Score != 74 || got.Priority != HighPriority {
		t.Fatalf("high boundary input: %+v %v", got, err)
	}
	got, err = CalculatePriority(PriorityInput{Now: now})
	if err != nil || got.Priority != MediumPriority {
		t.Fatalf("deadline-less priority: %+v %v", got, err)
	}
	near, later := now.Add(2*time.Hour), now.Add(2*24*time.Hour)
	got, err = CalculatePriority(PriorityInput{Now: now, DescendantDeadlines: []time.Time{later, near}})
	if err != nil || got.Deadline == nil || !got.Deadline.Equal(near) || got.Priority != HighPriority {
		t.Fatalf("virtual deadline: %+v %v", got, err)
	}
}

// 不正な依存入力の拒否検証
func TestScoringRejectsInvalidDependencyInputs(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	for _, input := range []PriorityInput{
		{Now: now, DirectDependents: -1},
		{Now: now, IndirectDependents: -1},
	} {
		if _, err := CalculatePriority(input); !errors.Is(err, ErrInvalidScoringInput) {
			t.Fatalf("input=%+v err=%v", input, err)
		}
	}
	base := TaskScoreInput{Status: InProgress, EffectivePriority: MediumPriority, CreatedAt: now, Now: now}
	base.DirectDependents = -1
	if _, err := CalculateTaskScore(base); !errors.Is(err, ErrInvalidScoringInput) {
		t.Fatal(err)
	}
	for _, distance := range []int{-1, 0, 1} {
		base.DirectDependents = 0
		base.IndirectDependentDistances = []int{distance}
		if _, err := CalculateTaskScore(base); !errors.Is(err, ErrInvalidScoringInput) {
			t.Fatalf("distance=%d err=%v", distance, err)
		}
	}
}

// 識別子・状態・時刻の不正入力拒否検証
func TestScoringRejectsInvalidIdentityStatusAndTimes(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	zero := time.Time{}
	for _, input := range []PriorityInput{
		{},
		{Now: now, Deadline: &zero},
		{Now: now, DescendantDeadlines: []time.Time{zero}},
	} {
		if _, err := CalculatePriority(input); !errors.Is(err, ErrInvalidScoringInput) {
			t.Fatalf("priority input=%+v err=%v", input, err)
		}
	}
	base := TaskScoreInput{ID: "task-1", Status: InProgress, EffectivePriority: MediumPriority, CreatedAt: now, Now: now}
	invalid := []TaskScoreInput{
		{ID: "", Status: InProgress, EffectivePriority: MediumPriority, CreatedAt: now, Now: now},
		{ID: "task-1", Status: Status("UNKNOWN"), EffectivePriority: MediumPriority, CreatedAt: now, Now: now},
		{ID: "task-1", Status: InProgress, EffectivePriority: MediumPriority, CreatedAt: now},
		{ID: "task-1", Status: InProgress, EffectivePriority: MediumPriority, Now: now},
		{ID: "task-1", Status: Completed, EffectivePriority: MediumPriority, CreatedAt: now},
	}
	for _, input := range invalid {
		if _, err := CalculateTaskScore(input); !errors.Is(err, ErrInvalidScoringInput) {
			t.Fatalf("task input=%+v err=%v", input, err)
		}
	}
	zeroDeadline := base
	zeroDeadline.Deadline = &zero
	if _, err := CalculateTaskScore(zeroDeadline); !errors.Is(err, ErrInvalidScoringInput) {
		t.Fatalf("zero deadline err=%v", err)
	}
}

// 巨大依存入力の飽和とオーバーフロー回避検証
func TestScoringSaturatesHugeDependencyInputsWithoutOverflow(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	maxInt := int(^uint(0) >> 1)
	priority, err := CalculatePriority(PriorityInput{Now: now, DirectDependents: maxInt, IndirectDependents: maxInt})
	if err != nil || priority.Score < 0 || priority.Score > 100 {
		t.Fatalf("priority=%+v err=%v", priority, err)
	}
	result, err := CalculateTaskScore(TaskScoreInput{
		ID: "task-1", Status: InProgress, EffectivePriority: MediumPriority,
		CreatedAt: now, Now: now, DirectDependents: maxInt,
		IndirectDependentDistances: []int{2, maxInt},
	})
	if err != nil || result.Score < 0 || result.Score > 100 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

// 不正進捗の優先度算出拒否検証
func TestCalculatedPriorityRejectsInvalidProgress(t *testing.T) {
	invalid := 101
	if _, err := CalculatePriority(PriorityInput{Progress: &invalid}); err != ErrInvalidProgress {
		t.Fatal(err)
	}
}

// 有効な手動優先度の優先採用検証
func TestEffectivePriorityPrefersValidManualValue(t *testing.T) {
	manual := HighPriority
	if got, err := EffectivePriority(&manual, LowPriority); err != nil || got != HighPriority {
		t.Fatal(got, err)
	}
	if got, err := EffectivePriority(nil, MediumPriority); err != nil || got != MediumPriority {
		t.Fatal(got, err)
	}
	invalid := Priority("URGENT")
	if _, err := EffectivePriority(&invalid, LowPriority); err != ErrInvalidPriority {
		t.Fatal(err)
	}
}

// スコア理由とタイブレーク順序の検証
func TestTaskScoreReasonsBlockingAndOrderingTies(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	due := now.Add(24 * time.Hour)
	created := now.Add(-40 * 24 * time.Hour)
	result, err := CalculateTaskScore(TaskScoreInput{
		ID: "b", Status: InProgress, EffectivePriority: HighPriority, Deadline: &due,
		Progress: intPointer(10), CreatedAt: created, Now: now,
		UnmetDependencies: []string{"前提B", "前提A"}, DirectDependents: 3, IndirectDependentDistances: []int{2, 3},
	})
	if err != nil || !result.Eligible || result.Score < 0 || result.Score > 100 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	wantReasons := []string{"期限が近い", "優先度が高い", "進捗が低い", "複数タスクをブロック中", "前提タスク前提Aが未完了", "前提タスク前提Bが未完了", "長期間未着手"}
	if !reflect.DeepEqual(result.Reasons, wantReasons) {
		t.Fatalf("reasons=%v", result.Reasons)
	}
	done, err := CalculateTaskScore(TaskScoreInput{ID: "done", Status: Completed, CreatedAt: now, Now: now})
	if err != nil || done.Eligible {
		t.Fatalf("completed=%+v err=%v", done, err)
	}
	later := now.Add(48 * time.Hour)
	items := []TaskScoreResult{{ID: "c", Score: 80, Deadline: nil, Eligible: true}, {ID: "b", Score: 80, Deadline: &later, Eligible: true}, {ID: "a", Score: 80, Deadline: &due, Eligible: true}}
	SortTaskScores(items)
	if ids := []string{items[0].ID, items[1].ID, items[2].ID}; !reflect.DeepEqual(ids, []string{"a", "b", "c"}) {
		t.Fatal(ids)
	}
}

// 期限切れ時の確認待ちペナルティ緩和検証
func TestTaskScoreCheckingPenaltyIsRelaxedWhenOverdue(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	base := TaskScoreInput{ID: "x", Status: InProgress, EffectivePriority: MediumPriority, CreatedAt: now, Now: now, HasCheckingConfirmation: true}
	base.Deadline = &future
	normal, _ := CalculateTaskScore(base)
	base.Deadline = &past
	overdue, _ := CalculateTaskScore(base)
	if overdue.Score <= normal.Score || !containsReason(overdue.Reasons, "期限切れ") {
		t.Fatalf("normal=%+v overdue=%+v", normal, overdue)
	}
}

// タスクスコア加重合成の検証
func TestTaskScoreWeightComposition(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	neutral, err := CalculateTaskScore(TaskScoreInput{
		ID: "neutral", Status: InProgress, EffectivePriority: MediumPriority,
		CreatedAt: now, Now: now,
	})
	if err != nil || neutral.Score != 48 {
		t.Fatalf("neutral score = %d, err = %v", neutral.Score, err)
	}

	due := now.Add(24 * time.Hour)
	maxComponents, err := CalculateTaskScore(TaskScoreInput{
		ID: "max-components", Status: InProgress, EffectivePriority: HighPriority,
		Deadline: &due, Progress: intPointer(0), CreatedAt: now.Add(-30 * 24 * time.Hour),
		Now: now, DirectDependents: 4,
	})
	if err != nil || maxComponents.Score != 98 {
		t.Fatalf("weighted score = %d, err = %v", maxComponents.Score, err)
	}
}

// 理由文字列の含有判定
func containsReason(reasons []string, target string) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}
