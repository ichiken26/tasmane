package repository

import (
	"context"
	"testing"

	"tasmane/internal/domain"
)

// タスク同期状態の更新（enum列とテキスト比較で同一パラメータを使うSQLの型推論）
func TestPostgresGoogleTaskSyncStatusTransitions(t *testing.T) {
	database := freshSchemaDatabase(t, "google_sync_status_test_")
	ctx := context.Background()
	const (
		userID = "01890f3e-4c00-7000-8000-000000000501"
		genre  = "01890f3e-4c00-7000-8000-000000000502"
		task   = "01890f3e-4c00-7000-8000-000000000503"
	)
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	mustExec(`INSERT INTO users (id,google_subject,email) VALUES ($1,'g-sync','s@example.com')`, userID)
	mustExec(`INSERT INTO genres (id,user_id,name) VALUES ($1,$2,'Work')`, genre, userID)
	mustExec(`INSERT INTO tasks (id,user_id,genre_id,title) VALUES ($1,$2,$3,'Due task')`, task, userID, genre)
	mustExec(`INSERT INTO google_task_syncs (id,user_id,task_id,google_task_list_id,google_task_id,status) VALUES ('01890f3e-4c00-7000-8000-000000000511',$1,$2,'list','remote-1','PENDING')`, userID, task)
	repo := NewPostgres(database, nil)
	job := domain.GoogleJob{UserID: userID, TaskID: task}

	status := func() (string, bool) {
		t.Helper()
		var value string
		var synced bool
		if err := database.QueryRow(`SELECT status::text, last_synced_at IS NOT NULL FROM google_task_syncs WHERE task_id=$1`, task).Scan(&value, &synced); err != nil {
			t.Fatal(err)
		}
		return value, synced
	}
	if err := repo.MarkSyncing(ctx, job); err != nil {
		t.Fatalf("MarkSyncing: %v", err)
	}
	if got, synced := status(); got != "SYNCING" || synced {
		t.Fatalf("status=%s synced=%v", got, synced)
	}
	if err := repo.MarkSynced(ctx, job); err != nil {
		t.Fatalf("MarkSynced: %v", err)
	}
	if got, synced := status(); got != "SYNCED" || !synced {
		t.Fatalf("status=%s synced=%v", got, synced)
	}
	if err := repo.MarkFailed(ctx, job, "boom"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if got, _ := status(); got != "FAILED" {
		t.Fatalf("status=%s", got)
	}
}
