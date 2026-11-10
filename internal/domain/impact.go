package domain

import "sort"

// ジャンル移動の影響計画
type GenreMovePlan struct {
	TaskID              string
	NewGenreID          string
	DetachedParentID    string
	MovedTaskIDs        []string
	RemovedDependencies []Dependency
}

// 部分木移動と境界依存の原子変更計画
func PlanGenreMove(tasks []GraphTask, dependencies []Dependency, taskID, newGenreID string) (GenreMovePlan, error) {
	// 階層スナップショットと入力検証
	byID, err := validatedHierarchySnapshot(tasks)
	if err != nil {
		return GenreMovePlan{}, err
	}
	if taskID == "" || newGenreID == "" {
		return GenreMovePlan{}, ErrInvalidGraphInput
	}
	target, exists := byID[taskID]
	if !exists {
		return GenreMovePlan{}, ErrGraphTaskNotFound
	}
	if target.GenreID == newGenreID {
		return GenreMovePlan{}, ErrGraphGenre
	}
	// 移動対象部分木の収集
	children := graphChildren(tasks)
	moved := map[string]bool{}
	queue := []string{taskID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if moved[id] {
			continue
		}
		moved[id] = true
		queue = append(queue, children[id]...)
	}
	movedIDs := sortedSet(moved)
	removed := make([]Dependency, 0)
	// ジャンル境界を跨ぐ依存の抽出
	for _, dependency := range dependencies {
		from, fromOK := byID[dependency.TaskID]
		to, toOK := byID[dependency.DependsOnTaskID]
		if !fromOK || !toOK {
			return GenreMovePlan{}, ErrGraphTaskNotFound
		}
		if from.UserID != to.UserID {
			return GenreMovePlan{}, ErrGraphOwnership
		}
		if from.GenreID != to.GenreID {
			return GenreMovePlan{}, ErrGraphGenre
		}
		if moved[dependency.TaskID] != moved[dependency.DependsOnTaskID] {
			removed = append(removed, dependency)
		}
	}
	// 削除依存の安定ソート
	sort.Slice(removed, func(i, j int) bool {
		if removed[i].TaskID == removed[j].TaskID {
			return removed[i].DependsOnTaskID < removed[j].DependsOnTaskID
		}
		return removed[i].TaskID < removed[j].TaskID
	})
	return GenreMovePlan{
		TaskID: taskID, NewGenreID: newGenreID, DetachedParentID: target.ParentID,
		MovedTaskIDs: movedIDs, RemovedDependencies: removed,
	}, nil
}

// タスク削除の影響計画
type TaskDeletionPlan struct{ TaskIDs []string }

// 子孫と逆依存を含む原子削除対象の計画
func PlanTaskDeletion(tasks []GraphTask, dependencies []Dependency, taskID string) (TaskDeletionPlan, error) {
	// 階層スナップショットと入力検証
	byID, err := validatedHierarchySnapshot(tasks)
	if err != nil {
		return TaskDeletionPlan{}, err
	}
	if taskID == "" {
		return TaskDeletionPlan{}, ErrInvalidGraphInput
	}
	if _, exists := byID[taskID]; !exists {
		return TaskDeletionPlan{}, ErrGraphTaskNotFound
	}
	children := graphChildren(tasks)
	dependents := make(map[string][]string)
	// 依存関係の所有者とジャンル検証
	for _, dependency := range dependencies {
		from, fromOK := byID[dependency.TaskID]
		to, toOK := byID[dependency.DependsOnTaskID]
		if !fromOK || !toOK {
			return TaskDeletionPlan{}, ErrGraphTaskNotFound
		}
		if from.UserID != to.UserID {
			return TaskDeletionPlan{}, ErrGraphOwnership
		}
		if from.GenreID != to.GenreID {
			return TaskDeletionPlan{}, ErrGraphGenre
		}
		dependents[dependency.DependsOnTaskID] = append(dependents[dependency.DependsOnTaskID], dependency.TaskID)
	}
	// 子孫と再帰的依存先の閉包収集
	affected := map[string]bool{}
	queue := []string{taskID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if affected[id] {
			continue
		}
		affected[id] = true
		queue = append(queue, children[id]...)
		queue = append(queue, dependents[id]...)
	}
	return TaskDeletionPlan{TaskIDs: sortedSet(affected)}, nil
}

// 初回子追加の影響計画
type ChildAdditionPlan struct {
	RequiresConfirmation bool
	ClearLeafProgress    bool
	Parent               Task
}

// リーフから親への初回子追加計画
func PlanFirstChildAddition(task Task) (ChildAdditionPlan, error) {
	// 完了タスクと既存親の判定
	if task.Status == Completed {
		return ChildAdditionPlan{}, ErrNotCompletable
	}
	if task.HasChildren {
		return ChildAdditionPlan{Parent: task.ForPersistence()}, nil
	}
	// リーフ進捗のクリアと確認要求
	task.HasChildren = true
	task = task.ForPersistence()
	return ChildAdditionPlan{
		RequiresConfirmation: true,
		ClearLeafProgress:    true,
		Parent:               task,
	}, nil
}

// 階層スナップショットの検証付き索引
func validatedHierarchySnapshot(tasks []GraphTask) (map[string]GraphTask, error) {
	byID, err := indexGraphTasks(tasks)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if err := ValidateHierarchyAssignment(tasks, task.ID, task.ParentID); err != nil {
			return nil, err
		}
	}
	return byID, nil
}

// 親IDから子ID一覧への索引
func graphChildren(tasks []GraphTask) map[string][]string {
	children := make(map[string][]string)
	for _, task := range tasks {
		if task.ParentID != "" {
			children[task.ParentID] = append(children[task.ParentID], task.ID)
		}
	}
	return children
}

// 集合の安定ソート済みスライス化
func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
