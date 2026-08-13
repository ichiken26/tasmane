package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"time"
)

type GoogleJobAction = domain.GoogleJobAction

const (
	GoogleSyncTask     = domain.GoogleSyncTask
	GoogleDeleteTask   = domain.GoogleDeleteTask
	GoogleRecreateTask = domain.GoogleRecreateTask
	GooglePullUser     = domain.GooglePullUser
)

type GoogleJob = domain.GoogleJob
type GoogleQueue interface {
	Enqueue(context.Context, GoogleJob) error
}
type GoogleDurableQueue interface {
	GoogleQueue
	Claim(context.Context, string, time.Time, time.Duration) (GoogleJob, bool, error)
	Ack(context.Context, string, string) error
}
type GoogleJobs struct{ Queue GoogleQueue }
type GoogleSyncGate interface {
	AcquireGoogleSyncLock(context.Context, string, time.Time, time.Duration) (bool, error)
}
type GoogleQueryRepository interface {
	ListGoogleConflicts(context.Context, string) ([]domain.GoogleConflictRecord, error)
	ListGoogleImportCandidates(context.Context, string) ([]domain.GoogleImportCandidate, error)
}
type GoogleQueries struct{ Repo GoogleQueryRepository }

func (q GoogleQueries) Conflicts(ctx context.Context, userID string) ([]domain.GoogleConflictRecord, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	return q.Repo.ListGoogleConflicts(ctx, userID)
}
func (q GoogleQueries) ImportCandidates(ctx context.Context, userID string) ([]domain.GoogleImportCandidate, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	return q.Repo.ListGoogleImportCandidates(ctx, userID)
}

func (j GoogleJobs) Enqueue(ctx context.Context, userID, taskID string, action GoogleJobAction) error {
	if userID == "" || action == "" || j.Queue == nil || (action == GooglePullUser) != (taskID == "") {
		return domain.ErrForbidden
	}
	return j.Queue.Enqueue(ctx, GoogleJob{UserID: userID, TaskID: taskID, Action: action})
}

func (j GoogleJobs) Trigger(ctx context.Context, userID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	gate, ok := j.Queue.(GoogleSyncGate)
	if !ok {
		return errors.New("google sync gate unavailable")
	}
	acquired, err := gate.AcquireGoogleSyncLock(ctx, userID, time.Now().UTC(), 2*time.Minute)
	if err != nil {
		return err
	}
	if !acquired {
		return domain.ErrConflict
	}
	if err := j.Enqueue(ctx, userID, "", GooglePullUser); err != nil {
		if store, ok := j.Queue.(GoogleSyncStore); ok {
			_ = store.MarkFailed(ctx, GoogleJob{UserID: userID, Action: GooglePullUser}, err.Error())
		}
		return err
	}
	return nil
}

type GoogleAPIError struct {
	Status         int
	Err            error
	DomainConflict bool
}

func (e GoogleAPIError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "google api error"
}

type GoogleClient interface {
	Execute(context.Context, GoogleJob) error
	Refresh(context.Context, string) error
}
type GoogleSyncStore interface {
	MarkSyncing(context.Context, GoogleJob) error
	MarkSynced(context.Context, GoogleJob) error
	MarkFailed(context.Context, GoogleJob, string) error
	MarkConflict(context.Context, GoogleJob, string) error
}
type GoogleConsumer struct {
	Client      GoogleClient
	Store       GoogleSyncStore
	Queue       GoogleQueue
	MaxAttempts int
	Backoff     func(int) time.Duration
}

func (c GoogleConsumer) Consume(ctx context.Context, job GoogleJob) error {
	if err := c.Store.MarkSyncing(ctx, job); err != nil {
		return err
	}
	err := c.Client.Execute(ctx, job)
	if err == nil {
		return c.Store.MarkSynced(ctx, job)
	}
	var apiErr GoogleAPIError
	if !errors.As(err, &apiErr) {
		return c.Store.MarkFailed(ctx, job, err.Error())
	}
	action := domain.ClassifyGoogleFailure(apiErr.Status, apiErr.DomainConflict)
	if action == domain.GoogleRefreshAndRetry {
		if refreshErr := c.Client.Refresh(ctx, job.UserID); refreshErr != nil {
			return c.Store.MarkFailed(ctx, job, refreshErr.Error())
		}
		action = domain.GoogleRetry
	}
	max := c.MaxAttempts
	if max == 0 {
		max = 5
	}
	if action == domain.GoogleRetry && job.Attempt+1 < max {
		job.Attempt++
		return c.Queue.Enqueue(ctx, job)
	}
	if action == domain.GoogleConflict {
		return c.Store.MarkConflict(ctx, job, err.Error())
	}
	return c.Store.MarkFailed(ctx, job, err.Error())
}
