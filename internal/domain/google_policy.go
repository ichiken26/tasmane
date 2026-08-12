package domain

import (
	"errors"
	"time"
)

var ErrGoogleHierarchyConflict = errors.New("google date change requires hierarchy propagation")

// PlanGoogleDueChange permits Google due updates only when no ancestor must be
// mutated. Conflict resolution then remains explicit instead of bypassing the
// hierarchy date rules.
func PlanGoogleDueChange(tasks []DateTask, taskID string, endAt *time.Time) ([]DateChange, error) {
	var startAt *time.Time
	for _, task := range tasks {
		if task.ID == taskID {
			startAt = task.StartAt
			break
		}
	}
	changes, err := PlanDateChange(tasks, taskID, startAt, endAt)
	if err != nil {
		return nil, err
	}
	var current *DateTask
	ancestors := map[string]bool{}
	byID := make(map[string]DateTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
		if task.ID == taskID {
			copy := task
			current = &copy
		}
	}
	if current == nil {
		return nil, ErrInvalidDatePropagation
	}
	for parentID := current.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
		ancestors[parentID] = true
	}
	for _, change := range changes {
		if ancestors[change.TaskID] {
			return nil, ErrGoogleHierarchyConflict
		}
	}
	return changes, nil
}

const DedicatedGoogleTaskListName = "Tasmane"

type GoogleTaskListAction string

const (
	CreateGoogleTaskList GoogleTaskListAction = "CREATE"
	ReuseGoogleTaskList  GoogleTaskListAction = "REUSE"
)

type GoogleTaskListPlan struct {
	Action     GoogleTaskListAction
	TaskListID string
	Name       string
}

func PlanDedicatedGoogleTaskList(storedID string, available bool) GoogleTaskListPlan {
	if storedID != "" && available {
		return GoogleTaskListPlan{Action: ReuseGoogleTaskList, TaskListID: storedID, Name: DedicatedGoogleTaskListName}
	}
	return GoogleTaskListPlan{Action: CreateGoogleTaskList, Name: DedicatedGoogleTaskListName}
}
