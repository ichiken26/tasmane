package domain

import (
	"errors"
	"time"
)

var ErrGoogleHierarchyConflict = errors.New("google date change requires hierarchy propagation")

// 祖先変更を伴わないGoogle期限更新の計画
func PlanGoogleDueChange(tasks []DateTask, taskID string, endAt *time.Time) ([]DateChange, error) {
	// 対象の開始時刻保持と日付変更計画
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
	// 祖先集合の構築
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
	// 祖先への伝播有無の判定
	for _, change := range changes {
		if ancestors[change.TaskID] {
			return nil, ErrGoogleHierarchyConflict
		}
	}
	return changes, nil
}

// 専用Googleタスクリスト名
const DedicatedGoogleTaskListName = "Tasmane"

// Googleタスクリスト操作の種別
type GoogleTaskListAction string

const (
	CreateGoogleTaskList GoogleTaskListAction = "CREATE"
	ReuseGoogleTaskList  GoogleTaskListAction = "REUSE"
)

// 専用Googleタスクリストの確保計画
type GoogleTaskListPlan struct {
	Action     GoogleTaskListAction
	TaskListID string
	Name       string
}

// 専用タスクリストの作成または再利用計画
func PlanDedicatedGoogleTaskList(storedID string, available bool) GoogleTaskListPlan {
	if storedID != "" && available {
		return GoogleTaskListPlan{Action: ReuseGoogleTaskList, TaskListID: storedID, Name: DedicatedGoogleTaskListName}
	}
	return GoogleTaskListPlan{Action: CreateGoogleTaskList, Name: DedicatedGoogleTaskListName}
}
