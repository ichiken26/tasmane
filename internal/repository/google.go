package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
	"time"
)

func (p *Postgres) Enqueue(ctx context.Context, j domain.GoogleJob) error {
	id, e := p.ids.New()
	if e != nil {
		return e
	}
	var taskID any
	if j.TaskID != "" {
		taskID = j.TaskID
	}
	_, e = p.database.ExecContext(ctx, `INSERT INTO google_sync_jobs(id,user_id,task_id,action,attempt)VALUES($1,$2,$3,$4,$5)`, id, j.UserID, taskID, j.Action, j.Attempt)
	return e
}

type GoogleIntegration struct {
	ID, UserID, GoogleSubject, EncryptedRefreshToken string
	TaskListID                                       *string
	LastSyncFinishedAt                               *time.Time
}

func (p *Postgres) SaveGoogleIntegration(ctx context.Context, v GoogleIntegration) error {
	if p == nil || p.database == nil || v.ID == "" || v.UserID == "" || v.EncryptedRefreshToken == "" {
		return domain.ErrForbidden
	}
	_, err := p.database.ExecContext(ctx, `INSERT INTO google_integrations(id,user_id,google_subject,encrypted_refresh_token,google_task_list_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id) DO UPDATE SET google_subject=EXCLUDED.google_subject,encrypted_refresh_token=EXCLUDED.encrypted_refresh_token,google_task_list_id=COALESCE(google_integrations.google_task_list_id,EXCLUDED.google_task_list_id)`, v.ID, v.UserID, v.GoogleSubject, v.EncryptedRefreshToken, v.TaskListID)
	if err != nil {
		return fmt.Errorf("save google integration: %w", err)
	}
	return nil
}
func (p *Postgres) GetGoogleIntegration(ctx context.Context, userID string) (GoogleIntegration, error) {
	var v GoogleIntegration
	var list sql.NullString
	var finished sql.NullTime
	err := p.database.QueryRowContext(ctx, `SELECT id,user_id,google_subject,encrypted_refresh_token,google_task_list_id,last_sync_finished_at FROM google_integrations WHERE user_id=$1`, userID).Scan(&v.ID, &v.UserID, &v.GoogleSubject, &v.EncryptedRefreshToken, &list, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return v, domain.ErrForbidden
	}
	if err != nil {
		return v, fmt.Errorf("get google integration: %w", err)
	}
	if list.Valid {
		v.TaskListID = &list.String
	}
	if finished.Valid {
		v.LastSyncFinishedAt = &finished.Time
	}
	return v, nil
}
func (p *Postgres) SetGoogleTaskListID(ctx context.Context, userID, listID string) error {
	r, e := p.database.ExecContext(ctx, `UPDATE google_integrations SET google_task_list_id=$1 WHERE user_id=$2`, listID, userID)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return domain.ErrForbidden
	}
	return nil
}
func (p *Postgres) AcquireGoogleSyncLock(ctx context.Context, userID string, now time.Time, d time.Duration) (bool, error) {
	var ok bool
	e := p.database.QueryRowContext(ctx, `UPDATE google_integrations SET sync_lock_until=$1,last_sync_started_at=$2 WHERE user_id=$3 AND (sync_lock_until IS NULL OR sync_lock_until<$2) RETURNING true`, now.Add(d), now, userID).Scan(&ok)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return ok, e
}
func (p *Postgres) ShouldEnqueueGoogleSync(ctx context.Context, userID string, now time.Time) (bool, error) {
	var due bool
	e := p.database.QueryRowContext(ctx, `SELECT integration.last_sync_finished_at IS NULL OR integration.last_sync_finished_at < $2-(settings.google_sync_interval_minutes*interval '1 minute') FROM google_integrations integration JOIN user_server_settings settings ON settings.user_id=integration.user_id WHERE integration.user_id=$1`, userID, now).Scan(&due)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return due, e
}
func (p *Postgres) MarkSyncing(ctx context.Context, j domain.GoogleJob) error {
	return p.setSyncStatus(ctx, j, "SYNCING", "")
}
func (p *Postgres) MarkSynced(ctx context.Context, j domain.GoogleJob) error {
	return p.setSyncStatus(ctx, j, "SYNCED", "")
}
func (p *Postgres) MarkFailed(ctx context.Context, j domain.GoogleJob, m string) error {
	return p.setSyncStatus(ctx, j, "FAILED", m)
}
func (p *Postgres) MarkConflict(ctx context.Context, j domain.GoogleJob, m string) error {
	return p.setSyncStatus(ctx, j, "CONFLICT", m)
}
func (p *Postgres) setSyncStatus(ctx context.Context, j domain.GoogleJob, status, message string) error {
	if j.TaskID == "" {
		if status == "SYNCED" {
			_, e := p.database.ExecContext(ctx, `UPDATE google_integrations SET last_sync_finished_at=now(),sync_lock_until=NULL WHERE user_id=$1`, j.UserID)
			return e
		}
		return nil
	}
	r, e := p.database.ExecContext(ctx, `UPDATE google_task_syncs SET status=$1,last_error_message=NULLIF($2,''),last_synced_at=CASE WHEN $1='SYNCED' THEN now() ELSE last_synced_at END WHERE user_id=$3 AND task_id=$4`, status, message, j.UserID, j.TaskID)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return domain.ErrForbidden
	}
	return nil
}

func (p *Postgres) ListGoogleImportCandidates(ctx context.Context, userID string) ([]domain.GoogleImportCandidate, error) {
	rows, e := p.database.QueryContext(ctx, `SELECT google_task_list_id,google_task_id,last_synced_snapshot FROM google_task_syncs WHERE user_id=$1 AND task_id IS NULL ORDER BY google_task_id`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.GoogleImportCandidate{}
	for rows.Next() {
		var v domain.GoogleImportCandidate
		if e := rows.Scan(&v.GoogleTaskListID, &v.GoogleTaskID, &v.Snapshot); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) ListGoogleConflicts(ctx context.Context, userID string) ([]domain.GoogleConflictRecord, error) {
	rows, e := p.database.QueryContext(ctx, `SELECT task_id,COALESCE(last_error_code,'CONFLICT'),COALESCE(last_error_message,'') FROM google_task_syncs WHERE user_id=$1 AND status='CONFLICT' ORDER BY updated_at DESC`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.GoogleConflictRecord{}
	for rows.Next() {
		var v domain.GoogleConflictRecord
		if e := rows.Scan(&v.TaskID, &v.Code, &v.Message); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
