package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
	"time"
)

type googleInboundContextKey struct{}

// WithGoogleInbound marks a mutation as originating at Google Tasks so the
// transactional outbox does not echo the same change back to Google.
func WithGoogleInbound(ctx context.Context) context.Context {
	return context.WithValue(ctx, googleInboundContextKey{}, true)
}

func isGoogleInbound(ctx context.Context) bool {
	value, _ := ctx.Value(googleInboundContextKey{}).(bool)
	return value
}

func (p *Postgres) Enqueue(ctx context.Context, j domain.GoogleJob) error {
	id, e := p.ids.New()
	if e != nil {
		return e
	}
	var taskID any
	if j.TaskID != "" {
		taskID = j.TaskID
	}
	delay := time.Duration(0)
	if j.Attempt > 0 {
		delay = time.Second * time.Duration(1<<min(j.Attempt, 10))
	}
	_, e = p.database.ExecContext(ctx, `INSERT INTO google_sync_jobs(id,user_id,task_id,action,attempt,available_at)VALUES($1,$2,$3,$4,$5,$6)`, id, j.UserID, taskID, j.Action, j.Attempt, time.Now().UTC().Add(delay))
	return e
}

func (p *Postgres) enqueueGoogleTaskTx(ctx context.Context, tx *sql.Tx, userID, taskID string, action domain.GoogleJobAction) error {
	id, err := p.ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO google_sync_jobs(id,user_id,task_id,action)
    SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM google_integrations WHERE user_id=$2)
      AND NOT EXISTS (SELECT 1 FROM google_sync_jobs WHERE user_id=$2 AND task_id=$3 AND action=$4)`, id, userID, taskID, action)
	if err != nil {
		return fmt.Errorf("enqueue google task transactionally: %w", err)
	}
	return nil
}

func (p *Postgres) Claim(ctx context.Context, workerID string, now time.Time, lease time.Duration) (domain.GoogleJob, bool, error) {
	if workerID == "" || lease <= 0 {
		return domain.GoogleJob{}, false, domain.ErrForbidden
	}
	var job domain.GoogleJob
	var taskID sql.NullString
	err := p.database.QueryRowContext(ctx, `WITH candidate AS (
    SELECT id FROM google_sync_jobs
    WHERE available_at <= $1 AND (claimed_at IS NULL OR claimed_at < $1-$3::interval)
    ORDER BY available_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1
  ) UPDATE google_sync_jobs job SET claimed_at=$1,claimed_by=$2
    FROM candidate WHERE job.id=candidate.id
    RETURNING job.id,job.user_id,job.task_id,job.action,job.attempt`, now, workerID, intervalLiteral(lease)).Scan(
		&job.ID, &job.UserID, &taskID, &job.Action, &job.Attempt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.GoogleJob{}, false, nil
	}
	if err != nil {
		return domain.GoogleJob{}, false, fmt.Errorf("claim google job: %w", err)
	}
	if taskID.Valid {
		job.TaskID = taskID.String
	}
	return job, true, nil
}

func intervalLiteral(duration time.Duration) string {
	return fmt.Sprintf("%f seconds", duration.Seconds())
}

func (p *Postgres) Ack(ctx context.Context, jobID, workerID string) error {
	result, err := p.database.ExecContext(ctx, `DELETE FROM google_sync_jobs WHERE id=$1 AND claimed_by=$2`, jobID, workerID)
	if err != nil {
		return fmt.Errorf("ack google job: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.ErrForbidden
	}
	return nil
}

type GoogleIntegration struct {
	ID, UserID, GoogleSubject, EncryptedRefreshToken string
	TaskListID                                       *string
	LastSyncFinishedAt                               *time.Time
}

type GoogleTaskSync struct {
	ID, UserID, TaskID, GoogleTaskListID, GoogleTaskID string
	Snapshot                                           []byte
}

func (p *Postgres) GetGoogleTaskSync(ctx context.Context, userID, taskID string) (GoogleTaskSync, error) {
	var value GoogleTaskSync
	var snapshot []byte
	err := p.database.QueryRowContext(ctx, `SELECT id,user_id,task_id,google_task_list_id,google_task_id,COALESCE(last_synced_snapshot,'{}'::jsonb) FROM google_task_syncs WHERE user_id=$1 AND task_id=$2`, userID, taskID).Scan(
		&value.ID, &value.UserID, &value.TaskID, &value.GoogleTaskListID, &value.GoogleTaskID, &snapshot,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return value, sql.ErrNoRows
	}
	if err != nil {
		return value, fmt.Errorf("get google task sync: %w", err)
	}
	value.Snapshot = snapshot
	return value, nil
}

func (p *Postgres) FindGoogleTaskSync(ctx context.Context, userID, listID, googleTaskID string) (GoogleTaskSync, error) {
	var value GoogleTaskSync
	var taskID sql.NullString
	err := p.database.QueryRowContext(ctx, `SELECT id,user_id,task_id,google_task_list_id,google_task_id,COALESCE(last_synced_snapshot,'{}'::jsonb) FROM google_task_syncs WHERE user_id=$1 AND google_task_list_id=$2 AND google_task_id=$3`, userID, listID, googleTaskID).Scan(
		&value.ID, &value.UserID, &taskID, &value.GoogleTaskListID, &value.GoogleTaskID, &value.Snapshot,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return value, sql.ErrNoRows
	}
	if err != nil {
		return value, fmt.Errorf("find google task sync: %w", err)
	}
	if taskID.Valid {
		value.TaskID = taskID.String
	}
	return value, nil
}

func (p *Postgres) SaveGoogleTaskSync(ctx context.Context, value GoogleTaskSync) error {
	if value.ID == "" {
		id, err := p.ids.New()
		if err != nil {
			return err
		}
		value.ID = id
	}
	var taskID any
	if value.TaskID != "" {
		taskID = value.TaskID
	}
	_, err := p.database.ExecContext(ctx, `INSERT INTO google_task_syncs(id,user_id,task_id,google_task_list_id,google_task_id,last_synced_at,status,last_synced_snapshot)
    VALUES($1,$2,$3,$4,$5,now(),'SYNCED',$6)
    ON CONFLICT(user_id,google_task_list_id,google_task_id) DO UPDATE SET task_id=COALESCE(EXCLUDED.task_id,google_task_syncs.task_id),last_synced_at=now(),status='SYNCED',last_synced_snapshot=EXCLUDED.last_synced_snapshot,last_error_code=NULL,last_error_message=NULL`,
		value.ID, value.UserID, taskID, value.GoogleTaskListID, value.GoogleTaskID, value.Snapshot)
	return err
}

func (p *Postgres) DisconnectGoogleTask(ctx context.Context, userID, taskID string) error {
	_, err := p.database.ExecContext(ctx, `DELETE FROM google_task_syncs WHERE user_id=$1 AND task_id=$2`, userID, taskID)
	return err
}

func (p *Postgres) RecordGoogleConflict(ctx context.Context, userID, taskID, code, message string) error {
	result, err := p.database.ExecContext(ctx, `UPDATE google_task_syncs SET status='CONFLICT',last_error_code=$1,last_error_message=$2 WHERE user_id=$3 AND task_id=$4`, code, message, userID, taskID)
	if err != nil {
		return fmt.Errorf("record google conflict: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.ErrForbidden
	}
	return nil
}

func (p *Postgres) PlanGoogleDueUpdate(ctx context.Context, userID, taskID string, endAt *time.Time) ([]domain.DateChange, error) {
	rows, err := p.database.QueryContext(ctx, `SELECT id,parent_task_id,start_at,end_at FROM tasks WHERE user_id=$1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.DateTask{}
	for rows.Next() {
		var value domain.DateTask
		var parent sql.NullString
		var start, end sql.NullTime
		if err := rows.Scan(&value.ID, &parent, &start, &end); err != nil {
			return nil, err
		}
		if parent.Valid {
			value.ParentID = parent.String
		}
		if start.Valid {
			value.StartAt = &start.Time
		}
		if end.Valid {
			value.EndAt = &end.Time
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domain.PlanGoogleDueChange(values, taskID, endAt)
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
		if status == "FAILED" || status == "CONFLICT" {
			_, e := p.database.ExecContext(ctx, `UPDATE google_integrations SET sync_lock_until=NULL WHERE user_id=$1`, j.UserID)
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
		if status == "SYNCING" {
			return nil
		}
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
