package domain

import "errors"

type Status string

const (
	NotStarted Status = "NOT_STARTED"
	InProgress Status = "IN_PROGRESS"
	Completed  Status = "COMPLETED"
)

var (
	ErrInvalidTransition   = errors.New("invalid task transition")
	ErrNotCompletable      = errors.New("task is not completable")
	ErrVersionConflict     = errors.New("version conflict")
	ErrForbidden           = errors.New("forbidden")
	ErrParentNeedsChildren = errors.New("parent needs children")
	ErrInvalidProgress     = errors.New("invalid progress")
	ErrInvalidPriority     = errors.New("invalid priority")
)

type Priority string

const (
	HighPriority   Priority = "HIGH"
	MediumPriority Priority = "MEDIUM"
	LowPriority    Priority = "LOW"
)

type ProgressContribution struct {
	Progress          *int
	EffectivePriority Priority
}

type Task struct {
	ID, UserID              string
	Status                  Status
	Progress                *int
	Version                 int64
	HasChildren             bool
	IncompleteDescendants   int
	UnresolvedConfirmations int
}

// ForPersistence enforces the leaf_progress ownership boundary. Parent
// progress is always derived from children and must never be written to the
// leaf_progress column, even though transition code may carry a display value.
func (t Task) ForPersistence() Task {
	if t.HasChildren {
		t.Progress = nil
	}
	return t
}

func (t Task) Transition(to Status) (Task, error) {
	if t.Status == NotStarted && to == InProgress {
		v := 0
		t.Status, t.Progress = to, &v
		return t, nil
	}
	if t.Status == InProgress && to == Completed {
		if t.IncompleteDescendants > 0 || t.UnresolvedConfirmations > 0 {
			return Task{}, ErrNotCompletable
		}
		v := 100
		t.Status, t.Progress = to, &v
		return t, nil
	}
	if t.Status == InProgress && to == NotStarted {
		t.Status, t.Progress = to, nil
		return t, nil
	}
	if t.Status == Completed && to == InProgress {
		v := 95
		t.Status, t.Progress = to, &v
		return t, nil
	}
	return Task{}, ErrInvalidTransition
}

// ParentProgress derives a parent's display progress from its direct children.
// Integer division deliberately rounds down so an unfinished fraction is not
// presented as completed progress.
func ParentProgress(status Status, children []ProgressContribution) (int, error) {
	if len(children) == 0 {
		return 0, ErrParentNeedsChildren
	}
	if status == Completed {
		return 100, nil
	}
	weightedProgress, totalWeight := 0, 0
	for _, child := range children {
		weight, ok := priorityWeight(child.EffectivePriority)
		if !ok {
			return 0, ErrInvalidPriority
		}
		progress := 0
		if child.Progress != nil {
			progress = *child.Progress
			if progress < 0 || progress > 100 {
				return 0, ErrInvalidProgress
			}
		}
		weightedProgress += progress * weight
		totalWeight += weight
	}
	progress := weightedProgress / totalWeight
	if progress > 95 {
		return 95, nil
	}
	return progress, nil
}

func priorityWeight(priority Priority) (int, bool) {
	switch priority {
	case HighPriority:
		return 5, true
	case MediumPriority:
		return 3, true
	case LowPriority:
		return 1, true
	default:
		return 0, false
	}
}

// RollbackCompletedAncestors applies the child COMPLETED -> IN_PROGRESS rule
// to ancestors ordered from the direct parent through the root.
func RollbackCompletedAncestors(ancestors []Task) []Task {
	result := append([]Task(nil), ancestors...)
	for index := range result {
		if result[index].Status == Completed {
			result[index].Status = InProgress
			result[index].HasChildren = true
			result[index].Progress = nil
		}
	}
	return result
}

// RollbackInProgressDescendants applies a parent's IN_PROGRESS -> NOT_STARTED
// propagation while preserving completed and already-not-started descendants.
func RollbackInProgressDescendants(descendants []Task) []Task {
	result := append([]Task(nil), descendants...)
	for index := range result {
		if result[index].Status == InProgress {
			result[index].Status = NotStarted
			result[index].Progress = nil
		}
	}
	return result
}
