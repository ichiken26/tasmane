package application

import (
	"context"

	"tasmane/internal/domain"
)

// ジャンルとタスク種別の永続化境界
type CatalogRepository interface {
	CreateGenre(context.Context, string, string) (domain.Genre, error)
	GetGenre(context.Context, string, string) (domain.Genre, error)
	ListGenres(context.Context, string) ([]domain.Genre, error)
	RenameGenre(context.Context, string, string, string) (domain.Genre, error)
	DeleteGenre(context.Context, string, string) error
	CreateTaskType(context.Context, string, string) (domain.TaskType, error)
	GetTaskType(context.Context, string, string) (domain.TaskType, error)
	ListTaskTypes(context.Context, string) ([]domain.TaskType, error)
	ListGenreTaskTypes(context.Context, string, string) ([]domain.TaskType, error)
	RenameTaskType(context.Context, string, string, string) (domain.TaskType, error)
	DeleteTaskType(context.Context, string, string) error
	AttachGenreTaskType(context.Context, string, string, string) error
	DetachGenreTaskType(context.Context, string, string, string) error
}

// カタログ操作のユースケース
type Catalog struct{ Repo CatalogRepository }

// ジャンルの新規作成
func (u Catalog) CreateGenre(ctx context.Context, userID, name string) (domain.Genre, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	// ジャンルの永続化
	return u.Repo.CreateGenre(ctx, userID, name)
}

// ジャンルの単件取得
func (u Catalog) GetGenre(ctx context.Context, userID, genreID string) (domain.Genre, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	// ジャンルの読み出し
	return u.Repo.GetGenre(ctx, userID, genreID)
}

// ジャンル一覧の取得
func (u Catalog) ListGenres(ctx context.Context, userID string) ([]domain.Genre, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// ジャンル一覧の読み出し
	return u.Repo.ListGenres(ctx, userID)
}

// ジャンル名の変更
func (u Catalog) RenameGenre(ctx context.Context, userID, genreID, name string) (domain.Genre, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	// ジャンル名の永続化
	return u.Repo.RenameGenre(ctx, userID, genreID, name)
}

// ジャンルの削除
func (u Catalog) DeleteGenre(ctx context.Context, userID, genreID string) error {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.ErrForbidden
	}
	// ジャンルの削除
	return u.Repo.DeleteGenre(ctx, userID, genreID)
}

// タスク種別の新規作成
func (u Catalog) CreateTaskType(ctx context.Context, userID, name string) (domain.TaskType, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	// タスク種別の永続化
	return u.Repo.CreateTaskType(ctx, userID, name)
}

// タスク種別の単件取得
func (u Catalog) GetTaskType(ctx context.Context, userID, taskTypeID string) (domain.TaskType, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	// タスク種別の読み出し
	return u.Repo.GetTaskType(ctx, userID, taskTypeID)
}

// タスク種別一覧の取得
func (u Catalog) ListTaskTypes(ctx context.Context, userID string) ([]domain.TaskType, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// タスク種別一覧の読み出し
	return u.Repo.ListTaskTypes(ctx, userID)
}

// ジャンル配下タスク種別一覧の取得
func (u Catalog) ListGenreTaskTypes(ctx context.Context, userID, genreID string) ([]domain.TaskType, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	// ジャンル配下種別の読み出し
	return u.Repo.ListGenreTaskTypes(ctx, userID, genreID)
}

// タスク種別名の変更
func (u Catalog) RenameTaskType(ctx context.Context, userID, taskTypeID, name string) (domain.TaskType, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	// タスク種別名の永続化
	return u.Repo.RenameTaskType(ctx, userID, taskTypeID, name)
}

// タスク種別の削除
func (u Catalog) DeleteTaskType(ctx context.Context, userID, taskTypeID string) error {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.ErrForbidden
	}
	// タスク種別の削除
	return u.Repo.DeleteTaskType(ctx, userID, taskTypeID)
}

// ジャンルへのタスク種別紐付け
func (u Catalog) AttachGenreTaskType(ctx context.Context, userID, genreID, taskTypeID string) error {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.ErrForbidden
	}
	// 種別紐付けの永続化
	return u.Repo.AttachGenreTaskType(ctx, userID, genreID, taskTypeID)
}

// ジャンルからのタスク種別解除
func (u Catalog) DetachGenreTaskType(ctx context.Context, userID, genreID, taskTypeID string) error {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.ErrForbidden
	}
	// 種別紐付けの解除
	return u.Repo.DetachGenreTaskType(ctx, userID, genreID, taskTypeID)
}
