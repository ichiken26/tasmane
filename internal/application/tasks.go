package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
)

type Repository interface {
	Get(context.Context, string, string) (domain.Task, error)
	UpdateStatus(context.Context, domain.Task, int64, string) (domain.Task, error)
}

type Tasks struct{ Repo Repository }

func (u Tasks) Transition(ctx context.Context, userID, taskID string, expected int64, to domain.Status, via string) (domain.Task, error) {
	if userID == "" || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	t, err := u.Repo.Get(ctx, userID, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if t.UserID != userID {
		return domain.Task{}, domain.ErrForbidden
	}
	next, err := t.Transition(to)
	if err != nil {
		return domain.Task{}, err
	}
	return u.Repo.UpdateStatus(ctx, next, expected, via)
}

func (u Tasks) GoogleComplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	t, err := u.Repo.Get(ctx, userID, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if t.Status == domain.NotStarted {
		t, err = u.Transition(ctx, userID, taskID, expected, domain.InProgress, "GOOGLE_TASKS")
		if err != nil {
			return domain.Task{}, err
		}
		expected = t.Version
	}
	if t.Status != domain.InProgress {
		return domain.Task{}, errors.New("google completion requires active task")
	}
	return u.Transition(ctx, userID, taskID, expected, domain.Completed, "GOOGLE_TASKS")
}
