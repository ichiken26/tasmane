package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"tasmane/internal/domain"
	"time"
)

// Google起点変更の往復同期抑止
func WithGoogleInbound(ctx context.Context) context.Context { return domain.WithGoogleInbound(ctx) }

// Google起点コンテキストの判定
func isGoogleInbound(ctx context.Context) bool { return domain.IsGoogleInbound(ctx) }

// Google同期ジョブの投入
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
	// ジョブ行の挿入
	_, e = p.database.ExecContext(ctx, `INSERT INTO google_sync_jobs(id,user_id,task_id,action,attempt,available_at)VALUES($1,$2,$3,$4,$5,$6)`, id, j.UserID, taskID, j.Action, j.Attempt, time.Now().UTC().Add(delay))
	return e
}

// 同一トランザクション内の同期ジョブ投入
func (p *Postgres) enqueueGoogleTaskTx(ctx context.Context, tx *sql.Tx, userID, taskID string, action domain.GoogleJobAction) error {
	id, err := p.ids.New()
	if err != nil {
		return err
	}
	// 重複除外付きジョブ挿入（同期中＝クレーム済みのジョブは後からの変更を読まない可能性があるため除外対象にしない）
	_, err = tx.ExecContext(ctx, `INSERT INTO google_sync_jobs(id,user_id,task_id,action)
    SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM google_integrations WHERE user_id=$2)
      AND NOT EXISTS (SELECT 1 FROM google_unlinked_tasks WHERE user_id=$2 AND task_id=$3)
      AND NOT EXISTS (SELECT 1 FROM google_sync_jobs WHERE user_id=$2 AND task_id=$3 AND action=$4 AND claimed_at IS NULL)`, id, userID, taskID, action)
	if err != nil {
		return fmt.Errorf("enqueue google task transactionally: %w", err)
	}
	return nil
}

// 同期ジョブのリース取得
func (p *Postgres) Claim(ctx context.Context, workerID string, now time.Time, lease time.Duration) (domain.GoogleJob, bool, error) {
	if workerID == "" || lease <= 0 {
		return domain.GoogleJob{}, false, domain.ErrForbidden
	}
	var job domain.GoogleJob
	var taskID sql.NullString
	// SKIP LOCKEDによる候補取得とリース更新
	err := p.database.QueryRowContext(ctx, `WITH candidate AS (
    SELECT id FROM google_sync_jobs
    WHERE available_at <= $1 AND (claimed_at IS NULL OR claimed_at < $1-$3::interval)
    ORDER BY available_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1
  ) UPDATE google_sync_jobs job SET claimed_at=$1,claimed_by=$2
    FROM candidate WHERE job.id=candidate.id
    RETURNING job.id,job.user_id,job.task_id,job.action,job.attempt`, now, workerID, intervalLiteral(lease)).Scan(
		&job.ID, &job.UserID, &taskID, &job.Action, &job.Attempt,
	)
	// 候補なし時の空結果変換
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

// リース間隔リテラルの生成
func intervalLiteral(duration time.Duration) string {
	return fmt.Sprintf("%f seconds", duration.Seconds())
}

// 同期ジョブの完了削除
func (p *Postgres) Ack(ctx context.Context, jobID, workerID string) error {
	// 所有ジョブの削除
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

// Google連携アカウント
type GoogleIntegration struct {
	ID, UserID, GoogleSubject, EncryptedRefreshToken string
	TaskListID                                       *string
	LastSyncFinishedAt                               *time.Time
}

// Googleタスク同期対応
type GoogleTaskSync struct {
	ID, UserID, TaskID, GoogleTaskListID, GoogleTaskID string
	Snapshot                                           []byte
}

// タスク同期対応の取得
func (p *Postgres) GetGoogleTaskSync(ctx context.Context, userID, taskID string) (GoogleTaskSync, error) {
	var value GoogleTaskSync
	var snapshot []byte
	// 同期対応行のスキャン
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

// Google側IDからの同期対応検索
func (p *Postgres) FindGoogleTaskSync(ctx context.Context, userID, listID, googleTaskID string) (GoogleTaskSync, error) {
	var value GoogleTaskSync
	var taskID sql.NullString
	// 同期対応行のスキャン
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

// タスク同期対応の保存
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
	// 同期対応のupsert
	_, err := p.database.ExecContext(ctx, `INSERT INTO google_task_syncs(id,user_id,task_id,google_task_list_id,google_task_id,last_synced_at,status,last_synced_snapshot)
    VALUES($1,$2,$3,$4,$5,now(),'SYNCED',$6)
    ON CONFLICT(user_id,google_task_list_id,google_task_id) DO UPDATE SET task_id=COALESCE(EXCLUDED.task_id,google_task_syncs.task_id),last_synced_at=now(),status='SYNCED',last_synced_snapshot=EXCLUDED.last_synced_snapshot,last_error_code=NULL,last_error_message=NULL`,
		value.ID, value.UserID, taskID, value.GoogleTaskListID, value.GoogleTaskID, jsonParam(value.Snapshot))
	return err
}

// JSON 列への引数（[]byte は bytea として送られるため文字列で渡す。空は NULL）
func jsonParam(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

// タスク同期対応の切断
func (p *Postgres) DisconnectGoogleTask(ctx context.Context, userID, taskID string) error {
	// 同期対応行の削除
	_, err := p.database.ExecContext(ctx, `DELETE FROM google_task_syncs WHERE user_id=$1 AND task_id=$2`, userID, taskID)
	return err
}

// Google競合の記録
func (p *Postgres) RecordGoogleConflict(ctx context.Context, userID, taskID, code, message string) error {
	return p.RecordGoogleConflictWithRemote(ctx, userID, taskID, code, message, nil)
}

// Google競合の記録（Google側値の保存付き）
func (p *Postgres) RecordGoogleConflictWithRemote(ctx context.Context, userID, taskID, code, message string, remote []byte) error {
	// 競合状態の更新
	result, err := p.database.ExecContext(ctx, `UPDATE google_task_syncs SET status='CONFLICT',last_error_code=$1,last_error_message=$2,conflict_remote_snapshot=$5 WHERE user_id=$3 AND task_id=$4`, code, message, userID, taskID, jsonParam(remote))
	if err != nil {
		return fmt.Errorf("record google conflict: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.ErrForbidden
	}
	return nil
}

// Google期限更新の影響計画
func (p *Postgres) PlanGoogleDueUpdate(ctx context.Context, userID, taskID string, endAt *time.Time) ([]domain.DateChange, error) {
	// 日付グラフの読取
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
		// 日付行のスキャン
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

// Google連携の保存
func (p *Postgres) SaveGoogleIntegration(ctx context.Context, v GoogleIntegration) error {
	if p == nil || p.database == nil || v.ID == "" || v.UserID == "" || v.EncryptedRefreshToken == "" {
		return domain.ErrForbidden
	}
	// 連携行のupsert
	_, err := p.database.ExecContext(ctx, `INSERT INTO google_integrations(id,user_id,google_subject,encrypted_refresh_token,google_task_list_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id) DO UPDATE SET google_subject=EXCLUDED.google_subject,encrypted_refresh_token=EXCLUDED.encrypted_refresh_token,google_task_list_id=COALESCE(google_integrations.google_task_list_id,EXCLUDED.google_task_list_id)`, v.ID, v.UserID, v.GoogleSubject, v.EncryptedRefreshToken, v.TaskListID)
	if err != nil {
		return fmt.Errorf("save google integration: %w", err)
	}
	return nil
}

// Google連携の取得
func (p *Postgres) GetGoogleIntegration(ctx context.Context, userID string) (GoogleIntegration, error) {
	var v GoogleIntegration
	var list sql.NullString
	var finished sql.NullTime
	// 連携行のスキャン
	err := p.database.QueryRowContext(ctx, `SELECT id,user_id,google_subject,encrypted_refresh_token,google_task_list_id,last_sync_finished_at FROM google_integrations WHERE user_id=$1`, userID).Scan(&v.ID, &v.UserID, &v.GoogleSubject, &v.EncryptedRefreshToken, &list, &finished)
	// 未存在時の禁止エラー変換
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

// GoogleタスクリストIDの設定
func (p *Postgres) SetGoogleTaskListID(ctx context.Context, userID, listID string) error {
	// リストID更新の実行
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

// ユーザー同期ロックの取得
func (p *Postgres) AcquireGoogleSyncLock(ctx context.Context, userID string, now time.Time, d time.Duration) (bool, error) {
	var ok bool
	// 同期ロックの更新
	e := p.database.QueryRowContext(ctx, `UPDATE google_integrations SET sync_lock_until=$1,last_sync_started_at=$2 WHERE user_id=$3 AND (sync_lock_until IS NULL OR sync_lock_until<$2) RETURNING true`, now.Add(d), now, userID).Scan(&ok)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return ok, e
}

// 同期投入要否の判定
func (p *Postgres) ShouldEnqueueGoogleSync(ctx context.Context, userID string, now time.Time) (bool, error) {
	var due bool
	// 同期間隔超過の判定
	e := p.database.QueryRowContext(ctx, `SELECT integration.last_sync_finished_at IS NULL OR integration.last_sync_finished_at < $2::timestamptz-(COALESCE(settings.google_sync_interval_minutes,10)*interval '1 minute') FROM google_integrations integration LEFT JOIN user_server_settings settings ON settings.user_id=integration.user_id WHERE integration.user_id=$1`, userID, now).Scan(&due)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	return due, e
}

// 同期中状態への更新
func (p *Postgres) MarkSyncing(ctx context.Context, j domain.GoogleJob) error {
	return p.setSyncStatus(ctx, j, "SYNCING", "")
}

// 同期完了状態への更新
func (p *Postgres) MarkSynced(ctx context.Context, j domain.GoogleJob) error {
	return p.setSyncStatus(ctx, j, "SYNCED", "")
}

// 同期失敗状態への更新
func (p *Postgres) MarkFailed(ctx context.Context, j domain.GoogleJob, m string) error {
	return p.setSyncStatus(ctx, j, "FAILED", m)
}

// 同期競合状態への更新
func (p *Postgres) MarkConflict(ctx context.Context, j domain.GoogleJob, m string) error {
	return p.setSyncStatus(ctx, j, "CONFLICT", m)
}

// 同期状態の永続化
func (p *Postgres) setSyncStatus(ctx context.Context, j domain.GoogleJob, status, message string) error {
	if j.TaskID == "" {
		if status == "SYNCED" {
			// ユーザー同期完了の記録
			_, e := p.database.ExecContext(ctx, `UPDATE google_integrations SET last_sync_finished_at=now(),sync_lock_until=NULL WHERE user_id=$1`, j.UserID)
			return e
		}
		if status == "FAILED" || status == "CONFLICT" {
			// ユーザー同期ロックの解放
			_, e := p.database.ExecContext(ctx, `UPDATE google_integrations SET sync_lock_until=NULL WHERE user_id=$1`, j.UserID)
			return e
		}
		return nil
	}
	// タスク同期状態の更新
	r, e := p.database.ExecContext(ctx, `UPDATE google_task_syncs SET status=$1::sync_status,last_error_message=NULLIF($2,''),last_synced_at=CASE WHEN $1::sync_status='SYNCED' THEN now() ELSE last_synced_at END WHERE user_id=$3 AND task_id=$4`, status, message, j.UserID, j.TaskID)
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

// 未紐付けGoogleタスク候補の一覧
func (p *Postgres) ListGoogleImportCandidates(ctx context.Context, userID string) ([]domain.GoogleImportCandidate, error) {
	// 候補行の取得
	rows, e := p.database.QueryContext(ctx, `SELECT google_task_list_id,google_task_id,last_synced_snapshot FROM google_task_syncs WHERE user_id=$1 AND task_id IS NULL ORDER BY google_task_id`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.GoogleImportCandidate{}
	for rows.Next() {
		var v domain.GoogleImportCandidate
		// 候補行のスキャン
		if e := rows.Scan(&v.GoogleTaskListID, &v.GoogleTaskID, &v.Snapshot); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Google競合一覧の取得（タスマネ側・Google側の値付き）
func (p *Postgres) ListGoogleConflicts(ctx context.Context, userID string) ([]domain.GoogleConflictRecord, error) {
	rows, e := p.database.QueryContext(ctx, `SELECT sync.task_id,COALESCE(sync.last_error_code,'CONFLICT'),COALESCE(sync.last_error_message,''),
      task.title,COALESCE(task.body,''),task.end_at,task.status,COALESCE(sync.conflict_remote_snapshot,'{}'::jsonb)
    FROM google_task_syncs sync JOIN tasks task ON task.id=sync.task_id AND task.user_id=sync.user_id
    WHERE sync.user_id=$1 AND sync.status='CONFLICT' AND task.deleted_at IS NULL ORDER BY sync.updated_at DESC`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	location := p.UserLocation(ctx, userID)
	out := []domain.GoogleConflictRecord{}
	for rows.Next() {
		var v domain.GoogleConflictRecord
		var body, status string
		var endAt sql.NullTime
		var remote []byte
		if e := rows.Scan(&v.TaskID, &v.Code, &v.Message, &v.TaskTitle, &body, &endAt, &status, &remote); e != nil {
			return nil, e
		}
		v.Local = map[string]string{"title": v.TaskTitle, "notes": body, "status": "needsAction"}
		if domain.Status(status) == domain.Completed {
			v.Local["status"] = "completed"
		}
		if endAt.Valid {
			v.Local["due"] = domain.GoogleDueDate(endAt.Time, location)
		}
		v.Remote = map[string]string{}
		_ = json.Unmarshal(remote, &v.Remote)
		out = append(out, v)
	}
	return out, rows.Err()
}

// Google競合の解決（KEEP_TASMANE / RECREATE_GOOGLE / UNLINK）
func (p *Postgres) ResolveGoogleConflict(ctx context.Context, userID, taskID string, resolution domain.GoogleResolution) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin resolve google conflict: %w", err)
	}
	defer tx.Rollback()
	var code string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(last_error_code,'') FROM google_task_syncs WHERE user_id=$1 AND task_id=$2 AND status='CONFLICT' FOR UPDATE`, userID, taskID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("lock google conflict: %w", err)
	}
	switch resolution {
	case domain.GoogleKeepTasmane:
		if code == "GOOGLE_DELETED" {
			// Google側が存在しないため上書きではなく再作成を選ぶ必要がある
			return domain.ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE google_task_syncs SET status='PENDING',last_error_code=NULL,last_error_message=NULL,conflict_remote_snapshot=NULL WHERE user_id=$1 AND task_id=$2`, userID, taskID); err != nil {
			return fmt.Errorf("keep tasmane value: %w", err)
		}
		if err := p.enqueueGoogleTaskTx(ctx, tx, userID, taskID, domain.GoogleSyncTask); err != nil {
			return err
		}
	case domain.GoogleRecreate:
		if _, err := tx.ExecContext(ctx, `DELETE FROM google_task_syncs WHERE user_id=$1 AND task_id=$2`, userID, taskID); err != nil {
			return fmt.Errorf("drop deleted google link: %w", err)
		}
		if err := p.enqueueGoogleTaskTx(ctx, tx, userID, taskID, domain.GoogleRecreateTask); err != nil {
			return err
		}
	case domain.GoogleUnlink:
		if _, err := tx.ExecContext(ctx, `DELETE FROM google_task_syncs WHERE user_id=$1 AND task_id=$2`, userID, taskID); err != nil {
			return fmt.Errorf("unlink google task: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO google_unlinked_tasks(user_id,task_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userID, taskID); err != nil {
			return fmt.Errorf("record unlinked task: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM google_sync_jobs WHERE user_id=$1 AND task_id=$2 AND claimed_at IS NULL`, userID, taskID); err != nil {
			return fmt.Errorf("drop pending google jobs: %w", err)
		}
	default:
		return domain.ErrInvalidTaskInput
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resolve google conflict: %w", err)
	}
	return nil
}

// 未紐付け取込候補の取得
func (p *Postgres) GetGoogleImportCandidate(ctx context.Context, userID, listID, googleTaskID string) (domain.GoogleImportCandidate, error) {
	var v domain.GoogleImportCandidate
	err := p.database.QueryRowContext(ctx, `SELECT google_task_list_id,google_task_id,COALESCE(last_synced_snapshot,'{}'::jsonb) FROM google_task_syncs
    WHERE user_id=$1 AND google_task_list_id=$2 AND google_task_id=$3 AND task_id IS NULL`, userID, listID, googleTaskID).Scan(&v.GoogleTaskListID, &v.GoogleTaskID, &v.Snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return v, domain.ErrForbidden
	}
	return v, err
}

// 取込候補とタスクの紐付け（候補が残っている場合のみ）
func (p *Postgres) LinkGoogleImportCandidate(ctx context.Context, userID, listID, googleTaskID, taskID string) error {
	result, err := p.database.ExecContext(ctx, `UPDATE google_task_syncs SET task_id=$4,status='SYNCED',last_synced_at=now()
    WHERE user_id=$1 AND google_task_list_id=$2 AND google_task_id=$3 AND task_id IS NULL`, userID, listID, googleTaskID, taskID)
	if err != nil {
		return fmt.Errorf("link google candidate: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.ErrConflict
	}
	return nil
}

// Google側で削除された取込候補の破棄
func (p *Postgres) DeleteGoogleImportCandidate(ctx context.Context, userID, listID, googleTaskID string) error {
	_, err := p.database.ExecContext(ctx, `DELETE FROM google_task_syncs WHERE user_id=$1 AND google_task_list_id=$2 AND google_task_id=$3 AND task_id IS NULL`, userID, listID, googleTaskID)
	return err
}

// 同期投入要否の判定と同期ロック獲得（アクセス時の自動同期）
func (p *Postgres) AcquireDueGoogleSync(ctx context.Context, userID string, now time.Time, lock time.Duration) (bool, error) {
	due, err := p.ShouldEnqueueGoogleSync(ctx, userID, now)
	if err != nil || !due {
		return false, err
	}
	return p.AcquireGoogleSyncLock(ctx, userID, now, lock)
}

// Google連携の同期状態（UI表示・手動再同期用。件数と失敗一覧も1往復で読む）
func (p *Postgres) GoogleSyncStatus(ctx context.Context, userID string) (domain.GoogleSyncStatus, error) {
	status := domain.GoogleSyncStatus{Failed: []domain.GoogleFailedTask{}}
	var finished, missing sql.NullTime
	var failed []byte
	err := p.database.QueryRowContext(ctx, `SELECT integration.last_sync_finished_at, integration.task_list_missing_at,
    (SELECT count(*) FROM google_task_syncs WHERE user_id=$1 AND status='CONFLICT'),
    (SELECT count(*) FROM google_sync_jobs WHERE user_id=$1),
    (SELECT COALESCE(json_agg(json_build_object('taskId', f.task_id, 'title', f.title, 'message', f.message)), '[]') FROM (
      SELECT sync.task_id, task.title, COALESCE(sync.last_error_message,'') AS message
      FROM google_task_syncs sync JOIN tasks task ON task.id=sync.task_id AND task.user_id=sync.user_id
      WHERE sync.user_id=$1 AND sync.status='FAILED' AND task.deleted_at IS NULL ORDER BY sync.updated_at DESC LIMIT 50) f)
    FROM google_integrations integration WHERE integration.user_id=$1`, userID).
		Scan(&finished, &missing, &status.ConflictCount, &status.PendingJobs, &failed)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return status, fmt.Errorf("google sync status: %w", err)
	}
	status.Connected = true
	status.TaskListMissing = missing.Valid
	if finished.Valid {
		status.LastSyncFinishedAt = &finished.Time
	}
	if err := json.Unmarshal(failed, &status.Failed); err != nil {
		return status, fmt.Errorf("decode failed google tasks: %w", err)
	}
	return status, nil
}

// 最終FAILEDタスクの手動再同期投入
func (p *Postgres) RequeueFailedGoogleTasks(ctx context.Context, userID string) error {
	rows, err := p.database.QueryContext(ctx, `SELECT task_id FROM google_task_syncs WHERE user_id=$1 AND status='FAILED' AND task_id IS NOT NULL`, userID)
	if err != nil {
		return fmt.Errorf("list failed google tasks: %w", err)
	}
	taskIDs := []string{}
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			rows.Close()
			return err
		}
		taskIDs = append(taskIDs, taskID)
	}
	rows.Close()
	for _, taskID := range taskIDs {
		tx, err := p.database.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE google_task_syncs SET status='PENDING' WHERE user_id=$1 AND task_id=$2 AND status='FAILED'`, userID, taskID); err != nil {
			tx.Rollback()
			return err
		}
		if err := p.enqueueGoogleTaskTx(ctx, tx, userID, taskID, domain.GoogleSyncTask); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// ユーザーTimezone（未設定・不正値はUTC）
func (p *Postgres) UserLocation(ctx context.Context, userID string) *time.Location {
	var name string
	if err := p.database.QueryRowContext(ctx, `SELECT timezone FROM user_server_settings WHERE user_id=$1`, userID).Scan(&name); err != nil {
		return time.UTC
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}

// 専用 Task List の喪失を記録（復旧フローへ）
func (p *Postgres) MarkGoogleTaskListMissing(ctx context.Context, userID string) error {
	_, err := p.database.ExecContext(ctx, `UPDATE google_integrations SET task_list_missing_at=COALESCE(task_list_missing_at, now()) WHERE user_id=$1`, userID)
	return err
}

// 専用 Task List の作り直し: 紐付けを破棄し、全タスクを新しいリストへ再作成する
func (p *Postgres) ResetGoogleTaskList(ctx context.Context, userID string) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset google list: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE google_integrations SET google_task_list_id=NULL, task_list_missing_at=NULL WHERE user_id=$1`, userID)
	if err != nil {
		return fmt.Errorf("reset google list: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.ErrForbidden
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM google_task_syncs WHERE user_id=$1`, userID); err != nil {
		return fmt.Errorf("drop google links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM google_sync_jobs WHERE user_id=$1 AND claimed_at IS NULL`, userID); err != nil {
		return fmt.Errorf("drop pending google jobs: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM tasks WHERE user_id=$1 AND deleted_at IS NULL ORDER BY id`, userID)
	if err != nil {
		return fmt.Errorf("list tasks for google recreate: %w", err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := p.enqueueGoogleTaskTx(ctx, tx, userID, id, domain.GoogleSyncTask); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset google list: %w", err)
	}
	return nil
}
