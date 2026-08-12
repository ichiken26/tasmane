package domain

import (
	"errors"
	"time"
)

var ErrInvalidDeadlineProjection = errors.New("invalid deadline projection")

// DeadlineTask is the minimum hierarchy snapshot needed to derive overdue and
// fire state. Derived state is intentionally not persisted.
type DeadlineTask struct {
	ID       string
	ParentID string
	Status   Status
	EndAt    *time.Time
}

type DeadlineState struct {
	IsOverdue bool
	ShowFire  bool
}

// ProjectDeadlineState derives each task's own overdue state and propagates an
// overdue descendant signal to every ancestor. A task is overdue only after
// its end instant has passed; completed tasks never display fire themselves.
func ProjectDeadlineState(tasks []DeadlineTask, now time.Time) (map[string]DeadlineState, error) {
	if now.IsZero() {
		return nil, ErrInvalidDeadlineProjection
	}
	byID := make(map[string]DeadlineTask, len(tasks))
	children := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		if task.ID == "" {
			return nil, ErrInvalidDeadlineProjection
		}
		if _, exists := byID[task.ID]; exists {
			return nil, ErrInvalidDeadlineProjection
		}
		byID[task.ID] = task
	}
	for _, task := range tasks {
		if task.ParentID == "" {
			continue
		}
		if _, exists := byID[task.ParentID]; !exists {
			return nil, ErrInvalidDeadlineProjection
		}
		children[task.ParentID] = append(children[task.ParentID], task.ID)
	}

	states := make(map[string]DeadlineState, len(tasks))
	subtreeOverdue := make(map[string]bool, len(tasks))
	visiting, visited := map[string]bool{}, map[string]bool{}
	var project func(string) (bool, error)
	project = func(id string) (bool, error) {
		if visiting[id] {
			return false, ErrInvalidDeadlineProjection
		}
		if visited[id] {
			return subtreeOverdue[id], nil
		}
		visiting[id] = true
		task := byID[id]
		overdueInSubtree := task.Status != Completed && task.EndAt != nil && task.EndAt.Before(now)
		ownOverdue := overdueInSubtree
		for _, childID := range children[id] {
			childOverdue, err := project(childID)
			if err != nil {
				return false, err
			}
			overdueInSubtree = overdueInSubtree || childOverdue
		}
		visiting[id], visited[id] = false, true
		states[id] = DeadlineState{
			IsOverdue: ownOverdue,
			ShowFire:  task.Status != Completed && overdueInSubtree,
		}
		subtreeOverdue[id] = overdueInSubtree
		return overdueInSubtree, nil
	}
	for id := range byID {
		if _, err := project(id); err != nil {
			return nil, err
		}
	}
	return states, nil
}
