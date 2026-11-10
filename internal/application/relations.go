package application

import (
	"context"
	"tasmane/internal/domain"
	"time"
)

// 階層と依存関係書き込みの単一永続化境界
type RelationRepository interface {
	LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error)
	ListTaskDependencies(context.Context, string, string) ([]domain.Dependency, error)
	SetParent(context.Context, string, string, string) error
	AddDependency(context.Context, string, domain.Dependency, int64) error
	RemoveDependency(context.Context, string, domain.Dependency) error
}

// 階層・依存関係の検証付きユースケース
type Relations struct{ Repo RelationRepository }

// 所有権検証付きグラフスナップショット取得
func (u Relations) Snapshot(ctx context.Context, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, nil, domain.ErrForbidden
	}
	// グラフの読み出し
	tasks, dependencies, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	// 全タスクの所有権検証
	if err := validateGraphOwner(tasks, userID); err != nil {
		return nil, nil, err
	}
	return tasks, dependencies, nil
}

// 循環検出後の親タスク割当
func (u Relations) AssignParent(ctx context.Context, userID, taskID, parentID string) error {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.ErrForbidden
	}
	// グラフの読み出し
	tasks, _, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	// 全タスクの所有権検証
	if err := validateGraphOwner(tasks, userID); err != nil {
		return err
	}
	// 階層割当のドメイン検証
	if err := domain.ValidateHierarchyAssignment(tasks, taskID, parentID); err != nil {
		return err
	}
	// 親IDの永続化
	return u.Repo.SetParent(ctx, userID, taskID, parentID)
}

// 循環検出後の依存関係追加
func (u Relations) AddDependency(ctx context.Context, userID, taskID, dependsOnTaskID string, expectedVersion int64) error {
	// 未認証または不正バージョンの拒否
	if userID == "" || expectedVersion < 1 {
		return domain.ErrForbidden
	}
	// グラフの読み出し
	tasks, dependencies, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	// 全タスクの所有権検証
	if err := validateGraphOwner(tasks, userID); err != nil {
		return err
	}
	// 依存追加のドメイン検証
	if err := domain.ValidateDependencyAddition(tasks, dependencies, taskID, dependsOnTaskID); err != nil {
		return err
	}
	// 依存エッジの永続化
	return u.Repo.AddDependency(ctx, userID, domain.Dependency{TaskID: taskID, DependsOnTaskID: dependsOnTaskID}, expectedVersion)
}

// タスク依存関係の一覧取得
func (u Relations) ListDependencies(ctx context.Context, userID, taskID string) ([]domain.Dependency, error) {
	// 未認証または空タスクIDの拒否
	if userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	// 依存関係の読み出し
	return u.Repo.ListTaskDependencies(ctx, userID, taskID)
}

// 既存エッジ限定の依存関係削除
func (u Relations) RemoveDependency(ctx context.Context, userID, taskID, dependsOnTaskID string) error {
	// 未認証または空IDの拒否
	if userID == "" || taskID == "" || dependsOnTaskID == "" {
		return domain.ErrForbidden
	}
	// グラフの読み出し
	tasks, dependencies, err := u.Repo.LoadGraph(ctx, userID)
	if err != nil {
		return err
	}
	// 全タスクの所有権検証
	if err := validateGraphOwner(tasks, userID); err != nil {
		return err
	}
	// 対象エッジの存在確認
	found := false
	for _, edge := range dependencies {
		if edge.TaskID == taskID && edge.DependsOnTaskID == dependsOnTaskID {
			found = true
			break
		}
	}
	// 未検出エッジの拒否
	if !found {
		return domain.ErrForbidden
	}
	// 依存エッジの削除
	return u.Repo.RemoveDependency(ctx, userID, domain.Dependency{TaskID: taskID, DependsOnTaskID: dependsOnTaskID})
}

// グラフタスクの所有者一致検証
func validateGraphOwner(tasks []domain.GraphTask, userID string) error {
	for _, task := range tasks {
		if task.UserID != userID {
			return domain.ErrGraphOwnership
		}
	}
	return nil
}

// 確認事項の永続化境界
type ConfirmationRepository interface {
	CreateConfirmation(context.Context, string, string, string, string, *time.Time) (domain.Confirmation, error)
	GetConfirmation(context.Context, string, string) (domain.Confirmation, error)
	ListConfirmations(context.Context, string, string) ([]domain.Confirmation, error)
	UpdateConfirmation(context.Context, string, string, domain.Confirmation) error
}

// 確認事項の作成・遷移ユースケース
type Confirmations struct{ Repo ConfirmationRepository }

// 確認事項の新規作成
func (u Confirmations) Create(ctx context.Context, userID, taskID, content, targetName string, dueAt *time.Time) (domain.Confirmation, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	// 確認事項の永続化
	return u.Repo.CreateConfirmation(ctx, userID, taskID, content, targetName, dueAt)
}

// タスク配下の確認事項一覧取得
func (u Confirmations) List(ctx context.Context, userID, taskID string) ([]domain.Confirmation, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// 確認事項一覧の読み出し
	return u.Repo.ListConfirmations(ctx, userID, taskID)
}

// 確認事項の状態遷移
func (u Confirmations) Transition(ctx context.Context, userID, confirmationID string, to domain.ConfirmationStatus, at time.Time) (domain.Confirmation, error) {
	// 未認証・空ID・ゼロ時刻の拒否
	if userID == "" || confirmationID == "" || at.IsZero() {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	// 現行確認事項の取得
	current, err := u.Repo.GetConfirmation(ctx, userID, confirmationID)
	if err != nil {
		return domain.Confirmation{}, err
	}
	// ドメイン遷移の適用
	next, err := current.Transition(to, at)
	if err != nil {
		return domain.Confirmation{}, err
	}
	// 遷移結果の永続化
	if err := u.Repo.UpdateConfirmation(ctx, userID, confirmationID, next); err != nil {
		return domain.Confirmation{}, err
	}
	return next, nil
}
