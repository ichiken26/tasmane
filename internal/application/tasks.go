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

// タスク書き込みの永続化境界
type Repository interface {
	Create(context.Context, string, domain.TaskDraft, domain.TaskCreateAuthorization) (domain.Task, error)
	Get(context.Context, string, string) (domain.Task, error)
	UpdateStatus(context.Context, domain.Task, int64, string) (domain.Task, error)
	Update(context.Context, domain.TaskUpdate, int64) (domain.Task, error)
	DeleteTask(context.Context, string, string, int64, bool) (domain.TaskDeletionPlan, error)
	GoogleUncomplete(context.Context, string, string, int64) (domain.Task, error)
}

// Google起点の未完了化
func (u Tasks) GoogleUncomplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	// 未認証・空ID・不正バージョンの拒否
	if userID == "" || taskID == "" || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	// Google未完了の永続化
	return u.Repo.GoogleUncomplete(ctx, userID, taskID, expected)
}

// 楽観ロック付きタスク削除
func (u Tasks) Delete(ctx context.Context, userID, taskID string, expected int64, confirmed bool) (domain.TaskDeletionPlan, error) {
	// 未認証・空ID・不正バージョンの拒否
	if userID == "" || taskID == "" || expected < 1 {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	}
	// 削除計画の永続化
	return u.Repo.DeleteTask(ctx, userID, taskID, expected, confirmed)
}

// 下書き検証付きタスク更新
func (u Tasks) Update(ctx context.Context, userID string, update domain.TaskUpdate, expected int64) (domain.Task, error) {
	// 未認証・所有者不一致・不正バージョンの拒否
	if userID == "" || update.Task.UserID != userID || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	// 更新内容の下書き正規化
	draft := domain.TaskDraft{GenreID: update.Task.GenreID, TaskTypeID: update.Task.TaskTypeID,
		ParentTaskID: update.Task.ParentTaskID, Title: update.Task.Title, Body: update.Task.Body,
		StartAt: update.Task.StartAt, EndAt: update.Task.EndAt, IsAllDay: update.Task.IsAllDay,
		ManualPriority: update.Task.ManualPriority}
	// 下書きの妥当性検証
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return domain.Task{}, err
	}
	// 正規化タイトルの反映
	update.Task.Title = draft.Title
	// 楽観ロック付き更新
	return u.Repo.Update(ctx, update, expected)
}

// タスク検索・ツリー読み出しの永続化境界
type TaskReadRepository interface {
	Search(context.Context, string, domain.TaskSearch) ([]domain.Task, error)
	Tree(context.Context, string, string) ([]domain.Task, error)
}

// 読み出しリポジトリ未提供（テスト用スタブ等）
var ErrTaskReadUnavailable = errors.New("task read repository unavailable")

// タスク操作のユースケース
type Tasks struct{ Repo Repository }

// 状態遷移計画の永続化境界
type StatusTransitionRepository interface {
	PreviewStatusTransition(context.Context, string, string, int64, domain.Status) (domain.StatusTransitionPlan, error)
	ApplyStatusTransition(context.Context, string, string, int64, domain.Status, string, domain.StatusTransitionPlan) (domain.Task, error)
}

// 状態遷移計画の事前算出
func (u Tasks) PreviewTransition(ctx context.Context, userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	// 未認証・空ID・不正バージョンの拒否
	if userID == "" || taskID == "" || expected < 1 {
		return domain.StatusTransitionPlan{}, domain.ErrForbidden
	}
	// 遷移計画リポジトリの解決
	repository, ok := u.Repo.(StatusTransitionRepository)
	if !ok {
		return domain.StatusTransitionPlan{}, errors.New("status transition repository unavailable")
	}
	// 遷移計画の読み出し
	return repository.PreviewStatusTransition(ctx, userID, taskID, expected, to)
}

// HTTP/MCP共通の影響トークン付き状態遷移
func (u Tasks) TransitionWithImpact(ctx context.Context, userID, taskID string, expected int64, to domain.Status, via, impactToken string, tokens ImpactTokens) (domain.Task, domain.StatusTransitionPlan, error) {
	// 遷移計画の事前算出
	plan, err := u.PreviewTransition(ctx, userID, taskID, expected, to)
	if err != nil {
		// 計画失敗時のトークン無効化
		if impactToken != "" && (errors.Is(err, domain.ErrVersionConflict) || errors.Is(err, domain.ErrInvalidTransition)) {
			return domain.Task{}, domain.StatusTransitionPlan{}, ErrInvalidImpactToken
		}
		return domain.Task{}, domain.StatusTransitionPlan{}, err
	}
	// 確認必須時のトークン検証
	if plan.RequiresConfirmation {
		if impactToken == "" {
			return domain.Task{}, plan, domain.ErrImpactConfirmationRequired
		}
		claims := ImpactClaims{UserID: userID, Operation: StatusTransitionImpactOperation(plan), ResourceID: taskID, Version: expected}
		if err := tokens.Verify(impactToken, claims); err != nil {
			return domain.Task{}, plan, err
		}
	}
	// 計画の適用
	repository := u.Repo.(StatusTransitionRepository)
	result, err := repository.ApplyStatusTransition(ctx, userID, taskID, expected, to, via, plan)
	// 計画変更時のトークン無効化
	if errors.Is(err, domain.ErrStatusImpactPlanChanged) {
		return domain.Task{}, plan, ErrInvalidImpactToken
	}
	return result, plan, err
}

// 状態遷移計画の影響操作識別子算出
func StatusTransitionImpactOperation(plan domain.StatusTransitionPlan) string {
	hash := sha256.Sum256([]byte(statusTransitionPlanCanonical(plan)))
	return fmt.Sprintf("task-status:%s:%s", plan.ToStatus, hex.EncodeToString(hash[:]))
}

// 状態遷移計画の正規化文字列組み立て
func statusTransitionPlanCanonical(plan domain.StatusTransitionPlan) string {
	var value strings.Builder
	// 計画識別子の連結
	fmt.Fprintf(&value, "%s|%s|%d|", plan.TaskID, plan.ToStatus, plan.ExpectedVersion)
	// 変更集合の正規化追記
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

// 条件付きタスク検索
func (u Tasks) Search(ctx context.Context, userID string, search domain.TaskSearch) ([]domain.Task, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// 読み出しリポジトリの解決
	repository, ok := u.Repo.(TaskReadRepository)
	if !ok {
		return nil, ErrTaskReadUnavailable
	}
	// 検索結果の読み出し
	return repository.Search(ctx, userID, search)
}

// タスクツリーの取得
func (u Tasks) Tree(ctx context.Context, userID, taskID string) ([]domain.Task, error) {
	// 未認証または空タスクIDの拒否
	if userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	// 読み出しリポジトリの解決
	repository, ok := u.Repo.(TaskReadRepository)
	if !ok {
		return nil, ErrTaskReadUnavailable
	}
	// ツリーの読み出し
	return repository.Tree(ctx, userID, taskID)
}

// 下書き検証付きタスク作成
func (u Tasks) Create(ctx context.Context, userID string, draft domain.TaskDraft, authorization domain.TaskCreateAuthorization) (domain.Task, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	// 下書きの妥当性検証
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return domain.Task{}, err
	}
	// 認可付きタスク作成
	return u.Repo.Create(ctx, userID, draft, authorization)
}

// 所有権付きタスク単件取得
func (u Tasks) Get(ctx context.Context, userID, taskID string) (domain.Task, error) {
	// 未認証または空タスクIDの拒否
	if userID == "" || taskID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	// タスクの読み出し
	return u.Repo.Get(ctx, userID, taskID)
}

// 所有権付きタスク状態遷移
func (u Tasks) Transition(ctx context.Context, userID, taskID string, expected int64, to domain.Status, via string) (domain.Task, error) {
	// 未認証または不正バージョンの拒否
	if userID == "" || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	// 影響計画リポジトリ経由の遷移
	if _, ok := u.Repo.(StatusTransitionRepository); ok {
		result, _, err := u.TransitionWithImpact(ctx, userID, taskID, expected, to, via, "", ImpactTokens{})
		return result, err
	}
	// 現行タスクの取得
	t, err := u.Repo.Get(ctx, userID, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	// 所有者不一致の拒否
	if t.UserID != userID {
		return domain.Task{}, domain.ErrForbidden
	}
	// ドメイン遷移の適用
	next, err := t.Transition(to)
	if err != nil {
		return domain.Task{}, err
	}
	// 楽観ロック付き状態永続化
	return u.Repo.UpdateStatus(ctx, next.ForPersistence(), expected, via)
}

// Google起点の完了遷移
func (u Tasks) GoogleComplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	// 現行タスクの取得
	t, err := u.Repo.Get(ctx, userID, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	// 未着手から進行中への中間遷移
	if t.Status == domain.NotStarted {
		t, err = u.Transition(ctx, userID, taskID, expected, domain.InProgress, "GOOGLE_TASKS")
		if err != nil {
			return domain.Task{}, err
		}
		expected = t.Version
	}
	// 進行中以外の完了拒否
	if t.Status != domain.InProgress {
		return domain.Task{}, errors.New("google completion requires active task")
	}
	// Google起点の完了遷移
	return u.Transition(ctx, userID, taskID, expected, domain.Completed, "GOOGLE_TASKS")
}
