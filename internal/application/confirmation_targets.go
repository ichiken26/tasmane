package application

import (
	"context"

	"tasmane/internal/domain"
)

// 確認先プリセットの永続化境界
type ConfirmationTargetRepository interface {
	ListConfirmationTargets(context.Context, string) ([]domain.ConfirmationTarget, error)
	CreateConfirmationTarget(context.Context, string, string) (domain.ConfirmationTarget, error)
	DeleteConfirmationTarget(context.Context, string, string) error
}

// 確認先プリセットのユースケース
type ConfirmationTargets struct{ Repo ConfirmationTargetRepository }

// 一覧
func (u ConfirmationTargets) List(ctx context.Context, userID string) ([]domain.ConfirmationTarget, error) {
	if userID == "" || u.Repo == nil {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListConfirmationTargets(ctx, userID)
}

// 登録
func (u ConfirmationTargets) Create(ctx context.Context, userID, name string) (domain.ConfirmationTarget, error) {
	if userID == "" || u.Repo == nil {
		return domain.ConfirmationTarget{}, domain.ErrForbidden
	}
	return u.Repo.CreateConfirmationTarget(ctx, userID, name)
}

// 削除（人間UI/APIのみ）
func (u ConfirmationTargets) Delete(ctx context.Context, userID, targetID string) error {
	if userID == "" || u.Repo == nil {
		return domain.ErrForbidden
	}
	return u.Repo.DeleteConfirmationTarget(ctx, userID, targetID)
}
