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
	row := p.database.QueryRowContext(ctx, `INSERT INTO task_confirmations
    (id, user_id, task_id, content, target_name, due_at)
    SELECT $1, $2, task.id, $4, $5, $6
    FROM tasks task
    WHERE task.id=$3 AND task.user_id=$2 AND task.deleted_at IS NULL
    RETURNING id, user_id, task_id, content, target_name, status, due_at, confirmed_at`,
		id, userID, taskID, content, targetName, dueAt)
	confirmation, err := scanConfirmation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Confirmation{}, fmt.Errorf("create confirmation: %w", err)
	}
	return confirmation, nil
}

func (p *Postgres) GetConfirmation(ctx context.Context, userID, confirmationID string) (domain.Confirmation, error) {
	if p == nil || p.database == nil || userID == "" || confirmationID == "" {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	row := p.database.QueryRowContext(ctx, `SELECT `+confirmationSelectColumns+`
    FROM task_confirmations confirmation
    JOIN tasks task ON task.id=confirmation.task_id AND task.user_id=confirmation.user_id
    WHERE confirmation.id=$1 AND confirmation.user_id=$2 AND task.deleted_at IS NULL`, confirmationID, userID)
	confirmation, err := scanConfirmation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Confirmation{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Confirmation{}, fmt.Errorf("get confirmation: %w", err)
	}
	return confirmation, nil
}

func (p *Postgres) ListConfirmations(ctx context.Context, userID, taskID string) ([]domain.Confirmation, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
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
	result, err := p.database.ExecContext(ctx, `UPDATE task_confirmations confirmation
    SET status=$1, confirmed_at=$2
    WHERE confirmation.id=$3 AND confirmation.user_id=$4 AND confirmation.status=$5
      AND EXISTS (SELECT 1 FROM tasks task WHERE task.id=confirmation.task_id
                  AND task.user_id=confirmation.user_id AND task.deleted_at IS NULL)`,
		confirmation.Status, confirmedAt, confirmationID, userID, confirmation.ExpectedStatus)
	if err != nil {
		return fmt.Errorf("update confirmation: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update confirmation result: %w", err)
	}
	if count != 1 {
		return domain.ErrVersionConflict
	}
	return nil
}

type confirmationScanner interface{ Scan(...any) error }

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
