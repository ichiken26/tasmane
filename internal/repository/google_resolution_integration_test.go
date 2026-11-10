package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"tasmane/internal/domain"
)

// コンフリクト解決・連携解除・取込候補紐付け・確認先プリセットの統合検証
func TestPostgresGoogleResolutionAgainstFreshSchema(t *testing.T) {
	database := freshSchemaDatabase(t, "google_resolution_test_")
	ctx := context.Background()
	const (
		userID = "01890f3e-4c00-7000-8000-000000000401"
		genre  = "01890f3e-4c00-7000-8000-000000000402"
		task   = "01890f3e-4c00-7000-8000-000000000403"
		other  = "01890f3e-4c00-7000-8000-000000000404"
	)
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	mustExec(`INSERT INTO users (id,google_subject,email) VALUES ($1,'g-user','g@example.com')`, userID)
	mustExec(`INSERT INTO genres (id,user_id,name) VALUES ($1,$2,'Work')`, genre, userID)
	mustExec(`INSERT INTO tasks (id,user_id,genre_id,title) VALUES ($1,$3,$4,'Synced'),($2,$3,$4,'Imported')`, task, other, userID, genre)
	mustExec(`INSERT INTO google_integrations (id,user_id,google_subject,encrypted_refresh_token) VALUES ('01890f3e-4c00-7000-8000-000000000410',$1,'g-user','x')`, userID)
	mustExec(`INSERT INTO google_task_syncs (id,user_id,task_id,google_task_list_id,google_task_id,status) VALUES ('01890f3e-4c00-7000-8000-000000000411',$1,$2,'list','remote-1','SYNCED')`, userID, task)
	mustExec(`INSERT INTO google_task_syncs (id,user_id,task_id,google_task_list_id,google_task_id,status,last_synced_snapshot) VALUES ('01890f3e-4c00-7000-8000-000000000412',$1,NULL,'list','remote-2','SYNCED','{"title":"Google only"}')`, userID)

	ids := &fixedIDs{values: []string{
		"01890f3e-4c00-7000-8000-000000000490", "01890f3e-4c00-7000-8000-000000000491",
		"01890f3e-4c00-7000-8000-000000000492", "01890f3e-4c00-7000-8000-000000000493",
		"01890f3e-4c00-7000-8000-000000000494", "01890f3e-4c00-7000-8000-000000000495",
		"01890f3e-4c00-7000-8000-000000000496", "01890f3e-4c00-7000-8000-000000000497",
		"01890f3e-4c00-7000-8000-000000000498", "01890f3e-4c00-7000-8000-000000000499",
	}}
	repo := NewPostgres(database, ids)

	// コンフリクトの記録と双方値付き一覧
	if err := repo.RecordGoogleConflictWithRemote(ctx, userID, task, "FIELD_CONFLICT", "title", []byte(`{"title":"Remote title","status":"needsAction"}`)); err != nil {
		t.Fatal(err)
	}
	conflicts, err := repo.ListGoogleConflicts(ctx, userID)
	if err != nil || len(conflicts) != 1 || conflicts[0].Remote["title"] != "Remote title" || conflicts[0].Local["title"] != "Synced" {
		t.Fatalf("conflicts=%+v err=%v", conflicts, err)
	}
	// タスマネ値の採用でPENDINGへ戻り同期ジョブ投入
	if err := repo.ResolveGoogleConflict(ctx, userID, task, domain.GoogleKeepTasmane); err != nil {
		t.Fatal(err)
	}
	var jobs int
	if err := database.QueryRow(`SELECT count(*) FROM google_sync_jobs WHERE task_id=$1 AND action='SYNC_TASK'`, task).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("jobs=%d err=%v", jobs, err)
	}
	if err := repo.ResolveGoogleConflict(ctx, userID, task, domain.GoogleKeepTasmane); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("resolving non-conflict must fail: %v", err)
	}
	// 連携解除後はGoogle同期ジョブを投入しない
	if err := repo.RecordGoogleConflict(ctx, userID, task, "GOOGLE_DELETED", "deleted"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ResolveGoogleConflict(ctx, userID, task, domain.GoogleUnlink); err != nil {
		t.Fatal(err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.enqueueGoogleTaskTx(ctx, tx, userID, task, domain.GoogleSyncTask); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM google_sync_jobs WHERE task_id=$1`, task).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("unlinked task jobs=%d err=%v", jobs, err)
	}
	// 取込候補の取得と紐付け（二重紐付けは拒否）
	candidate, err := repo.GetGoogleImportCandidate(ctx, userID, "list", "remote-2")
	if err != nil || candidate.GoogleTaskID != "remote-2" {
		t.Fatalf("candidate=%+v err=%v", candidate, err)
	}
	if err := repo.LinkGoogleImportCandidate(ctx, userID, "list", "remote-2", other); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkGoogleImportCandidate(ctx, userID, "list", "remote-2", other); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("double link err=%v", err)
	}
	// 確認先プリセット（大文字小文字を区別せず一意）
	if _, err := repo.CreateConfirmationTarget(ctx, userID, " Owner "); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateConfirmationTarget(ctx, userID, "owner"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate preset err=%v", err)
	}
	targets, err := repo.ListConfirmationTargets(ctx, userID)
	if err != nil || len(targets) != 1 || targets[0].Name != "Owner" {
		t.Fatalf("targets=%+v err=%v", targets, err)
	}
	if err := repo.DeleteConfirmationTarget(ctx, userID, targets[0].ID); err != nil {
		t.Fatal(err)
	}
	// アクセス時自動同期: 未同期なら投入可能、ロック中は不可
	acquired, err := repo.AcquireDueGoogleSync(ctx, userID, nowUTC(), 2*60*1e9)
	if err != nil || !acquired {
		t.Fatalf("first due sync acquired=%v err=%v", acquired, err)
	}
	acquired, err = repo.AcquireDueGoogleSync(ctx, userID, nowUTC(), 2*60*1e9)
	if err != nil || acquired {
		t.Fatalf("locked due sync acquired=%v err=%v", acquired, err)
	}
	// 最終FAILEDの表示と手動再同期での再投入
	mustExec(`UPDATE google_task_syncs SET status='FAILED',last_error_message='quota' WHERE google_task_id='remote-2'`)
	status, err := repo.GoogleSyncStatus(ctx, userID)
	if err != nil || !status.Connected || len(status.Failed) != 1 || status.Failed[0].Message != "quota" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if err := repo.RequeueFailedGoogleTasks(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM google_sync_jobs WHERE task_id=$1 AND action='SYNC_TASK'`, other).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("requeued jobs=%d err=%v", jobs, err)
	}
	// 専用リスト喪失の表示と作り直し（紐付け破棄・未解除タスクの再作成投入）
	if err := repo.MarkGoogleTaskListMissing(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if status, err = repo.GoogleSyncStatus(ctx, userID); err != nil || !status.TaskListMissing {
		t.Fatalf("missing status=%+v err=%v", status, err)
	}
	if err := repo.ResetGoogleTaskList(ctx, userID); err != nil {
		t.Fatal(err)
	}
	var links int
	if err := database.QueryRow(`SELECT count(*) FROM google_task_syncs WHERE user_id=$1`, userID).Scan(&links); err != nil || links != 0 {
		t.Fatalf("links=%d err=%v", links, err)
	}
	// 連携解除済みのタスクは再作成しない
	if err := database.QueryRow(`SELECT count(*) FROM google_sync_jobs WHERE task_id=$1`, task).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("unlinked task recreated jobs=%d err=%v", jobs, err)
	}
	if status, err = repo.GoogleSyncStatus(ctx, userID); err != nil || status.TaskListMissing {
		t.Fatalf("reset status=%+v err=%v", status, err)
	}
}

// 現在時刻（UTC）
func nowUTC() time.Time { return time.Now().UTC() }
