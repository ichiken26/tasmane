package domain

import "errors"

// グラフドメインのエラー群
var (
	ErrHierarchyCycle    = errors.New("hierarchy cycle")
	ErrDependencyCycle   = errors.New("dependency cycle")
	ErrGraphOwnership    = errors.New("graph tasks must have the same user")
	ErrGraphGenre        = errors.New("graph tasks must have the same genre")
	ErrGraphTaskNotFound = errors.New("graph task not found")
	ErrInvalidGraphInput = errors.New("invalid graph input")
)

// グラフ検証用のタスク
type GraphTask struct {
	ID, UserID, GenreID, ParentID string
}

// タスク間依存
type Dependency struct {
	TaskID, DependsOnTaskID string
}

// 階層割当の所有者・ジャンル・循環検証
func ValidateHierarchyAssignment(tasks []GraphTask, taskID, parentID string) error {
	// 索引と対象タスクの入力検証
	byID, err := indexGraphTasks(tasks)
	if err != nil || taskID == "" {
		return ErrInvalidGraphInput
	}
	task, ok := byID[taskID]
	if !ok {
		return ErrGraphTaskNotFound
	}
	if parentID == "" {
		return nil
	}
	// 親の所有者とジャンルの検証
	parent, ok := byID[parentID]
	if !ok {
		return ErrGraphTaskNotFound
	}
	if task.UserID != parent.UserID {
		return ErrGraphOwnership
	}
	if task.GenreID != parent.GenreID {
		return ErrGraphGenre
	}
	// 祖先経路の循環検出
	seen := map[string]bool{}
	for current := parentID; current != ""; {
		if current == taskID || seen[current] {
			return ErrHierarchyCycle
		}
		seen[current] = true
		node, exists := byID[current]
		if !exists {
			return ErrGraphTaskNotFound
		}
		if node.UserID != task.UserID {
			return ErrGraphOwnership
		}
		if node.GenreID != task.GenreID {
			return ErrGraphGenre
		}
		current = node.ParentID
	}
	return nil
}

// 依存追加の所有者・ジャンル・循環検証
func ValidateDependencyAddition(tasks []GraphTask, dependencies []Dependency, taskID, dependsOnTaskID string) error {
	// 索引と端点の入力検証
	byID, err := indexGraphTasks(tasks)
	if err != nil || taskID == "" || dependsOnTaskID == "" {
		return ErrInvalidGraphInput
	}
	task, ok := byID[taskID]
	if !ok {
		return ErrGraphTaskNotFound
	}
	dependency, ok := byID[dependsOnTaskID]
	if !ok {
		return ErrGraphTaskNotFound
	}
	if task.UserID != dependency.UserID {
		return ErrGraphOwnership
	}
	if task.GenreID != dependency.GenreID {
		return ErrGraphGenre
	}
	if taskID == dependsOnTaskID {
		return ErrDependencyCycle
	}
	// 既存辺の検証と隣接リスト構築
	adjacency := map[string][]string{}
	for _, edge := range dependencies {
		from, fromOK := byID[edge.TaskID]
		to, toOK := byID[edge.DependsOnTaskID]
		if edge.TaskID == "" || edge.DependsOnTaskID == "" {
			return ErrInvalidGraphInput
		}
		if !fromOK || !toOK {
			return ErrGraphTaskNotFound
		}
		if from.UserID != to.UserID {
			return ErrGraphOwnership
		}
		if from.GenreID != to.GenreID {
			return ErrGraphGenre
		}
		adjacency[edge.TaskID] = append(adjacency[edge.TaskID], edge.DependsOnTaskID)
	}
	adjacency[taskID] = append(adjacency[taskID], dependsOnTaskID)
	// 追加辺を含む循環検出
	visiting, visited := map[string]bool{}, map[string]bool{}
	var hasCycle func(string) bool
	hasCycle = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, next := range adjacency[id] {
			if hasCycle(next) {
				return true
			}
		}
		visiting[id], visited[id] = false, true
		return false
	}
	for id := range byID {
		if hasCycle(id) {
			return ErrDependencyCycle
		}
	}
	return nil
}

// グラフタスクの一意索引
func indexGraphTasks(tasks []GraphTask) (map[string]GraphTask, error) {
	result := make(map[string]GraphTask, len(tasks))
	for _, task := range tasks {
		if task.ID == "" || task.UserID == "" || task.GenreID == "" {
			return nil, ErrInvalidGraphInput
		}
		if _, exists := result[task.ID]; exists {
			return nil, ErrInvalidGraphInput
		}
		result[task.ID] = task
	}
	return result, nil
}
