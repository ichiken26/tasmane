package application

import (
	"context"
	"errors"
	"fmt"
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

// Google同期ジョブの投入境界
type GoogleQueue interface {
	Enqueue(context.Context, GoogleJob) error
}

// 耐久キューの取得・完了境界
type GoogleDurableQueue interface {
	GoogleQueue
	Claim(context.Context, string, time.Time, time.Duration) (GoogleJob, bool, error)
	Ack(context.Context, string, string) error
}

// Googleジョブ投入のユースケース
type GoogleJobs struct{ Queue GoogleQueue }

// ユーザー単位同期ロックの獲得境界
type GoogleSyncGate interface {
	AcquireGoogleSyncLock(context.Context, string, time.Time, time.Duration) (bool, error)
}

// Google競合・取込候補の読み出し境界
type GoogleQueryRepository interface {
	ListGoogleConflicts(context.Context, string) ([]domain.GoogleConflictRecord, error)
	ListGoogleImportCandidates(context.Context, string) ([]domain.GoogleImportCandidate, error)
}

// Google照会のユースケース
type GoogleQueries struct{ Repo GoogleQueryRepository }

// Google競合一覧の取得
func (q GoogleQueries) Conflicts(ctx context.Context, userID string) ([]domain.GoogleConflictRecord, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// 競合記録の読み出し
	return q.Repo.ListGoogleConflicts(ctx, userID)
}

// Google取込候補一覧の取得
func (q GoogleQueries) ImportCandidates(ctx context.Context, userID string) ([]domain.GoogleImportCandidate, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// 取込候補の読み出し
	return q.Repo.ListGoogleImportCandidates(ctx, userID)
}

// 検証付きGoogleジョブの投入
func (j GoogleJobs) Enqueue(ctx context.Context, userID, taskID string, action GoogleJobAction) error {
	// 未認証・空操作・キュー未設定・ID不整合の拒否
	if userID == "" || action == "" || j.Queue == nil || (action == GooglePullUser) != (taskID == "") {
		return domain.ErrForbidden
	}
	// ジョブのキュー投入
	return j.Queue.Enqueue(ctx, GoogleJob{UserID: userID, TaskID: taskID, Action: action})
}

// 同期ロック付きユーザー単位プルの起動
func (j GoogleJobs) Trigger(ctx context.Context, userID string) error {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.ErrForbidden
	}
	// 同期ゲートの解決
	gate, ok := j.Queue.(GoogleSyncGate)
	if !ok {
		return errors.New("google sync gate unavailable")
	}
	// 同期ロックの獲得
	acquired, err := gate.AcquireGoogleSyncLock(ctx, userID, time.Now().UTC(), 2*time.Minute)
	if err != nil {
		return err
	}
	// ロック失敗時の競合返却
	if !acquired {
		return domain.ErrConflict
	}
	// 手動再同期では最終FAILEDタスクも再送
	if requeue, ok := j.Queue.(GoogleFailedRequeue); ok {
		if err := requeue.RequeueFailedGoogleTasks(ctx, userID); err != nil {
			return err
		}
	}
	// ユーザー単位プルの投入
	if err := j.Enqueue(ctx, userID, "", GooglePullUser); err != nil {
		// 投入失敗時の失敗記録
		if store, ok := j.Queue.(GoogleSyncStore); ok {
			_ = store.MarkFailed(ctx, GoogleJob{UserID: userID, Action: GooglePullUser}, err.Error())
		}
		return err
	}
	return nil
}

// Google API失敗の状態と競合情報
type GoogleAPIError struct {
	Status         int
	Err            error
	DomainConflict bool
}

// APIエラーメッセージの返却
func (e GoogleAPIError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "google api error"
}

// Google API実行とトークン更新の境界
type GoogleClient interface {
	Execute(context.Context, GoogleJob) error
	Refresh(context.Context, string) error
}

// Google同期状態の永続化境界
type GoogleSyncStore interface {
	MarkSyncing(context.Context, GoogleJob) error
	MarkSynced(context.Context, GoogleJob) error
	MarkFailed(context.Context, GoogleJob, string) error
	MarkConflict(context.Context, GoogleJob, string) error
}

// Googleジョブ消費のユースケース
type GoogleConsumer struct {
	Client      GoogleClient
	Store       GoogleSyncStore
	Queue       GoogleQueue
	MaxAttempts int
	Backoff     func(int) time.Duration
}

// 失敗分類付きGoogleジョブの消費
func (c GoogleConsumer) Consume(ctx context.Context, job GoogleJob) error {
	// 同期中状態への更新
	if err := c.Store.MarkSyncing(ctx, job); err != nil {
		return err
	}
	// Google APIの実行
	err := c.Client.Execute(ctx, job)
	if err == nil {
		return c.Store.MarkSynced(ctx, job)
	}
	// 非APIエラーの失敗記録
	var apiErr GoogleAPIError
	if !errors.As(err, &apiErr) {
		return c.recordFailure(ctx, job, err)
	}
	// 失敗分類の決定
	action := domain.ClassifyGoogleFailure(apiErr.Status, apiErr.DomainConflict)
	// トークン更新後の再試行転換
	if action == domain.GoogleRefreshAndRetry {
		if refreshErr := c.Client.Refresh(ctx, job.UserID); refreshErr != nil {
			return c.recordFailure(ctx, job, refreshErr)
		}
		action = domain.GoogleRetry
	}
	// 最大試行回数の決定
	max := c.MaxAttempts
	if max == 0 {
		max = 5
	}
	// 再試行ジョブの再投入
	if action == domain.GoogleRetry && job.Attempt+1 < max {
		job.Attempt++
		return c.Queue.Enqueue(ctx, job)
	}
	// 競合状態への更新
	if action == domain.GoogleConflict {
		return keepCause(err, c.Store.MarkConflict(ctx, job, err.Error()))
	}
	// 永続失敗の記録
	return c.recordFailure(ctx, job, err)
}

// 失敗の記録（記録できなくても元の原因をワーカーのログへ残す）
func (c GoogleConsumer) recordFailure(ctx context.Context, job GoogleJob, cause error) error {
	return keepCause(cause, c.Store.MarkFailed(ctx, job, cause.Error()))
}

// 状態記録エラーへの元原因の付与
func keepCause(cause, recordErr error) error {
	if recordErr == nil {
		return nil
	}
	return fmt.Errorf("%v (record sync state: %w)", cause, recordErr)
}
