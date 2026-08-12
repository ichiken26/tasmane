package domain

import (
	"errors"
	"sort"
	"time"
)

var (
	ErrInvalidDatePropagation = errors.New("invalid date propagation")
	ErrInvalidDateRange       = errors.New("start must not be after end")
)

type DateTask struct {
	ID       string
	ParentID string
	StartAt  *time.Time
	EndAt    *time.Time
}

type DateRange struct {
	StartAt *time.Time
	EndAt   *time.Time
}

type DateChange struct {
	TaskID string
	Before DateRange
	After  DateRange
}

// PlanDateChange returns every mutation required to keep an edited task within
// its ancestors and all descendants within the edited task. The returned list
// is stable by TaskID so HTTP, MCP, and sync adapters can present one shared
// impact plan without deriving hierarchy effects themselves.
func PlanDateChange(tasks []DateTask, taskID string, startAt, endAt *time.Time) ([]DateChange, error) {
	byID := make(map[string]DateTask, len(tasks))
	children := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		if task.ID == "" || invalidDateRange(task.StartAt, task.EndAt) {
			if invalidDateRange(task.StartAt, task.EndAt) {
				return nil, ErrInvalidDateRange
			}
			return nil, ErrInvalidDatePropagation
		}
		if _, exists := byID[task.ID]; exists {
			return nil, ErrInvalidDatePropagation
		}
		byID[task.ID] = task
	}
	if taskID == "" {
		return nil, ErrInvalidDatePropagation
	}
	target, exists := byID[taskID]
	if !exists {
		return nil, ErrInvalidDatePropagation
	}
	for _, task := range tasks {
		if task.ParentID == "" {
			continue
		}
		if _, exists := byID[task.ParentID]; !exists {
			return nil, ErrInvalidDatePropagation
		}
		children[task.ParentID] = append(children[task.ParentID], task.ID)
	}
	if hasDateHierarchyCycle(byID) || invalidDateRange(startAt, endAt) {
		if invalidDateRange(startAt, endAt) {
			return nil, ErrInvalidDateRange
		}
		return nil, ErrInvalidDatePropagation
	}

	proposed := make(map[string]DateRange, len(tasks))
	for _, task := range tasks {
		proposed[task.ID] = DateRange{StartAt: task.StartAt, EndAt: task.EndAt}
	}
	proposed[taskID] = DateRange{StartAt: startAt, EndAt: endAt}

	// Extend ancestors only where the edited boundary crosses an existing one.
	for parentID := target.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
		rangeValue := proposed[parentID]
		if startAt != nil && rangeValue.StartAt != nil && startAt.Before(*rangeValue.StartAt) {
			rangeValue.StartAt = startAt
		}
		if endAt != nil && rangeValue.EndAt != nil && endAt.After(*rangeValue.EndAt) {
			rangeValue.EndAt = endAt
		}
		proposed[parentID] = rangeValue
	}

	// Contract every descendant that crosses the edited task's new boundary.
	var contractDescendants func(string)
	contractDescendants = func(parentID string) {
		for _, childID := range children[parentID] {
			rangeValue := proposed[childID]
			if startAt != nil && rangeValue.StartAt != nil && rangeValue.StartAt.Before(*startAt) {
				rangeValue.StartAt = startAt
			}
			if endAt != nil && rangeValue.EndAt != nil && rangeValue.EndAt.After(*endAt) {
				rangeValue.EndAt = endAt
			}
			proposed[childID] = rangeValue
			contractDescendants(childID)
		}
	}
	contractDescendants(taskID)

	ids := make([]string, 0, len(tasks))
	for id, after := range proposed {
		beforeTask := byID[id]
		before := DateRange{StartAt: beforeTask.StartAt, EndAt: beforeTask.EndAt}
		if invalidDateRange(after.StartAt, after.EndAt) {
			return nil, ErrInvalidDateRange
		}
		if !sameDateRange(before, after) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	changes := make([]DateChange, 0, len(ids))
	for _, id := range ids {
		before := byID[id]
		changes = append(changes, DateChange{
			TaskID: id,
			Before: DateRange{StartAt: before.StartAt, EndAt: before.EndAt},
			After:  proposed[id],
		})
	}
	return changes, nil
}

func invalidDateRange(startAt, endAt *time.Time) bool {
	return startAt != nil && endAt != nil && startAt.After(*endAt)
}

func sameDateRange(left, right DateRange) bool {
	return sameInstant(left.StartAt, right.StartAt) && sameInstant(left.EndAt, right.EndAt)
}

func sameInstant(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}

func hasDateHierarchyCycle(tasks map[string]DateTask) bool {
	for id := range tasks {
		seen := map[string]bool{}
		for current := id; current != ""; current = tasks[current].ParentID {
			if seen[current] {
				return true
			}
			seen[current] = true
		}
	}
	return false
}
