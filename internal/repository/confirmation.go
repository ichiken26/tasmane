package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tasmane/internal/domain"
)

const confirmationSelectColumns = `confirmation.id, confirmation.user_id, confirmation.task_id,
  confirmation.content, confirmation.target_name, confirmation.status,
  confirmation.due_at, confirmation.confirmed_at`

// 確認事項の作成
func (p *Postgres) CreateConfirmation(ctx context.Context, userID, taskID, content, targetName string, dueAt *time.Time) (domain.Confirmation, error) {
	content, targetName = strings.TrimSpace(content), strings.TrimSpace(targetName)
	if p == nil || p.database == nil || p.ids == nil || userID == "" || taskID == "" {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	if content == "" || targetName == "" || (dueAt != nil && dueAt.IsZero()) {
		return domain.Confirmation{}, domain.ErrInvalidConfirmationInput
	}
	id, err := p.ids.New()
	if err != nil {
		return domain.Confirmation{}, fmt.Errorf("generate confirmation id: %w", err)
	}
	// 所有中・未完了タスクへの確認事項挿入（FOR SHAREで完了処理と直列化）
	row := p.database.QueryRowContext(ctx, `INSERT INTO task_confirmations
    (id, user_id, task_id, content, target_name, due_at)
    SELECT $1, $2, task.id, $4, $5, $6
    FROM tasks task
    WHERE task.id=$3 AND task.user_id=$2 AND task.deleted_at IS NULL AND task.status <> 'COMPLETED'
    FOR SHARE OF task
    RETURNING id, user_id, task_id, content, target_name, status, due_at, confirmed_at`,
		id, userID, taskID, content, targetName, dueAt)
	confirmation, err := scanConfirmation(row)
	// 未存在・完了済みの区別
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Confirmation{}, p.confirmationTaskError(ctx, userID, taskID, domain.ErrForbidden)
	}
	if err != nil {
		return domain.Confirmation{}, fmt.Errorf("create confirmation: %w", err)
	}
	return confirmation, nil
}

// 確認事項を書き込めなかった理由（完了済みタスクか否か）
func (p *Postgres) confirmationTaskError(ctx context.Context, userID, taskID string, fallback error) error {
	var completed bool
	err := p.database.QueryRowContext(ctx, `SELECT status='COMPLETED' FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, taskID, userID).Scan(&completed)
	if err == nil && completed {
		return domain.ErrConfirmationOnCompletedTask
	}
	return fallback
}

// 確認事項の取得
func (p *Postgres) GetConfirmation(ctx context.Context, userID, confirmationID string) (domain.Confirmation, error) {
	if p == nil || p.database == nil || userID == "" || confirmationID == "" {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	// 確認事項行の取得
	row := p.database.QueryRowContext(ctx, `SELECT `+confirmationSelectColumns+`
    FROM task_confirmations confirmation
    JOIN tasks task ON task.id=confirmation.task_id AND task.user_id=confirmation.user_id
    WHERE confirmation.id=$1 AND confirmation.user_id=$2 AND task.deleted_at IS NULL`, confirmationID, userID)
	confirmation, err := scanConfirmation(row)
	// 未存在時の禁止エラー変換
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Confirmation{}, fmt.Errorf("get confirmation: %w", err)
	}
	return confirmation, nil
}

// 確認事項一覧の取得
func (p *Postgres) ListConfirmations(ctx context.Context, userID, taskID string) ([]domain.Confirmation, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	// 確認事項行の取得
	rows, err := p.database.QueryContext(ctx, `SELECT `+confirmationSelectColumns+`
    FROM task_confirmations confirmation
    JOIN tasks task ON task.id=confirmation.task_id AND task.user_id=confirmation.user_id
    WHERE confirmation.user_id=$1 AND confirmation.task_id=$2 AND task.deleted_at IS NULL
    ORDER BY confirmation.created_at, confirmation.id`, userID, taskID)
	if err != nil {
		return nil, fmt.Errorf("list confirmations: %w", err)
	}
	defer rows.Close()
	confirmations := []domain.Confirmation{}
	for rows.Next() {
		confirmation, err := scanConfirmation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan confirmation: %w", err)
		}
		confirmations = append(confirmations, confirmation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list confirmations: %w", err)
	}
	return confirmations, nil
}

// 複数タスクの確認事項一括取得（N+1回避）
func (p *Postgres) ListConfirmationsForTasks(ctx context.Context, userID string, taskIDs []string) (map[string][]domain.Confirmation, error) {
	if len(taskIDs) == 0 && p != nil && p.database != nil && userID != "" {
		return map[string][]domain.Confirmation{}, nil
	}
	return p.listConfirmationsWhere(ctx, userID, `confirmation.task_id=ANY($2::uuid[])`, taskIDs)
}

// ジャンル単位（空なら全ジャンル）の確認事項一括取得（投影用。タスク一覧と並行に読める）
func (p *Postgres) ListConfirmationsForGenres(ctx context.Context, userID string, genreIDs []string) (map[string][]domain.Confirmation, error) {
	return p.listConfirmationsWhere(ctx, userID, `(cardinality($2::uuid[])=0 OR task.genre_id=ANY($2::uuid[]))`, genreIDs)
}

// 削除済みタスクを除いた確認事項のタスク別一括取得
func (p *Postgres) listConfirmationsWhere(ctx context.Context, userID, filter string, ids []string) (map[string][]domain.Confirmation, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, domain.ErrForbidden
	}
	if ids == nil {
		ids = []string{}
	}
	rows, err := p.database.QueryContext(ctx, `SELECT `+confirmationSelectColumns+`
    FROM task_confirmations confirmation
    JOIN tasks task ON task.id=confirmation.task_id AND task.user_id=confirmation.user_id
    WHERE confirmation.user_id=$1 AND `+filter+` AND task.deleted_at IS NULL
    ORDER BY confirmation.created_at, confirmation.id`, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("list confirmations: %w", err)
	}
	defer rows.Close()
	result := map[string][]domain.Confirmation{}
	for rows.Next() {
		confirmation, err := scanConfirmation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan confirmation: %w", err)
		}
		result[confirmation.TaskID] = append(result[confirmation.TaskID], confirmation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list confirmations: %w", err)
	}
	return result, nil
}

// 確認事項状態の更新
func (p *Postgres) UpdateConfirmation(ctx context.Context, userID, confirmationID string, confirmation domain.Confirmation) error {
	if p == nil || p.database == nil || userID == "" || confirmationID == "" ||
		confirmation.ID != confirmationID || confirmation.UserID != userID || confirmation.ExpectedStatus == "" {
		return domain.ErrForbidden
	}
	var confirmedAt any
	if confirmation.ConfirmedAt != nil {
		if confirmation.ConfirmedAt.IsZero() {
			return domain.ErrInvalidConfirmationInput
		}
		confirmedAt = *confirmation.ConfirmedAt
	}
	// 期待状態付き更新の実行（確認済み以外へ戻すのは未完了タスクのみ）
	result, err := p.database.ExecContext(ctx, `UPDATE task_confirmations confirmation
    SET status=$1, confirmed_at=$2
    WHERE confirmation.id=$3 AND confirmation.user_id=$4 AND confirmation.status=$5
      AND EXISTS (SELECT 1 FROM tasks task WHERE task.id=confirmation.task_id
                  AND task.user_id=confirmation.user_id AND task.deleted_at IS NULL
                  AND ($1::confirmation_status='CONFIRMED' OR task.status <> 'COMPLETED') FOR SHARE)`,
		confirmation.Status, confirmedAt, confirmationID, userID, confirmation.ExpectedStatus)
	if err != nil {
		return fmt.Errorf("update confirmation: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update confirmation result: %w", err)
	}
	if count != 1 {
		if confirmation.Status != domain.ConfirmationConfirmed && confirmation.TaskID != "" {
			return p.confirmationTaskError(ctx, userID, confirmation.TaskID, domain.ErrVersionConflict)
		}
		return domain.ErrVersionConflict
	}
	return nil
}

type confirmationScanner interface{ Scan(...any) error }

// 確認事項行のスキャン
func scanConfirmation(scanner confirmationScanner) (domain.Confirmation, error) {
	var confirmation domain.Confirmation
	var status string
	var dueAt, confirmedAt sql.NullTime
	err := scanner.Scan(
		&confirmation.ID, &confirmation.UserID, &confirmation.TaskID,
		&confirmation.Content, &confirmation.TargetName, &status, &dueAt, &confirmedAt,
	)
	if err != nil {
		return domain.Confirmation{}, err
	}
	confirmation.Status = domain.ConfirmationStatus(status)
	if dueAt.Valid {
		confirmation.DueAt = &dueAt.Time
	}
	if confirmedAt.Valid {
		confirmation.ConfirmedAt = &confirmedAt.Time
	}
	return confirmation, nil
}
