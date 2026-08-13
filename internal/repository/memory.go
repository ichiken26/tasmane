package repository

import (
	"context"
	"fmt"
	"sync"
	"tasmane/internal/domain"
)

const OptimisticUpdateSQL = `UPDATE tasks SET status = $1, leaf_progress = $2, version = version + 1, updated_at = now() WHERE id = $3 AND user_id = $4 AND version = $5 AND deleted_at IS NULL`

type Memory struct {
	mu    sync.Mutex
	Tasks map[string]domain.Task
}

func NewMemory(tasks ...domain.Task) *Memory {
	m := &Memory{Tasks: map[string]domain.Task{}}
	for _, t := range tasks {
		m.Tasks[t.ID] = t
	}
	return m
}
func (m *Memory) Create(_ context.Context, userID string, draft domain.TaskDraft, _ domain.TaskCreateAuthorization) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("task-%d", len(m.Tasks)+1)
	task, err := domain.NewTask(id, userID, draft)
	if err != nil {
		return domain.Task{}, err
	}
	m.Tasks[id] = task
	return task, nil
}
func (m *Memory) Get(_ context.Context, userID, id string) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.Tasks[id]
	if !ok || t.UserID != userID {
		return domain.Task{}, domain.ErrForbidden
	}
	return t, nil
}
func (m *Memory) UpdateStatus(_ context.Context, t domain.Task, expected int64, _ string) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Tasks[t.ID]
	if !ok || current.UserID != t.UserID {
		return domain.Task{}, domain.ErrForbidden
	}
	if current.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	t.Version = expected + 1
	m.Tasks[t.ID] = t
	return t, nil
}

func (m *Memory) PreviewStatusTransition(_ context.Context, userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusTransitionPlan(userID, taskID, expected, to)
}

func (m *Memory) statusTransitionPlan(userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	target, ok := m.Tasks[taskID]
	if !ok || target.UserID != userID {
		return domain.StatusTransitionPlan{}, domain.ErrForbidden
	}
	if target.Version != expected {
		return domain.StatusTransitionPlan{}, domain.ErrVersionConflict
	}
	ancestors := []domain.Task{}
	for parentID := target.ParentTaskID; parentID != nil; {
		parent, exists := m.Tasks[*parentID]
		if !exists || parent.UserID != userID {
			return domain.StatusTransitionPlan{}, domain.ErrForbidden
		}
		ancestors = append(ancestors, parent)
		parentID = parent.ParentTaskID
	}
	descendants := []domain.Task{}
	queue := []string{taskID}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		for _, candidate := range m.Tasks {
			if candidate.UserID == userID && candidate.ParentTaskID != nil && *candidate.ParentTaskID == parentID {
				descendants = append(descendants, candidate)
				queue = append(queue, candidate.ID)
			}
		}
	}
	return domain.PlanStatusTransition(target, ancestors, descendants, to)
}

func (m *Memory) ApplyStatusTransition(_ context.Context, userID, taskID string, expected int64, to domain.Status, _ string, approved domain.StatusTransitionPlan) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.statusTransitionPlan(userID, taskID, expected, to)
	if err != nil {
		return domain.Task{}, err
	}
	if !sameStatusPlan(current, approved) {
		return domain.Task{}, domain.ErrStatusImpactPlanChanged
	}
	for _, change := range current.Changes {
		task := m.Tasks[change.TaskID]
		if task.Version != change.ExpectedVersion {
			return domain.Task{}, domain.ErrVersionConflict
		}
		task.Status, task.Progress, task.Version = change.ToStatus, change.ToProgress, task.Version+1
		m.Tasks[task.ID] = task
	}
	return m.Tasks[taskID], nil
}

func sameStatusPlan(a, b domain.StatusTransitionPlan) bool {
	if a.TaskID != b.TaskID || a.ToStatus != b.ToStatus || a.ExpectedVersion != b.ExpectedVersion || len(a.Changes) != len(b.Changes) {
		return false
	}
	for i := range a.Changes {
		x, y := a.Changes[i], b.Changes[i]
		if x.TaskID != y.TaskID || x.FromStatus != y.FromStatus || x.ToStatus != y.ToStatus || x.ExpectedVersion != y.ExpectedVersion || !sameInt(x.FromProgress, y.FromProgress) || !sameInt(x.ToProgress, y.ToProgress) {
			return false
		}
	}
	return true
}

func sameInt(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func (m *Memory) Update(_ context.Context, update domain.TaskUpdate, expected int64) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Tasks[update.Task.ID]
	if !ok || current.UserID != update.Task.UserID {
		return domain.Task{}, domain.ErrForbidden
	}
	if current.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	update.Task.Version = expected + 1
	m.Tasks[update.Task.ID] = update.Task
	return update.Task, nil
}
func (m *Memory) DeleteTask(_ context.Context, userID, taskID string, expected int64, confirmed bool) (domain.TaskDeletionPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.Tasks[taskID]
	if !ok || task.UserID != userID {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	}
	if task.Version != expected {
		return domain.TaskDeletionPlan{}, domain.ErrVersionConflict
	}
	plan := domain.TaskDeletionPlan{TaskIDs: []string{taskID}}
	if !confirmed {
		return plan, domain.ErrImpactConfirmationRequired
	}
	delete(m.Tasks, taskID)
	return plan, nil
}

func (m *Memory) GoogleUncomplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	task, err := m.Get(ctx, userID, taskID)
	if err != nil {
		return task, err
	}
	if task.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	next, err := task.Transition(domain.InProgress)
	if err != nil {
		return domain.Task{}, err
	}
	return m.UpdateStatus(ctx, next, expected, "GOOGLE_TASKS")
}
