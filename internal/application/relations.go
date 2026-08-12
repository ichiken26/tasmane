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
	SetParent(context.Context, string, string, string) error
	AddDependency(context.Context, string, domain.Dependency) error
}

type Relations struct{ Repo RelationRepository }

func (u Relations) AssignParent(ctx context.Context, userID, taskID, parentID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	tasks, _, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	if err := domain.ValidateHierarchyAssignment(tasks, taskID, parentID); err != nil {
		return err
	}
	return u.Repo.SetParent(ctx, userID, taskID, parentID)
}

func (u Relations) AddDependency(ctx context.Context, userID, taskID, dependsOnTaskID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	tasks, dependencies, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	if err := domain.ValidateDependencyAddition(tasks, dependencies, taskID, dependsOnTaskID); err != nil {
		return err
	}
	return u.Repo.AddDependency(ctx, userID, domain.Dependency{TaskID: taskID, DependsOnTaskID: dependsOnTaskID})
}

type ConfirmationRepository interface {
	GetConfirmation(context.Context, string, string) (domain.Confirmation, error)
	UpdateConfirmation(context.Context, string, string, domain.Confirmation) error
}

type Confirmations struct{ Repo ConfirmationRepository }

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
