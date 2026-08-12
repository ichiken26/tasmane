package domain

import "sort"

type GenreMovePlan struct {
	TaskID              string
	NewGenreID          string
	DetachedParentID    string
	MovedTaskIDs        []string
	RemovedDependencies []Dependency
}

// PlanGenreMove calculates the complete atomic mutation: subtree movement,
// root detachment, and every dependency that would cross the new genre border.
func PlanGenreMove(tasks []GraphTask, dependencies []Dependency, taskID, newGenreID string) (GenreMovePlan, error) {
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

type TaskDeletionPlan struct{ TaskIDs []string }

// PlanTaskDeletion closes over hierarchy descendants and reverse dependency
// edges, yielding the exact deduplicated set that must be soft-deleted atomically.
func PlanTaskDeletion(tasks []GraphTask, dependencies []Dependency, taskID string) (TaskDeletionPlan, error) {
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

type ChildAdditionPlan struct {
	RequiresConfirmation bool
	ClearLeafProgress    bool
	Parent               Task
}

func PlanFirstChildAddition(task Task) (ChildAdditionPlan, error) {
	if task.Status == Completed {
		return ChildAdditionPlan{}, ErrNotCompletable
	}
	if task.HasChildren {
		return ChildAdditionPlan{Parent: task.ForPersistence()}, nil
	}
	task.HasChildren = true
	task = task.ForPersistence()
	return ChildAdditionPlan{
		RequiresConfirmation: true,
		ClearLeafProgress:    true,
		Parent:               task,
	}, nil
}

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

func graphChildren(tasks []GraphTask) map[string][]string {
	children := make(map[string][]string)
	for _, task := range tasks {
		if task.ParentID != "" {
			children[task.ParentID] = append(children[task.ParentID], task.ID)
		}
	}
	return children
}

func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
