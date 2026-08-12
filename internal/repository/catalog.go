package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"tasmane/internal/domain"
)

func (p *Postgres) CreateGenre(ctx context.Context, userID, name string) (domain.Genre, error) {
	name = strings.TrimSpace(name)
	if p == nil || p.database == nil || p.ids == nil || userID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	if name == "" {
		return domain.Genre{}, domain.ErrNameRequired
	}
	id, err := p.ids.New()
	if err != nil {
		return domain.Genre{}, fmt.Errorf("generate genre id: %w", err)
	}
	var genre domain.Genre
	err = p.database.QueryRowContext(ctx, `INSERT INTO genres (id, user_id, name)
    VALUES ($1, $2, $3) RETURNING id, user_id, name`, id, userID, name).
		Scan(&genre.ID, &genre.UserID, &genre.Name)
	if err != nil {
		return domain.Genre{}, catalogWriteError("create genre", err)
	}
	return genre, nil
}

func (p *Postgres) GetGenre(ctx context.Context, userID, genreID string) (domain.Genre, error) {
	if p == nil || p.database == nil || userID == "" || genreID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	var genre domain.Genre
	err := p.database.QueryRowContext(ctx, `SELECT id, user_id, name FROM genres
    WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, genreID, userID).
		Scan(&genre.ID, &genre.UserID, &genre.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Genre{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Genre{}, fmt.Errorf("get genre: %w", err)
	}
	return genre, nil
}

func (p *Postgres) ListGenres(ctx context.Context, userID string) ([]domain.Genre, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, domain.ErrForbidden
	}
	rows, err := p.database.QueryContext(ctx, `SELECT id, user_id, name FROM genres
    WHERE user_id=$1 AND deleted_at IS NULL ORDER BY normalized_name, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	defer rows.Close()
	genres := []domain.Genre{}
	for rows.Next() {
		var genre domain.Genre
		if err := rows.Scan(&genre.ID, &genre.UserID, &genre.Name); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		genres = append(genres, genre)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	return genres, nil
}

func (p *Postgres) RenameGenre(ctx context.Context, userID, genreID, name string) (domain.Genre, error) {
	name = strings.TrimSpace(name)
	if p == nil || p.database == nil || userID == "" || genreID == "" {
		return domain.Genre{}, domain.ErrForbidden
	}
	if name == "" {
		return domain.Genre{}, domain.ErrNameRequired
	}
	var genre domain.Genre
	err := p.database.QueryRowContext(ctx, `UPDATE genres SET name=$1
    WHERE id=$2 AND user_id=$3 AND deleted_at IS NULL RETURNING id, user_id, name`, name, genreID, userID).
		Scan(&genre.ID, &genre.UserID, &genre.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Genre{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Genre{}, catalogWriteError("rename genre", err)
	}
	return genre, nil
}

func (p *Postgres) DeleteGenre(ctx context.Context, userID, genreID string) error {
	if p == nil || p.database == nil || userID == "" || genreID == "" {
		return domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin genre delete: %w", err)
	}
	defer tx.Rollback()
	var lockedID string
	if err := tx.QueryRowContext(ctx, catalogLockSQL("genres"), genreID, userID).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return domain.ErrForbidden
	} else if err != nil {
		return fmt.Errorf("lock genre: %w", err)
	}
	var inUse bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM tasks WHERE genre_id=$1 AND user_id=$2 AND deleted_at IS NULL)`, genreID, userID).Scan(&inUse); err != nil {
		return fmt.Errorf("check genre use: %w", err)
	}
	if inUse {
		return domain.ErrResourceInUse
	}
	if _, err := tx.ExecContext(ctx, `UPDATE genres SET deleted_at=now() WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, genreID, userID); err != nil {
		return fmt.Errorf("delete genre: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit genre delete: %w", err)
	}
	return nil
}

func catalogLockSQL(table string) string {
	return `SELECT id FROM ` + table + ` WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`
}

func catalogWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return domain.ErrConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func (p *Postgres) CreateTaskType(ctx context.Context, userID, name string) (domain.TaskType, error) {
	name = strings.TrimSpace(name)
	if p == nil || p.database == nil || p.ids == nil || userID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	if name == "" {
		return domain.TaskType{}, domain.ErrNameRequired
	}
	id, err := p.ids.New()
	if err != nil {
		return domain.TaskType{}, fmt.Errorf("generate task type id: %w", err)
	}
	var taskType domain.TaskType
	err = p.database.QueryRowContext(ctx, `INSERT INTO task_types (id, user_id, name)
    VALUES ($1, $2, $3) RETURNING id, user_id, name`, id, userID, name).
		Scan(&taskType.ID, &taskType.UserID, &taskType.Name)
	if err != nil {
		return domain.TaskType{}, catalogWriteError("create task type", err)
	}
	return taskType, nil
}

func (p *Postgres) GetTaskType(ctx context.Context, userID, taskTypeID string) (domain.TaskType, error) {
	if p == nil || p.database == nil || userID == "" || taskTypeID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	var taskType domain.TaskType
	err := p.database.QueryRowContext(ctx, `SELECT id, user_id, name FROM task_types
    WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, taskTypeID, userID).
		Scan(&taskType.ID, &taskType.UserID, &taskType.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TaskType{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.TaskType{}, fmt.Errorf("get task type: %w", err)
	}
	return taskType, nil
}

func (p *Postgres) ListTaskTypes(ctx context.Context, userID string) ([]domain.TaskType, error) {
	return p.listTaskTypes(ctx, `SELECT id, user_id, name FROM task_types
    WHERE user_id=$1 AND deleted_at IS NULL ORDER BY normalized_name, id`, userID)
}

func (p *Postgres) ListGenreTaskTypes(ctx context.Context, userID, genreID string) ([]domain.TaskType, error) {
	if genreID == "" {
		return nil, domain.ErrForbidden
	}
	return p.listTaskTypes(ctx, `SELECT task_type.id, task_type.user_id, task_type.name
    FROM task_types task_type
    JOIN genre_task_types link ON link.task_type_id=task_type.id AND link.user_id=task_type.user_id
    JOIN genres genre ON genre.id=link.genre_id AND genre.user_id=link.user_id
    WHERE link.user_id=$1 AND link.genre_id=$2
      AND task_type.deleted_at IS NULL AND genre.deleted_at IS NULL
    ORDER BY task_type.normalized_name, task_type.id`, userID, genreID)
}

func (p *Postgres) listTaskTypes(ctx context.Context, query string, args ...any) ([]domain.TaskType, error) {
	if p == nil || p.database == nil || len(args) == 0 || args[0] == "" {
		return nil, domain.ErrForbidden
	}
	rows, err := p.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list task types: %w", err)
	}
	defer rows.Close()
	taskTypes := []domain.TaskType{}
	for rows.Next() {
		var taskType domain.TaskType
		if err := rows.Scan(&taskType.ID, &taskType.UserID, &taskType.Name); err != nil {
			return nil, fmt.Errorf("scan task type: %w", err)
		}
		taskTypes = append(taskTypes, taskType)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list task types: %w", err)
	}
	return taskTypes, nil
}

func (p *Postgres) RenameTaskType(ctx context.Context, userID, taskTypeID, name string) (domain.TaskType, error) {
	name = strings.TrimSpace(name)
	if p == nil || p.database == nil || userID == "" || taskTypeID == "" {
		return domain.TaskType{}, domain.ErrForbidden
	}
	if name == "" {
		return domain.TaskType{}, domain.ErrNameRequired
	}
	var taskType domain.TaskType
	err := p.database.QueryRowContext(ctx, `UPDATE task_types SET name=$1
    WHERE id=$2 AND user_id=$3 AND deleted_at IS NULL RETURNING id, user_id, name`, name, taskTypeID, userID).
		Scan(&taskType.ID, &taskType.UserID, &taskType.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TaskType{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.TaskType{}, catalogWriteError("rename task type", err)
	}
	return taskType, nil
}

func (p *Postgres) AttachGenreTaskType(ctx context.Context, userID, genreID, taskTypeID string) error {
	if p == nil || p.database == nil || userID == "" || genreID == "" || taskTypeID == "" {
		return domain.ErrForbidden
	}
	result, err := p.database.ExecContext(ctx, `INSERT INTO genre_task_types (user_id, genre_id, task_type_id)
    SELECT $1, genre.id, task_type.id
    FROM genres genre
    JOIN task_types task_type ON task_type.user_id=genre.user_id
    WHERE genre.id=$2 AND task_type.id=$3 AND genre.user_id=$1
      AND genre.deleted_at IS NULL AND task_type.deleted_at IS NULL
    ON CONFLICT (genre_id, task_type_id) DO NOTHING`, userID, genreID, taskTypeID)
	if err != nil {
		return fmt.Errorf("attach genre task type: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("attach genre task type result: %w", err)
	}
	if count == 1 {
		return nil
	}
	var exists bool
	if err := p.database.QueryRowContext(ctx, `SELECT EXISTS (
      SELECT 1 FROM genre_task_types WHERE user_id=$1 AND genre_id=$2 AND task_type_id=$3
    )`, userID, genreID, taskTypeID).Scan(&exists); err != nil {
		return fmt.Errorf("check genre task type: %w", err)
	}
	if !exists {
		return domain.ErrForbidden
	}
	return nil
}

func (p *Postgres) DetachGenreTaskType(ctx context.Context, userID, genreID, taskTypeID string) error {
	if p == nil || p.database == nil || userID == "" || genreID == "" || taskTypeID == "" {
		return domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task type detach: %w", err)
	}
	defer tx.Rollback()
	var lockedGenreID string
	if err := tx.QueryRowContext(ctx, `SELECT genre_id FROM genre_task_types
    WHERE user_id=$1 AND genre_id=$2 AND task_type_id=$3 FOR UPDATE`, userID, genreID, taskTypeID).Scan(&lockedGenreID); errors.Is(err, sql.ErrNoRows) {
		return domain.ErrForbidden
	} else if err != nil {
		return fmt.Errorf("lock genre task type: %w", err)
	}
	var inUse bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM tasks
    WHERE user_id=$1 AND genre_id=$2 AND task_type_id=$3 AND deleted_at IS NULL)`, userID, genreID, taskTypeID).Scan(&inUse); err != nil {
		return fmt.Errorf("check task type use: %w", err)
	}
	if inUse {
		return domain.ErrResourceInUse
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM genre_task_types WHERE user_id=$1 AND genre_id=$2 AND task_type_id=$3`, userID, genreID, taskTypeID); err != nil {
		return fmt.Errorf("detach genre task type: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task type detach: %w", err)
	}
	return nil
}

func (p *Postgres) DeleteTaskType(ctx context.Context, userID, taskTypeID string) error {
	if p == nil || p.database == nil || userID == "" || taskTypeID == "" {
		return domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task type delete: %w", err)
	}
	defer tx.Rollback()
	var lockedID string
	if err := tx.QueryRowContext(ctx, catalogLockSQL("task_types"), taskTypeID, userID).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return domain.ErrForbidden
	} else if err != nil {
		return fmt.Errorf("lock task type: %w", err)
	}
	var inUse bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM tasks
    WHERE task_type_id=$1 AND user_id=$2 AND deleted_at IS NULL)`, taskTypeID, userID).Scan(&inUse); err != nil {
		return fmt.Errorf("check task type use: %w", err)
	}
	if inUse {
		return domain.ErrResourceInUse
	}
	if _, err := tx.ExecContext(ctx, `UPDATE task_types SET deleted_at=now() WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, taskTypeID, userID); err != nil {
		return fmt.Errorf("delete task type: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task type delete: %w", err)
	}
	return nil
}
