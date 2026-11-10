package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"tasmane/internal/domain"
)

// 確認先プリセット一覧
func (p *Postgres) ListConfirmationTargets(ctx context.Context, userID string) ([]domain.ConfirmationTarget, error) {
	if p == nil || p.database == nil || userID == "" {
		return nil, domain.ErrForbidden
	}
	rows, err := p.database.QueryContext(ctx, `SELECT id, name FROM confirmation_targets WHERE user_id=$1 ORDER BY normalized_name, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list confirmation targets: %w", err)
	}
	defer rows.Close()
	result := []domain.ConfirmationTarget{}
	for rows.Next() {
		var value domain.ConfirmationTarget
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return nil, fmt.Errorf("scan confirmation target: %w", err)
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

// 確認先プリセットの登録（同一ユーザーで大文字小文字を区別せず一意）
func (p *Postgres) CreateConfirmationTarget(ctx context.Context, userID, name string) (domain.ConfirmationTarget, error) {
	name = strings.TrimSpace(name)
	if p == nil || p.database == nil || p.ids == nil || userID == "" {
		return domain.ConfirmationTarget{}, domain.ErrForbidden
	}
	if name == "" {
		return domain.ConfirmationTarget{}, domain.ErrNameRequired
	}
	id, err := p.ids.New()
	if err != nil {
		return domain.ConfirmationTarget{}, err
	}
	value := domain.ConfirmationTarget{}
	err = p.database.QueryRowContext(ctx, `INSERT INTO confirmation_targets (id, user_id, name) VALUES ($1,$2,$3) RETURNING id, name`, id, userID, name).Scan(&value.ID, &value.Name)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ConfirmationTarget{}, domain.ErrConflict
	}
	if err != nil {
		return domain.ConfirmationTarget{}, fmt.Errorf("create confirmation target: %w", err)
	}
	return value, nil
}

// 確認先プリセットの削除（既存確認事項の確認先文字列は保持）
func (p *Postgres) DeleteConfirmationTarget(ctx context.Context, userID, targetID string) error {
	if p == nil || p.database == nil || userID == "" || targetID == "" {
		return domain.ErrForbidden
	}
	result, err := p.database.ExecContext(ctx, `DELETE FROM confirmation_targets WHERE id=$1 AND user_id=$2`, targetID, userID)
	if err != nil {
		return fmt.Errorf("delete confirmation target: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.ErrForbidden
	}
	return nil
}
