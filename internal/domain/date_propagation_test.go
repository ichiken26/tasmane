package domain

import (
	"errors"
	"testing"
	"time"
)

// 子の期間拡張の祖先伝播検証
func TestPlanDateChangePropagatesChildExpansionToAncestors(t *testing.T) {
	d3, d4, d5, d10, d15, d16, d20 := day(3), day(4), day(5), day(10), day(15), day(16), day(20)
	tasks := []DateTask{
		{ID: "root", StartAt: &d3, EndAt: &d20},
		{ID: "parent", ParentID: "root", StartAt: &d5, EndAt: &d15},
		{ID: "child", ParentID: "parent", StartAt: &d5, EndAt: &d10},
	}

	changes, err := PlanDateChange(tasks, "child", &d4, &d16)
	if err != nil {
		t.Fatal(err)
	}
	byID := indexDateChanges(changes)
	assertDates(t, byID["child"].After, &d4, &d16)
	assertDates(t, byID["parent"].After, &d4, &d16)
	if _, changed := byID["root"]; changed {
		t.Fatalf("root already contains the new child range: %+v", byID["root"])
	}
}

// 親の期間収縮の全子孫伝播検証
func TestPlanDateChangePropagatesParentContractionToAllDescendants(t *testing.T) {
	d3, d5, d7, d8, d9, d10, d15 := day(3), day(5), day(7), day(8), day(9), day(10), day(15)
	tasks := []DateTask{
		{ID: "parent", StartAt: &d3, EndAt: &d15},
		{ID: "child", ParentID: "parent", StartAt: &d5, EndAt: &d10},
		{ID: "grandchild", ParentID: "child", StartAt: &d7, EndAt: &d9},
	}

	changes, err := PlanDateChange(tasks, "parent", &d8, &d8)
	if err != nil {
		t.Fatal(err)
	}
	byID := indexDateChanges(changes)
	assertDates(t, byID["parent"].After, &d8, &d8)
	assertDates(t, byID["child"].After, &d8, &d8)
	assertDates(t, byID["grandchild"].After, &d8, &d8)
}

// 決定的な影響一覧と不正計画拒否の検証
func TestPlanDateChangeIsDeterministicAndRejectsInvalidPlans(t *testing.T) {
	d1, d2, d3 := day(1), day(2), day(3)
	tasks := []DateTask{{ID: "b", StartAt: &d1, EndAt: &d3}, {ID: "a", ParentID: "b", StartAt: &d1, EndAt: &d2}}
	changes, err := PlanDateChange(tasks, "b", &d1, &d2)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].TaskID != "b" {
		t.Fatalf("unexpected deterministic impact list: %+v", changes)
	}

	invalidRange := []DateTask{{ID: "x", StartAt: &d2, EndAt: &d3}}
	if _, err := PlanDateChange(invalidRange, "x", &d3, &d2); !errors.Is(err, ErrInvalidDateRange) {
		t.Fatalf("got %v", err)
	}
	invalidGraph := []DateTask{{ID: "x", ParentID: "missing"}}
	if _, err := PlanDateChange(invalidGraph, "x", nil, nil); !errors.Is(err, ErrInvalidDatePropagation) {
		t.Fatalf("got %v", err)
	}
}

// テスト用の日付生成
func day(value int) time.Time {
	return time.Date(2026, 8, value, 0, 0, 0, 0, time.UTC)
}

// 日付変更のタスクID索引
func indexDateChanges(changes []DateChange) map[string]DateChange {
	result := make(map[string]DateChange, len(changes))
	for _, change := range changes {
		result[change.TaskID] = change
	}
	return result
}

// 日付範囲の期待値照合
func assertDates(t *testing.T, got DateRange, start, end *time.Time) {
	t.Helper()
	if !sameInstant(got.StartAt, start) || !sameInstant(got.EndAt, end) {
		t.Fatalf("got %+v; want start=%v end=%v", got, start, end)
	}
}
