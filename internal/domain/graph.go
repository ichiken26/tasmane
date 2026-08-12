package domain

import "errors"

var (
	ErrHierarchyCycle    = errors.New("hierarchy cycle")
	ErrDependencyCycle   = errors.New("dependency cycle")
	ErrGraphOwnership    = errors.New("graph tasks must have the same user")
	ErrGraphGenre        = errors.New("graph tasks must have the same genre")
	ErrGraphTaskNotFound = errors.New("graph task not found")
	ErrInvalidGraphInput = errors.New("invalid graph input")
)

type GraphTask struct {
	ID, UserID, GenreID, ParentID string
}

type Dependency struct {
	TaskID, DependsOnTaskID string
}

func ValidateHierarchyAssignment(tasks []GraphTask, taskID, parentID string) error {
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

func ValidateDependencyAddition(tasks []GraphTask, dependencies []Dependency, taskID, dependsOnTaskID string) error {
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
