package domain

import (
	"errors"
	"strings"
	"time"
)

// タスク状態の列挙
type Status string

const (
	NotStarted Status = "NOT_STARTED"
	InProgress Status = "IN_PROGRESS"
	Completed  Status = "COMPLETED"
)

// タスクドメインのエラー群
var (
	ErrInvalidTransition          = errors.New("invalid task transition")
	ErrNotCompletable             = errors.New("task is not completable")
	ErrVersionConflict            = errors.New("version conflict")
	ErrForbidden                  = errors.New("forbidden")
	ErrParentNeedsChildren        = errors.New("parent needs children")
	ErrInvalidProgress            = errors.New("invalid progress")
	ErrInvalidPriority            = errors.New("invalid priority")
	ErrInvalidTaskInput           = errors.New("invalid task input")
	ErrImpactConfirmationRequired = errors.New("impact confirmation required")
	ErrImpactPlanChanged          = errors.New("impact plan changed")
	ErrCompletedParent            = errors.New("completed task cannot receive children")
)

// タスク作成の下書き入力
type TaskDraft struct {
	GenreID        string
	TaskTypeID     *string
	ParentTaskID   *string
	Title          string
	Body           *string
	StartAt        *time.Time
	EndAt          *time.Time
	IsAllDay       bool
	ManualPriority *Priority
}

// リーフから親への変換認可
type TaskCreateAuthorization struct {
	AllowLeafConversion bool
	ParentVersion       int64
	// 影響確認済みの祖先日付伝播
	DateChanges []DateChange
}

// タスク検索条件
type TaskSearch struct {
	Query    string
	GenreIDs []string
	Priority *Priority
	Deadline string
}

// タスク更新と日付変更の束
type TaskUpdate struct {
	Task        Task
	DateChanges []DateChange
	// 影響トークン検証後のジャンル移動承認
	ApprovedGenreMove *GenreMovePlan
	// 影響トークン検証後の付け替え先葉タスクの親化承認（承認時点の親version）
	ApprovedLeafConversion *int64
}

// 下書きからの新規タスク生成
func NewTask(id, userID string, draft TaskDraft) (Task, error) {
	// 識別子と下書きの入力検証
	if id == "" || userID == "" {
		return Task{}, ErrInvalidTaskInput
	}
	if err := ValidateTaskDraft(&draft); err != nil {
		return Task{}, err
	}
	draft.Title = strings.TrimSpace(draft.Title)
	return Task{
		ID: id, UserID: userID, GenreID: draft.GenreID, TaskTypeID: draft.TaskTypeID,
		ParentTaskID: draft.ParentTaskID, Title: draft.Title, Body: draft.Body,
		Status: NotStarted, StartAt: draft.StartAt, EndAt: draft.EndAt,
		IsAllDay: draft.IsAllDay, ManualPriority: draft.ManualPriority, Version: 1,
	}, nil
}

// タスク下書きの入力検証
func ValidateTaskDraft(draft *TaskDraft) error {
	if draft == nil {
		return ErrInvalidTaskInput
	}
	draft.Title = strings.TrimSpace(draft.Title)
	if draft.GenreID == "" || draft.Title == "" {
		return ErrInvalidTaskInput
	}
	if invalidDateRange(draft.StartAt, draft.EndAt) {
		return ErrInvalidDateRange
	}
	if draft.ManualPriority != nil {
		if _, ok := priorityWeight(*draft.ManualPriority); !ok {
			return ErrInvalidPriority
		}
	}
	return nil
}

// 優先度の列挙
type Priority string

const (
	HighPriority   Priority = "HIGH"
	MediumPriority Priority = "MEDIUM"
	LowPriority    Priority = "LOW"
)

// 親進捗算出用の子貢献値
type ProgressContribution struct {
	Progress          *int
	EffectivePriority Priority
}

// タスクのドメイン実体
type Task struct {
	ID, UserID              string
	GenreID                 string
	TaskTypeID              *string
	ParentTaskID            *string
	Title                   string
	Body                    *string
	Status                  Status
	Progress                *int
	StartAt                 *time.Time
	EndAt                   *time.Time
	IsAllDay                bool
	ManualPriority          *Priority
	Version                 int64
	CreatedAt               time.Time
	HasChildren             bool
	IncompleteDescendants   int
	UnresolvedConfirmations int
}

// 親の派生進捗を永続化しない境界
func (t Task) ForPersistence() Task {
	// 親の leaf_progress 列クリア
	if t.HasChildren {
		t.Progress = nil
	}
	return t
}

// タスク状態遷移のドメイン規則
func (t Task) Transition(to Status) (Task, error) {
	if t.Status == NotStarted && to == InProgress {
		v := 0
		t.Status, t.Progress = to, &v
		return t, nil
	}
	if t.Status == InProgress && to == Completed {
		// 未完了子孫または未解決確認がある完了拒否
		if t.IncompleteDescendants > 0 || t.UnresolvedConfirmations > 0 {
			return Task{}, ErrNotCompletable
		}
		v := 100
		t.Status, t.Progress = to, &v
		return t, nil
	}
	if t.Status == InProgress && to == NotStarted {
		t.Status, t.Progress = to, nil
		return t, nil
	}
	if t.Status == Completed && to == InProgress {
		v := 95
		t.Status, t.Progress = to, &v
		return t, nil
	}
	return Task{}, ErrInvalidTransition
}

// 直下の子から親の表示進捗を導出
func ParentProgress(status Status, children []ProgressContribution) (int, error) {
	// 子の有無と完了状態の判定
	if len(children) == 0 {
		return 0, ErrParentNeedsChildren
	}
	if status == Completed {
		return 100, nil
	}
	// 優先度加重の進捗集計
	weightedProgress, totalWeight := 0, 0
	for _, child := range children {
		weight, ok := priorityWeight(child.EffectivePriority)
		if !ok {
			return 0, ErrInvalidPriority
		}
		progress := 0
		if child.Progress != nil {
			progress = *child.Progress
			if progress < 0 || progress > 100 {
				return 0, ErrInvalidProgress
			}
		}
		weightedProgress += progress * weight
		totalWeight += weight
	}
	progress := weightedProgress / totalWeight
	// 未完了親の進捗上限適用
	if progress > 95 {
		return 95, nil
	}
	return progress, nil
}

// 優先度の加重値
func priorityWeight(priority Priority) (int, bool) {
	switch priority {
	case HighPriority:
		return 5, true
	case MediumPriority:
		return 3, true
	case LowPriority:
		return 1, true
	default:
		return 0, false
	}
}

// 完了祖先の進行中巻き戻し
func RollbackCompletedAncestors(ancestors []Task) []Task {
	result := append([]Task(nil), ancestors...)
	for index := range result {
		if result[index].Status == Completed {
			result[index].Status = InProgress
			result[index].HasChildren = true
			result[index].Progress = nil
		}
	}
	return result
}

// 進行中子孫の未着手伝播
func RollbackInProgressDescendants(descendants []Task) []Task {
	result := append([]Task(nil), descendants...)
	for index := range result {
		if result[index].Status == InProgress {
			result[index].Status = NotStarted
			result[index].Progress = nil
		}
	}
	return result
}

// 葉タスク進捗の検証（進行中の葉のみ0..95・5刻み）
func ValidateLeafProgress(task Task, progress int) error {
	if task.HasChildren || task.Status != InProgress {
		return ErrInvalidProgress
	}
	if progress < 0 || progress > 95 || progress%5 != 0 {
		return ErrInvalidProgress
	}
	return nil
}

// 子を失い葉へ戻った親の進捗正規化値
func NormalizedLeafProgress(status Status) *int {
	switch status {
	case InProgress:
		value := 0
		return &value
	case Completed:
		value := 100
		return &value
	default:
		return nil
	}
}

// 復元可能な削除済みタスク
type DeletedTask struct {
	Task
	DeletedAt time.Time
}
