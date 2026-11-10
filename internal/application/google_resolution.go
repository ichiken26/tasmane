package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"tasmane/internal/domain"
)

// コンフリクト解決の永続化境界
type GoogleResolutionRepository interface {
	ResolveGoogleConflict(context.Context, string, string, domain.GoogleResolution) error
}

// 取込候補の永続化境界
type GoogleImportRepository interface {
	GetGoogleImportCandidate(context.Context, string, string, string) (domain.GoogleImportCandidate, error)
	LinkGoogleImportCandidate(context.Context, string, string, string, string) error
}

// 間隔超過判定付き同期ロック（アクセス時の自動同期）
type GoogleDueSyncGate interface {
	AcquireDueGoogleSync(context.Context, string, time.Time, time.Duration) (bool, error)
}

// コンフリクト解決（Google削除の「タスマネ削除」は通常の削除UseCaseで行う）
func (q GoogleQueries) Resolve(ctx context.Context, userID, taskID string, resolution domain.GoogleResolution) error {
	if userID == "" || taskID == "" {
		return domain.ErrForbidden
	}
	switch resolution {
	case domain.GoogleKeepTasmane, domain.GoogleRecreate, domain.GoogleUnlink:
	default:
		return domain.ErrInvalidTaskInput
	}
	repository, ok := q.Repo.(GoogleResolutionRepository)
	if !ok {
		return errors.New("google resolution repository unavailable")
	}
	return repository.ResolveGoogleConflict(ctx, userID, taskID, resolution)
}

// Google-only Taskの取込（ユーザーがGenreを選択して確定）
type GoogleImports struct {
	Tasks    Tasks
	Settings Settings
	Repo     GoogleImportRepository
}

// 取込候補からのタスク作成と紐付け
func (i GoogleImports) Import(ctx context.Context, userID, listID, googleTaskID, genreID string) (domain.Task, error) {
	if userID == "" || listID == "" || googleTaskID == "" || genreID == "" || i.Repo == nil {
		return domain.Task{}, domain.ErrForbidden
	}
	candidate, err := i.Repo.GetGoogleImportCandidate(ctx, userID, listID, googleTaskID)
	if err != nil {
		return domain.Task{}, err
	}
	snapshot := map[string]string{}
	_ = json.Unmarshal(candidate.Snapshot, &snapshot)
	title := strings.TrimSpace(snapshot["title"])
	if title == "" {
		title = "(無題のGoogleタスク)"
	}
	draft := domain.TaskDraft{GenreID: genreID, Title: title, IsAllDay: true}
	if notes := snapshot["notes"]; notes != "" {
		draft.Body = &notes
	}
	// Googleのdueは日付のみ: ユーザーTimezoneの当日終わりを期限とする
	if due := snapshot["due"]; due != "" {
		location := time.UTC
		if i.Settings.Repo != nil {
			if settings, settingsErr := i.Settings.Get(ctx, userID); settingsErr == nil {
				if loaded, loadErr := time.LoadLocation(settings.Timezone); loadErr == nil {
					location = loaded
				}
			}
		}
		day, parseErr := time.ParseInLocation("2006-01-02", due, location)
		if parseErr != nil {
			return domain.Task{}, domain.ErrInvalidTaskInput
		}
		endAt := day.Add(24*time.Hour - time.Millisecond).UTC()
		draft.EndAt = &endAt
	}
	// Google起点の作成（Googleへ再送しない）
	inbound := domain.WithGoogleInbound(ctx)
	task, err := i.Tasks.Create(inbound, userID, draft, domain.TaskCreateAuthorization{})
	if err != nil {
		return domain.Task{}, err
	}
	if err := i.Repo.LinkGoogleImportCandidate(ctx, userID, listID, googleTaskID, task.ID); err != nil {
		// 同時取込で候補が消えた場合は作成分を取り消す
		_, _ = i.Tasks.Delete(ctx, userID, task.ID, task.Version, true)
		return domain.Task{}, err
	}
	// Google側で完了済みなら通常の完了UseCase（NOT_STARTED→IN_PROGRESS→COMPLETED、via GOOGLE_TASKS）
	if snapshot["status"] == "completed" {
		completed, completeErr := i.Tasks.GoogleComplete(inbound, userID, task.ID, task.Version)
		if completeErr == nil {
			task = completed
		}
	}
	return task, nil
}

// 最終同期から設定間隔以上経過していれば非同期同期を投入（ユーザー単位lock）
func (j GoogleJobs) TriggerIfDue(ctx context.Context, userID string) (bool, error) {
	if userID == "" || j.Queue == nil {
		return false, domain.ErrForbidden
	}
	gate, ok := j.Queue.(GoogleDueSyncGate)
	if !ok {
		return false, nil
	}
	acquired, err := gate.AcquireDueGoogleSync(ctx, userID, time.Now().UTC(), 2*time.Minute)
	if err != nil || !acquired {
		return false, err
	}
	return true, j.Enqueue(ctx, userID, "", GooglePullUser)
}

// 同期状態の読み出し境界
type GoogleStatusRepository interface {
	GoogleSyncStatus(context.Context, string) (domain.GoogleSyncStatus, error)
}

// 最終FAILEDタスクの再投入境界
type GoogleFailedRequeue interface {
	RequeueFailedGoogleTasks(context.Context, string) error
}

// Google連携の同期状態
func (q GoogleQueries) Status(ctx context.Context, userID string) (domain.GoogleSyncStatus, error) {
	if userID == "" {
		return domain.GoogleSyncStatus{}, domain.ErrForbidden
	}
	repository, ok := q.Repo.(GoogleStatusRepository)
	if !ok {
		return domain.GoogleSyncStatus{Failed: []domain.GoogleFailedTask{}}, nil
	}
	return repository.GoogleSyncStatus(ctx, userID)
}

// 専用リスト作り直しの境界
type GoogleListResetRepository interface {
	ResetGoogleTaskList(context.Context, string) error
}

// 専用 Task List の作り直し（全タスクを新リストへ再作成）
func (q GoogleQueries) ResetTaskList(ctx context.Context, userID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	repository, ok := q.Repo.(GoogleListResetRepository)
	if !ok {
		return errors.New("google list reset unavailable")
	}
	return repository.ResetGoogleTaskList(ctx, userID)
}
