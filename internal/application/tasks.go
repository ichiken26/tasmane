package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
)

type Repository interface {
	Create(context.Context, string, domain.TaskDraft, domain.TaskCreateAuthorization) (domain.Task, error)
	Get(context.Context, string, string) (domain.Task, error)
	UpdateStatus(context.Context, domain.Task, int64, string) (domain.Task, error)
}

type TaskReadRepository interface {
	Search(context.Context, string, domain.TaskSearch) ([]domain.Task, error)
	Tree(context.Context, string, string) ([]domain.Task, error)
}

type Tasks struct{ Repo Repository }

func (u Tasks) Search(ctx context.Context, userID string, search domain.TaskSearch) ([]domain.Task, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	repository, ok := u.Repo.(TaskReadRepository)
	if !ok {
		return nil, errors.New("task read repository unavailable")
	}
	return repository.Search(ctx, userID, search)
}

func (u Tasks) Tree(ctx context.Context, userID, taskID string) ([]domain.Task, error) {
	if userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	repository, ok := u.Repo.(TaskReadRepository)
	if !ok {
		return nil, errors.New("task read repository unavailable")
	}
	return repository.Tree(ctx, userID, taskID)
}

func (u Tasks) Create(ctx context.Context, userID string, draft domain.TaskDraft, authorization domain.TaskCreateAuthorization) (domain.Task, error) {
	if userID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return domain.Task{}, err
	}
	return u.Repo.Create(ctx, userID, draft, authorization)
}

func (u Tasks) Get(ctx context.Context, userID, taskID string) (domain.Task, error) {
	if userID == "" || taskID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	return u.Repo.Get(ctx, userID, taskID)
}

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
	return u.Repo.UpdateStatus(ctx, next.ForPersistence(), expected, via)
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
