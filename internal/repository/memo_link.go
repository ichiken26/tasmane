package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tasmane/internal/domain"
)

// 所有かつ未削除タスクの確認
func (p *Postgres) requireOwnedTask(ctx context.Context, userID, taskID string) error {
	var exists bool
	if err := p.database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM tasks WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, taskID, userID).Scan(&exists); err != nil {
		return fmt.Errorf("check memo link task: %w", err)
	}
	if !exists {
		return domain.ErrForbidden
	}
	return nil
}

// メモリンク一覧（登録順）
func (p *Postgres) ListMemoLinks(ctx context.Context, userID, taskID string) ([]domain.MemoLink, error) {
	if p == nil || p.database == nil || userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	if err := p.requireOwnedTask(ctx, userID, taskID); err != nil {
		return nil, err
	}
	rows, err := p.database.QueryContext(ctx, `SELECT id, task_id, url, title, memo_date, created_at FROM task_memo_links WHERE user_id=$1 AND task_id=$2 ORDER BY created_at, id`, userID, taskID)
	if err != nil {
		return nil, fmt.Errorf("list memo links: %w", err)
	}
	defer rows.Close()
	result := []domain.MemoLink{}
	for rows.Next() {
		var link domain.MemoLink
		if err := rows.Scan(&link.ID, &link.TaskID, &link.URL, &link.Title, &link.MemoDate, &link.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memo link: %w", err)
		}
		result = append(result, link)
	}
	return result, rows.Err()
}

// メモリンクの登録（同一タスク・同一URLはタイトルのみ更新）
func (p *Postgres) UpsertMemoLink(ctx context.Context, userID, taskID, url, title string) (domain.MemoLink, error) {
	if p == nil || p.database == nil || p.ids == nil || userID == "" || taskID == "" {
		return domain.MemoLink{}, domain.ErrForbidden
	}
	id, err := p.ids.New()
	if err != nil {
		return domain.MemoLink{}, err
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return domain.MemoLink{}, fmt.Errorf("begin memo link: %w", err)
	}
	defer tx.Rollback()
	var link domain.MemoLink
	err = tx.QueryRowContext(ctx, `INSERT INTO task_memo_links (id, user_id, task_id, url, title, memo_date)
SELECT $1, task.user_id, task.id, $4, $5, $6::date FROM tasks task
WHERE task.id=$3 AND task.user_id=$2 AND task.deleted_at IS NULL
ON CONFLICT (task_id, url) DO UPDATE SET title=EXCLUDED.title, memo_date=EXCLUDED.memo_date, updated_at=now()
RETURNING id, task_id, url, title, memo_date, created_at`, id, userID, taskID, url, title, memoDateArg(url)).Scan(&link.ID, &link.TaskID, &link.URL, &link.Title, &link.MemoDate, &link.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MemoLink{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.MemoLink{}, fmt.Errorf("upsert memo link: %w", err)
	}
	// Google Tasks の詳細（メモのURL）を更新する同期
	if err := p.enqueueGoogleTaskTx(ctx, tx, userID, taskID, domain.GoogleSyncTask); err != nil {
		return domain.MemoLink{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MemoLink{}, fmt.Errorf("commit memo link: %w", err)
	}
	return link, nil
}

// メモリンクの解除
func (p *Postgres) DeleteMemoLink(ctx context.Context, userID, taskID, linkID string) error {
	if p == nil || p.database == nil || userID == "" || taskID == "" || linkID == "" {
		return domain.ErrForbidden
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin memo unlink: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM task_memo_links WHERE id=$1 AND task_id=$2 AND user_id=$3`, linkID, taskID, userID)
	if err != nil {
		return fmt.Errorf("delete memo link: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.ErrForbidden
	}
	// Google Tasks の詳細（メモのURL）を更新する同期
	if err := p.enqueueGoogleTaskTx(ctx, tx, userID, taskID, domain.GoogleSyncTask); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit memo unlink: %w", err)
	}
	return nil
}

// メモURLに含まれる日付（無ければNULL）
func memoDateArg(url string) any {
	if date := domain.MemoLinkDate(url); date != nil {
		return date.Format("2006-01-02")
	}
	return nil
}
