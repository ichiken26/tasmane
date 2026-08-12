package repository

import (
	"context"
	"database/sql"
	"fmt"

	"tasmane/internal/domain"
)

type graphQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

const graphTasksSQL = `SELECT id, user_id, genre_id, COALESCE(parent_task_id::text, '') AS parent_id
FROM tasks WHERE user_id=$1 AND deleted_at IS NULL ORDER BY id`

const graphDependenciesSQL = `SELECT dependency.task_id, dependency.depends_on_task_id
FROM task_dependencies dependency
JOIN tasks task ON task.id=dependency.task_id AND task.user_id=dependency.user_id
JOIN tasks required ON required.id=dependency.depends_on_task_id AND required.user_id=dependency.user_id
WHERE dependency.user_id=$1 AND task.deleted_at IS NULL AND required.deleted_at IS NULL
ORDER BY dependency.task_id, dependency.depends_on_task_id`

func (p *Postgres) LoadGraph(ctx context.Context, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, nil, domain.ErrForbidden
	}
	return loadGraph(ctx, p.database, userID)
}

func loadGraph(ctx context.Context, querier graphQuerier, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	rows, err := querier.QueryContext(ctx, graphTasksSQL, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("load graph tasks: %w", err)
	}
	tasks := []domain.GraphTask{}
	for rows.Next() {
		var task domain.GraphTask
		if err := rows.Scan(&task.ID, &task.UserID, &task.GenreID, &task.ParentID); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan graph task: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, fmt.Errorf("load graph tasks: %w", err)
	}
	rows.Close()

	rows, err = querier.QueryContext(ctx, graphDependenciesSQL, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("load dependencies: %w", err)
	}
	defer rows.Close()
	dependencies := []domain.Dependency{}
	for rows.Next() {
		var dependency domain.Dependency
		if err := rows.Scan(&dependency.TaskID, &dependency.DependsOnTaskID); err != nil {
			return nil, nil, fmt.Errorf("scan dependency: %w", err)
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("load dependencies: %w", err)
	}
	return tasks, dependencies, nil
}

func (p *Postgres) SetParent(ctx context.Context, userID, taskID, parentID string) error {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin parent assignment: %w", err)
	}
	defer tx.Rollback()
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return err
	}
	tasks, _, err := loadGraph(ctx, tx, userID)
	if err != nil {
		return err
	}
	if err := domain.ValidateHierarchyAssignment(tasks, taskID, parentID); err != nil {
		return err
	}
	var parentValue any
	if parentID != "" {
		parentValue = parentID
	}
	result, err := tx.ExecContext(ctx, `UPDATE tasks SET parent_task_id=$1
    WHERE id=$2 AND user_id=$3 AND deleted_at IS NULL`, parentValue, taskID, userID)
	if err != nil {
		return fmt.Errorf("set parent: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set parent result: %w", err)
	}
	if count != 1 {
		return domain.ErrForbidden
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit parent assignment: %w", err)
	}
	return nil
}

func (p *Postgres) AddDependency(ctx context.Context, userID string, dependency domain.Dependency) error {
	if p == nil || p.database == nil || userID == "" {
		return domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dependency addition: %w", err)
	}
	defer tx.Rollback()
	if err := lockUserGraph(ctx, tx, userID); err != nil {
		return err
	}
	tasks, dependencies, err := loadGraph(ctx, tx, userID)
	if err != nil {
		return err
	}
	if err := domain.ValidateDependencyAddition(tasks, dependencies, dependency.TaskID, dependency.DependsOnTaskID); err != nil {
		return err
	}
	var genreID string
	for _, task := range tasks {
		if task.ID == dependency.TaskID {
			genreID = task.GenreID
			break
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO task_dependencies
    (user_id, genre_id, task_id, depends_on_task_id) VALUES ($1, $2, $3, $4)
    ON CONFLICT (task_id, depends_on_task_id) DO NOTHING`,
		userID, genreID, dependency.TaskID, dependency.DependsOnTaskID)
	if err != nil {
		return fmt.Errorf("add dependency: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("add dependency result: %w", err)
	}
	if count != 1 {
		return domain.ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dependency addition: %w", err)
	}
	return nil
}

func (p *Postgres) ListTaskDependencies(ctx context.Context, userID, taskID string) ([]domain.Dependency, error) {
	tasks, dependencies, err := p.LoadGraph(ctx, userID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, task := range tasks {
		found = found || task.ID == taskID
	}
	if !found {
		return nil, domain.ErrForbidden
	}
	result := []domain.Dependency{}
	for _, dependency := range dependencies {
		if dependency.TaskID == taskID {
			result = append(result, dependency)
		}
	}
	return result, nil
}

func lockUserGraph(ctx context.Context, tx *sql.Tx, userID string) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userID); err != nil {
		return fmt.Errorf("lock user graph: %w", err)
	}
	return nil
}

func (p *Postgres) RemoveDependency(ctx context.Context, userID string, dependency domain.Dependency) error {
	if p == nil || p.database == nil || userID == "" || dependency.TaskID == "" || dependency.DependsOnTaskID == "" {
		return domain.ErrForbidden
	}
	result, err := p.database.ExecContext(ctx, `DELETE FROM task_dependencies
    WHERE user_id=$1 AND task_id=$2 AND depends_on_task_id=$3`, userID, dependency.TaskID, dependency.DependsOnTaskID)
	if err != nil {
		return fmt.Errorf("remove dependency: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove dependency result: %w", err)
	}
	if count != 1 {
		return domain.ErrForbidden
	}
	return nil
}
