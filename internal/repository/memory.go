package repository

import (
	"context"
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
