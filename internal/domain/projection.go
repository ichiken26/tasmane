package domain

import (
	"errors"
	"sort"
	"time"
)

var ErrInvalidProjectionInput = errors.New("invalid projection input")

// 算出値付きタスク投影（全Transport共通）
type TaskProjection struct {
	Task               Task
	Progress           *int
	CalculatedPriority Priority
	EffectivePriority  Priority
	IsOverdue          bool
	ShowFire           bool
	TaskScore          int
	Reasons            []string
	Recommended        bool
	CanComplete        bool
	// 期限なし親は子孫最短期限を仮想期限とする
	VirtualDeadline *time.Time
}

// 投影の入力
type ProjectionInput struct {
	Tasks         []Task
	Dependencies  []Dependency
	Confirmations map[string][]Confirmation
	Now           time.Time
}

// 進捗・優先度・炎・TaskScore・おすすめ順の一括投影
func ProjectTasks(input ProjectionInput) ([]TaskProjection, error) {
	if input.Now.IsZero() {
		return nil, ErrInvalidProjectionInput
	}
	byID := make(map[string]Task, len(input.Tasks))
	for _, task := range input.Tasks {
		if task.ID == "" {
			return nil, ErrInvalidProjectionInput
		}
		if _, exists := byID[task.ID]; exists {
			return nil, ErrInvalidProjectionInput
		}
		byID[task.ID] = task
	}
	// 親子索引（投影範囲外の親は根として扱う）
	children := make(map[string][]string, len(input.Tasks))
	parentOf := make(map[string]string, len(input.Tasks))
	for _, task := range input.Tasks {
		if task.ParentTaskID == nil {
			continue
		}
		if _, ok := byID[*task.ParentTaskID]; ok {
			children[*task.ParentTaskID] = append(children[*task.ParentTaskID], task.ID)
			parentOf[task.ID] = *task.ParentTaskID
		}
	}
	deadlineTasks := make([]DeadlineTask, 0, len(input.Tasks))
	for _, task := range input.Tasks {
		deadlineTasks = append(deadlineTasks, DeadlineTask{ID: task.ID, ParentID: parentOf[task.ID], Status: task.Status, EndAt: task.EndAt})
	}
	deadlineStates, err := ProjectDeadlineState(deadlineTasks, input.Now)
	if err != nil {
		return nil, err
	}

	// 依存索引（範囲外タスクの辺は無視）
	prerequisites := make(map[string][]string)
	dependents := make(map[string][]string)
	for _, edge := range input.Dependencies {
		if _, ok := byID[edge.TaskID]; !ok {
			continue
		}
		if _, ok := byID[edge.DependsOnTaskID]; !ok {
			continue
		}
		prerequisites[edge.TaskID] = append(prerequisites[edge.TaskID], edge.DependsOnTaskID)
		dependents[edge.DependsOnTaskID] = append(dependents[edge.DependsOnTaskID], edge.TaskID)
	}

	// 子孫期限・未完了子孫数の収集
	descendants := func(root string) (deadlines []time.Time, incomplete int) {
		queue := append([]string(nil), children[root]...)
		seen := map[string]bool{}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			task := byID[id]
			if task.EndAt != nil {
				deadlines = append(deadlines, *task.EndAt)
			}
			if task.Status != Completed {
				incomplete++
			}
			queue = append(queue, children[id]...)
		}
		return deadlines, incomplete
	}
	// 後続タスクの直接件数と間接距離
	dependentDistances := func(root string) (int, []int) {
		direct := len(dependents[root])
		distances := []int{}
		queueIDs := append([]string(nil), dependents[root]...)
		queueDistances := make([]int, len(queueIDs))
		for index := range queueDistances {
			queueDistances[index] = 1
		}
		seen := map[string]bool{root: true}
		for len(queueIDs) > 0 {
			id, distance := queueIDs[0], queueDistances[0]
			queueIDs, queueDistances = queueIDs[1:], queueDistances[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			if distance > 1 {
				distances = append(distances, distance)
			}
			for _, next := range dependents[id] {
				queueIDs = append(queueIDs, next)
				queueDistances = append(queueDistances, distance+1)
			}
		}
		return direct, distances
	}

	// 子から親への後順走査で進捗と実効優先度を確定
	projected := make(map[string]*TaskProjection, len(input.Tasks))
	visiting := map[string]bool{}
	var project func(string) (*TaskProjection, error)
	project = func(id string) (*TaskProjection, error) {
		if value, ok := projected[id]; ok {
			return value, nil
		}
		if visiting[id] {
			return nil, ErrInvalidProjectionInput
		}
		visiting[id] = true
		task := byID[id]
		result := &TaskProjection{Task: task}
		if len(children[id]) == 0 {
			// 葉は保存済み進捗（未着手はNULL）
			if task.Progress != nil {
				value := *task.Progress
				result.Progress = &value
			} else if task.Status == Completed {
				value := 100
				result.Progress = &value
			}
		} else {
			contributions := make([]ProgressContribution, 0, len(children[id]))
			for _, childID := range children[id] {
				child, err := project(childID)
				if err != nil {
					return nil, err
				}
				contributions = append(contributions, ProgressContribution{Progress: child.Progress, EffectivePriority: child.EffectivePriority})
			}
			value, err := ParentProgress(task.Status, contributions)
			if err != nil {
				return nil, err
			}
			result.Progress = &value
		}
		descendantDeadlines, incomplete := descendants(id)
		direct, indirect := dependentDistances(id)
		priority, err := CalculatePriority(PriorityInput{
			Now: input.Now, Deadline: task.EndAt, DescendantDeadlines: descendantDeadlines,
			Progress: result.Progress, DirectDependents: direct, IndirectDependents: len(indirect),
		})
		if err != nil {
			return nil, err
		}
		effective, err := EffectivePriority(task.ManualPriority, priority.Priority)
		if err != nil {
			return nil, err
		}
		result.CalculatedPriority, result.EffectivePriority = priority.Priority, effective
		result.VirtualDeadline = priority.Deadline
		state := deadlineStates[id]
		result.IsOverdue, result.ShowFire = state.IsOverdue, state.ShowFire

		// TaskScore（未完了前提・確認中を反映）
		unmet := []string{}
		for _, prerequisiteID := range prerequisites[id] {
			if prerequisite := byID[prerequisiteID]; prerequisite.Status != Completed {
				unmet = append(unmet, prerequisite.Title)
			}
		}
		checking, unresolved := false, 0
		for _, confirmation := range input.Confirmations[id] {
			checking = checking || confirmation.Status == ConfirmationChecking
			if confirmation.Status != ConfirmationConfirmed {
				unresolved++
			}
		}
		createdAt := task.CreatedAt
		if createdAt.IsZero() {
			createdAt = input.Now
		}
		score, err := CalculateTaskScore(TaskScoreInput{
			ID: id, Status: task.Status, EffectivePriority: effective, Deadline: priority.Deadline,
			Progress: result.Progress, CreatedAt: createdAt, Now: input.Now, DirectDependents: direct,
			IndirectDependentDistances: indirect, UnmetDependencies: unmet, HasCheckingConfirmation: checking,
		})
		if err != nil {
			return nil, err
		}
		result.TaskScore, result.Reasons, result.Recommended = score.Score, score.Reasons, score.Eligible
		// 「完了可能」: 進行中で未完了子孫・未確認確認事項なし
		result.CanComplete = task.Status == InProgress && incomplete == 0 && unresolved == 0
		visiting[id] = false
		projected[id] = result
		return result, nil
	}
	results := make([]TaskProjection, 0, len(input.Tasks))
	for _, task := range input.Tasks {
		value, err := project(task.ID)
		if err != nil {
			return nil, err
		}
		results = append(results, *value)
	}
	SortByRecommendation(results)
	return results, nil
}

// おすすめ順（対象→スコア降順→期限近い順→ID）
func SortByRecommendation(values []TaskProjection) {
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].Recommended != values[j].Recommended {
			return values[i].Recommended
		}
		if values[i].TaskScore != values[j].TaskScore {
			return values[i].TaskScore > values[j].TaskScore
		}
		return projectionDeadlineBefore(values[i], values[j])
	})
}

// 実効優先度の高い順（同順位は期限の近い順）
func SortByEffectivePriority(values []TaskProjection) {
	rank := map[Priority]int{HighPriority: 0, MediumPriority: 1, LowPriority: 2}
	sort.SliceStable(values, func(i, j int) bool {
		if rank[values[i].EffectivePriority] != rank[values[j].EffectivePriority] {
			return rank[values[i].EffectivePriority] < rank[values[j].EffectivePriority]
		}
		return projectionDeadlineBefore(values[i], values[j])
	})
}

// 期限の近い順（期限なしは後ろ）
func SortByDeadline(values []TaskProjection) {
	sort.SliceStable(values, func(i, j int) bool { return projectionDeadlineBefore(values[i], values[j]) })
}

// 期限比較（期限なしは後ろ、同値はID順）
func projectionDeadlineBefore(left, right TaskProjection) bool {
	l, r := left.Task.EndAt, right.Task.EndAt
	if l == nil != (r == nil) {
		return l != nil
	}
	if l != nil && !l.Equal(*r) {
		return l.Before(*r)
	}
	return left.Task.ID < right.Task.ID
}
