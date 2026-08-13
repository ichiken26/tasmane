package application

import (
	"context"
	"tasmane/internal/domain"
	"time"
)

// RelationRepository is the single persistence boundary for hierarchy and dependency writes.
// Adapters must call Relations instead of writing relation rows directly.
type RelationRepository interface {
	LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error)
	ListTaskDependencies(context.Context, string, string) ([]domain.Dependency, error)
	SetParent(context.Context, string, string, string) error
	AddDependency(context.Context, string, domain.Dependency, int64) error
}

type Relations struct{ Repo RelationRepository }

func (u Relations) Snapshot(ctx context.Context, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	if userID == "" {
		return nil, nil, domain.ErrForbidden
	}
	tasks, dependencies, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateGraphOwner(tasks, userID); err != nil {
		return nil, nil, err
	}
	return tasks, dependencies, nil
}

func (u Relations) AssignParent(ctx context.Context, userID, taskID, parentID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	tasks, _, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	if err := validateGraphOwner(tasks, userID); err != nil {
		return err
	}
	if err := domain.ValidateHierarchyAssignment(tasks, taskID, parentID); err != nil {
		return err
	}
	return u.Repo.SetParent(ctx, userID, taskID, parentID)
}

func (u Relations) AddDependency(ctx context.Context, userID, taskID, dependsOnTaskID string, expectedVersion int64) error {
	if userID == "" || expectedVersion < 1 {
		return domain.ErrForbidden
	}
	tasks, dependencies, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	if err := validateGraphOwner(tasks, userID); err != nil {
		return err
	}
	if err := domain.ValidateDependencyAddition(tasks, dependencies, taskID, dependsOnTaskID); err != nil {
		return err
	}
	return u.Repo.AddDependency(ctx, userID, domain.Dependency{TaskID: taskID, DependsOnTaskID: dependsOnTaskID}, expectedVersion)
}

func (u Relations) ListDependencies(ctx context.Context, userID, taskID string) ([]domain.Dependency, error) {
	if userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListTaskDependencies(ctx, userID, taskID)
}

func validateGraphOwner(tasks []domain.GraphTask, userID string) error {
	for _, task := range tasks {
		if task.UserID != userID {
			return domain.ErrGraphOwnership
		}
	}
	return nil
}

type ConfirmationRepository interface {
	CreateConfirmation(context.Context, string, string, string, string, *time.Time) (domain.Confirmation, error)
	GetConfirmation(context.Context, string, string) (domain.Confirmation, error)
	ListConfirmations(context.Context, string, string) ([]domain.Confirmation, error)
	UpdateConfirmation(context.Context, string, string, domain.Confirmation) error
}

type Confirmations struct{ Repo ConfirmationRepository }

func (u Confirmations) Create(ctx context.Context, userID, taskID, content, targetName string, dueAt *time.Time) (domain.Confirmation, error) {
	if userID == "" {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	return u.Repo.CreateConfirmation(ctx, userID, taskID, content, targetName, dueAt)
}

func (u Confirmations) List(ctx context.Context, userID, taskID string) ([]domain.Confirmation, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListConfirmations(ctx, userID, taskID)
}

func (u Confirmations) Transition(ctx context.Context, userID, confirmationID string, to domain.ConfirmationStatus, at time.Time) (domain.Confirmation, error) {
	if userID == "" || confirmationID == "" || at.IsZero() {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	current, err := u.Repo.GetConfirmation(ctx, userID, confirmationID)
	if err != nil {
		return domain.Confirmation{}, err
	}
	next, err := current.Transition(to, at)
	if err != nil {
		return domain.Confirmation{}, err
	}
	if err := u.Repo.UpdateConfirmation(ctx, userID, confirmationID, next); err != nil {
		return domain.Confirmation{}, err
	}
	return next, nil
}
