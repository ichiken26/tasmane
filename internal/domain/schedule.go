package domain

import "sort"

// 依存関係と日程の矛盾警告（保存は許可する非ブロッキング警告）
func DependencyScheduleWarnings(task Task, tasks []Task, dependencies []Dependency) []string {
	byID := make(map[string]Task, len(tasks))
	for _, value := range tasks {
		byID[value.ID] = value
	}
	warnings := []string{}
	for _, edge := range dependencies {
		// 前提タスクの期限より前に開始する
		if edge.TaskID == task.ID {
			prerequisite, ok := byID[edge.DependsOnTaskID]
			if ok && prerequisite.Status != Completed && prerequisite.EndAt != nil && task.StartAt != nil && prerequisite.EndAt.After(*task.StartAt) {
				warnings = append(warnings, "前提タスク「"+prerequisite.Title+"」の期限より前に開始する予定です。")
			}
		}
		// 後続タスクの開始より後に期限がある
		if edge.DependsOnTaskID == task.ID {
			dependent, ok := byID[edge.TaskID]
			if ok && dependent.Status != Completed && dependent.StartAt != nil && task.EndAt != nil && task.EndAt.After(*dependent.StartAt) {
				warnings = append(warnings, "後続タスク「"+dependent.Title+"」の開始より後に期限が設定されています。")
			}
		}
	}
	sort.Strings(warnings)
	return warnings
}

// 着手時に未完了の前提タスク（着手は許可、警告確認が必要）
func UnmetPrerequisites(taskID string, tasks []Task, dependencies []Dependency) []Task {
	byID := make(map[string]Task, len(tasks))
	for _, value := range tasks {
		byID[value.ID] = value
	}
	result := []Task{}
	for _, edge := range dependencies {
		if edge.TaskID != taskID {
			continue
		}
		if prerequisite, ok := byID[edge.DependsOnTaskID]; ok && prerequisite.Status != Completed {
			result = append(result, prerequisite)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
