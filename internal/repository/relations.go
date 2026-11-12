package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tasmane/internal/domain"
)

type graphQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// グラフのタスク（T）と依存（D）を1往復で読む（トランザクション内でも往復を減らす）
const graphSQL = `SELECT 'T' AS kind, id::text, user_id::text, genre_id::text, COALESCE(parent_task_id::text, '')
FROM tasks WHERE user_id=$1 AND deleted_at IS NULL
UNION ALL
SELECT 'D', dependency.task_id::text, dependency.depends_on_task_id::text, '', ''
FROM task_dependencies dependency
JOIN tasks task ON task.id=dependency.task_id AND task.user_id=dependency.user_id
JOIN tasks required ON required.id=dependency.depends_on_task_id AND required.user_id=dependency.user_id
WHERE dependency.user_id=$1 AND task.deleted_at IS NULL AND required.deleted_at IS NULL
ORDER BY 1, 2, 3`

// ユーザーグラフの読取
func (p *Postgres) LoadGraph(ctx context.Context, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, nil, domain.ErrForbidden
	}
	return loadGraph(ctx, p.database, userID)
}

// グラフタスクと依存の読取
func loadGraph(ctx context.Context, querier graphQuerier, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	rows, err := querier.QueryContext(ctx, graphSQL, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("load graph: %w", err)
	}
	defer rows.Close()
	tasks, dependencies := []domain.GraphTask{}, []domain.Dependency{}
	for rows.Next() {
		var kind, a, b, c, d string
		if err := rows.Scan(&kind, &a, &b, &c, &d); err != nil {
			return nil, nil, fmt.Errorf("scan graph: %w", err)
		}
		if kind == "T" {
			tasks = append(tasks, domain.GraphTask{ID: a, UserID: b, GenreID: c, ParentID: d})
		} else {
			dependencies = append(dependencies, domain.Dependency{TaskID: a, DependsOnTaskID: b})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("load graph: %w", err)
	}
	return tasks, dependencies, nil
}

// 親子関係の設定
func (p *Postgres) SetParent(ctx context.Context, userID, taskID, parentID string) error {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return domain.ErrForbidden
	}
	// トランザクション開始
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin parent assignment: %w", err)
	}
	defer tx.Rollback()
	// ユーザーグラフのロック取得
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
	// 親ID更新の実行
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
	// 親子付けの確定
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit parent assignment: %w", err)
	}
	return nil
}

// 依存関係の追加
func (p *Postgres) AddDependency(ctx context.Context, userID string, dependency domain.Dependency, expectedVersion int64) error {
	if p == nil || p.database == nil || userID == "" || expectedVersion < 1 {
		return domain.ErrForbidden
	}
	// トランザクション開始
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dependency addition: %w", err)
	}
	defer tx.Rollback()
	// ユーザーグラフのロック取得
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
	var currentVersion int64
	// 対象行のロック取得
	if err := tx.QueryRowContext(ctx, `SELECT version FROM tasks
    WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, dependency.TaskID, userID).Scan(&currentVersion); errors.Is(err, sql.ErrNoRows) {
		return domain.ErrForbidden
	} else if err != nil {
		return fmt.Errorf("lock dependency task version: %w", err)
	}
	if currentVersion != expectedVersion {
		return domain.ErrVersionConflict
	}
	var genreID string
	for _, task := range tasks {
		if task.ID == dependency.TaskID {
			genreID = task.GenreID
			break
		}
	}
	// 依存行の挿入
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
	// 版番号の加算
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET version=version+1
    WHERE id=$1 AND user_id=$2 AND version=$3 AND deleted_at IS NULL`, dependency.TaskID, userID, expectedVersion); err != nil {
		return fmt.Errorf("increment dependency task version: %w", err)
	}
	// 依存追加の確定
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dependency addition: %w", err)
	}
	return nil
}

// タスク依存の一覧
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

// ユーザーグラフのアドバイザリロック取得
func lockUserGraph(ctx context.Context, tx *sql.Tx, userID string) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userID); err != nil {
		return fmt.Errorf("lock user graph: %w", err)
	}
	return nil
}

// 依存関係の削除
func (p *Postgres) RemoveDependency(ctx context.Context, userID string, dependency domain.Dependency) error {
	if p == nil || p.database == nil || userID == "" || dependency.TaskID == "" || dependency.DependsOnTaskID == "" {
		return domain.ErrForbidden
	}
	// 依存行の削除
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
