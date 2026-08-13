package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"tasmane/internal/domain"
)

type Repository interface {
	Create(context.Context, string, domain.TaskDraft, domain.TaskCreateAuthorization) (domain.Task, error)
	Get(context.Context, string, string) (domain.Task, error)
	UpdateStatus(context.Context, domain.Task, int64, string) (domain.Task, error)
	Update(context.Context, domain.TaskUpdate, int64) (domain.Task, error)
	DeleteTask(context.Context, string, string, int64, bool) (domain.TaskDeletionPlan, error)
	GoogleUncomplete(context.Context, string, string, int64) (domain.Task, error)
}

func (u Tasks) GoogleUncomplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	if userID == "" || taskID == "" || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	return u.Repo.GoogleUncomplete(ctx, userID, taskID, expected)
}

func (u Tasks) Delete(ctx context.Context, userID, taskID string, expected int64, confirmed bool) (domain.TaskDeletionPlan, error) {
	if userID == "" || taskID == "" || expected < 1 {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	}
	return u.Repo.DeleteTask(ctx, userID, taskID, expected, confirmed)
}

func (u Tasks) Update(ctx context.Context, userID string, update domain.TaskUpdate, expected int64) (domain.Task, error) {
	if userID == "" || update.Task.UserID != userID || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	draft := domain.TaskDraft{GenreID: update.Task.GenreID, TaskTypeID: update.Task.TaskTypeID,
		ParentTaskID: update.Task.ParentTaskID, Title: update.Task.Title, Body: update.Task.Body,
		StartAt: update.Task.StartAt, EndAt: update.Task.EndAt, IsAllDay: update.Task.IsAllDay,
		ManualPriority: update.Task.ManualPriority}
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return domain.Task{}, err
	}
	update.Task.Title = draft.Title
	return u.Repo.Update(ctx, update, expected)
}

type TaskReadRepository interface {
	Search(context.Context, string, domain.TaskSearch) ([]domain.Task, error)
	Tree(context.Context, string, string) ([]domain.Task, error)
}

type Tasks struct{ Repo Repository }

type StatusTransitionRepository interface {
	PreviewStatusTransition(context.Context, string, string, int64, domain.Status) (domain.StatusTransitionPlan, error)
	ApplyStatusTransition(context.Context, string, string, int64, domain.Status, string, domain.StatusTransitionPlan) (domain.Task, error)
}

func (u Tasks) PreviewTransition(ctx context.Context, userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	if userID == "" || taskID == "" || expected < 1 {
		return domain.StatusTransitionPlan{}, domain.ErrForbidden
	}
	repository, ok := u.Repo.(StatusTransitionRepository)
	if !ok {
		return domain.StatusTransitionPlan{}, errors.New("status transition repository unavailable")
	}
	return repository.PreviewStatusTransition(ctx, userID, taskID, expected, to)
}

// TransitionWithImpact is the shared HTTP/MCP rollback protocol. An impact
// token approves exactly one deterministic plan, not merely the target task.
func (u Tasks) TransitionWithImpact(ctx context.Context, userID, taskID string, expected int64, to domain.Status, via, impactToken string, tokens ImpactTokens) (domain.Task, domain.StatusTransitionPlan, error) {
	plan, err := u.PreviewTransition(ctx, userID, taskID, expected, to)
	if err != nil {
		if impactToken != "" && (errors.Is(err, domain.ErrVersionConflict) || errors.Is(err, domain.ErrInvalidTransition)) {
			return domain.Task{}, domain.StatusTransitionPlan{}, ErrInvalidImpactToken
		}
		return domain.Task{}, domain.StatusTransitionPlan{}, err
	}
	if plan.RequiresConfirmation {
		if impactToken == "" {
			return domain.Task{}, plan, domain.ErrImpactConfirmationRequired
		}
		claims := ImpactClaims{UserID: userID, Operation: StatusTransitionImpactOperation(plan), ResourceID: taskID, Version: expected}
		if err := tokens.Verify(impactToken, claims); err != nil {
			return domain.Task{}, plan, err
		}
	}
	repository := u.Repo.(StatusTransitionRepository)
	result, err := repository.ApplyStatusTransition(ctx, userID, taskID, expected, to, via, plan)
	if errors.Is(err, domain.ErrStatusImpactPlanChanged) {
		return domain.Task{}, plan, ErrInvalidImpactToken
	}
	return result, plan, err
}

func StatusTransitionImpactOperation(plan domain.StatusTransitionPlan) string {
	hash := sha256.Sum256([]byte(statusTransitionPlanCanonical(plan)))
	return fmt.Sprintf("task-status:%s:%s", plan.ToStatus, hex.EncodeToString(hash[:]))
}

func statusTransitionPlanCanonical(plan domain.StatusTransitionPlan) string {
	var value strings.Builder
	fmt.Fprintf(&value, "%s|%s|%d|", plan.TaskID, plan.ToStatus, plan.ExpectedVersion)
	for _, change := range plan.Changes {
		fmt.Fprintf(&value, "%s:%s:%s:%d:", change.TaskID, change.FromStatus, change.ToStatus, change.ExpectedVersion)
		if change.FromProgress == nil {
			value.WriteString("null")
		} else {
			fmt.Fprintf(&value, "%d", *change.FromProgress)
		}
		value.WriteByte(':')
		if change.ToProgress == nil {
			value.WriteString("null")
		} else {
			fmt.Fprintf(&value, "%d", *change.ToProgress)
		}
		value.WriteByte('|')
	}
	return value.String()
}

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
	if _, ok := u.Repo.(StatusTransitionRepository); ok {
		result, _, err := u.TransitionWithImpact(ctx, userID, taskID, expected, to, via, "", ImpactTokens{})
		return result, err
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
