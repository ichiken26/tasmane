package application

import (
	"context"

	"tasmane/internal/domain"
)

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

type Catalog struct{ Repo CatalogRepository }

func (u Catalog) CreateGenre(ctx context.Context, userID, name string) (domain.Genre, error) {
	if userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	return u.Repo.CreateGenre(ctx, userID, name)
}

func (u Catalog) GetGenre(ctx context.Context, userID, genreID string) (domain.Genre, error) {
	if userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	return u.Repo.GetGenre(ctx, userID, genreID)
}

func (u Catalog) ListGenres(ctx context.Context, userID string) ([]domain.Genre, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListGenres(ctx, userID)
}

func (u Catalog) RenameGenre(ctx context.Context, userID, genreID, name string) (domain.Genre, error) {
	if userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	return u.Repo.RenameGenre(ctx, userID, genreID, name)
}

func (u Catalog) DeleteGenre(ctx context.Context, userID, genreID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	return u.Repo.DeleteGenre(ctx, userID, genreID)
}

func (u Catalog) CreateTaskType(ctx context.Context, userID, name string) (domain.TaskType, error) {
	if userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	return u.Repo.CreateTaskType(ctx, userID, name)
}

func (u Catalog) GetTaskType(ctx context.Context, userID, taskTypeID string) (domain.TaskType, error) {
	if userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	return u.Repo.GetTaskType(ctx, userID, taskTypeID)
}

func (u Catalog) ListTaskTypes(ctx context.Context, userID string) ([]domain.TaskType, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListTaskTypes(ctx, userID)
}

func (u Catalog) ListGenreTaskTypes(ctx context.Context, userID, genreID string) ([]domain.TaskType, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListGenreTaskTypes(ctx, userID, genreID)
}

func (u Catalog) RenameTaskType(ctx context.Context, userID, taskTypeID, name string) (domain.TaskType, error) {
	if userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	return u.Repo.RenameTaskType(ctx, userID, taskTypeID, name)
}

func (u Catalog) DeleteTaskType(ctx context.Context, userID, taskTypeID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	return u.Repo.DeleteTaskType(ctx, userID, taskTypeID)
}

func (u Catalog) AttachGenreTaskType(ctx context.Context, userID, genreID, taskTypeID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	return u.Repo.AttachGenreTaskType(ctx, userID, genreID, taskTypeID)
}

func (u Catalog) DetachGenreTaskType(ctx context.Context, userID, genreID, taskTypeID string) error {
	if userID == "" {
		return domain.ErrForbidden
	}
	return u.Repo.DetachGenreTaskType(ctx, userID, genreID, taskTypeID)
}
