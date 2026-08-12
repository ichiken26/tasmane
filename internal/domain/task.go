package domain

import (
	"errors"
	"strings"
	"time"
)

type Status string

const (
	NotStarted Status = "NOT_STARTED"
	InProgress Status = "IN_PROGRESS"
	Completed  Status = "COMPLETED"
)

var (
	ErrInvalidTransition          = errors.New("invalid task transition")
	ErrNotCompletable             = errors.New("task is not completable")
	ErrVersionConflict            = errors.New("version conflict")
	ErrForbidden                  = errors.New("forbidden")
	ErrParentNeedsChildren        = errors.New("parent needs children")
	ErrInvalidProgress            = errors.New("invalid progress")
	ErrInvalidPriority            = errors.New("invalid priority")
	ErrInvalidTaskInput           = errors.New("invalid task input")
	ErrImpactConfirmationRequired = errors.New("impact confirmation required")
)

type TaskDraft struct {
	GenreID        string
	TaskTypeID     *string
	ParentTaskID   *string
	Title          string
	Body           *string
	StartAt        *time.Time
	EndAt          *time.Time
	IsAllDay       bool
	ManualPriority *Priority
}

type TaskCreateAuthorization struct {
	AllowLeafConversion bool
	ParentVersion       int64
}

type TaskSearch struct {
	Query    string
	GenreIDs []string
	Priority *Priority
	Deadline string
}

type TaskUpdate struct {
	Task        Task
	DateChanges []DateChange
}

func NewTask(id, userID string, draft TaskDraft) (Task, error) {
	if id == "" || userID == "" {
		return Task{}, ErrInvalidTaskInput
	}
	if err := ValidateTaskDraft(&draft); err != nil {
		return Task{}, err
	}
	draft.Title = strings.TrimSpace(draft.Title)
	return Task{
		ID: id, UserID: userID, GenreID: draft.GenreID, TaskTypeID: draft.TaskTypeID,
		ParentTaskID: draft.ParentTaskID, Title: draft.Title, Body: draft.Body,
		Status: NotStarted, StartAt: draft.StartAt, EndAt: draft.EndAt,
		IsAllDay: draft.IsAllDay, ManualPriority: draft.ManualPriority, Version: 1,
	}, nil
}

func ValidateTaskDraft(draft *TaskDraft) error {
	if draft == nil {
		return ErrInvalidTaskInput
	}
	draft.Title = strings.TrimSpace(draft.Title)
	if draft.GenreID == "" || draft.Title == "" {
		return ErrInvalidTaskInput
	}
	if invalidDateRange(draft.StartAt, draft.EndAt) {
		return ErrInvalidDateRange
	}
	if draft.ManualPriority != nil {
		if _, ok := priorityWeight(*draft.ManualPriority); !ok {
			return ErrInvalidPriority
		}
	}
	return nil
}

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
	GenreID                 string
	TaskTypeID              *string
	ParentTaskID            *string
	Title                   string
	Body                    *string
	Status                  Status
	Progress                *int
	StartAt                 *time.Time
	EndAt                   *time.Time
	IsAllDay                bool
	ManualPriority          *Priority
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
