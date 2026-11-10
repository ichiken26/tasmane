package repository

import (
	"context"
	"testing"
)

// メモリンクの追加・解除でGoogle Tasksの詳細を更新する同期ジョブが積まれる
func TestPostgresMemoLinkChangesEnqueueGoogleSync(t *testing.T) {
	database := freshSchemaDatabase(t, "memo_link_sync_test_")
	ctx := context.Background()
	const (
		userID = "01890f3e-4c00-7000-8000-000000000601"
		genre  = "01890f3e-4c00-7000-8000-000000000602"
		task   = "01890f3e-4c00-7000-8000-000000000603"
	)
	for _, query := range []string{
		`INSERT INTO users (id,google_subject,email) VALUES ('` + userID + `','s-user','s@example.com')`,
		`INSERT INTO genres (id,user_id,name) VALUES ('` + genre + `','` + userID + `','Work')`,
		`INSERT INTO tasks (id,user_id,genre_id,title,status) VALUES ('` + task + `','` + userID + `','` + genre + `','Task','NOT_STARTED')`,
		`INSERT INTO google_integrations (id,user_id,google_subject,encrypted_refresh_token) VALUES ('01890f3e-4c00-7000-8000-000000000610','` + userID + `','s-user','x')`,
	} {
		if _, err := database.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	repo := NewPostgres(database, &fixedIDs{values: []string{
		"01890f3e-4c00-7000-8000-000000000690", "01890f3e-4c00-7000-8000-000000000691",
		"01890f3e-4c00-7000-8000-000000000692", "01890f3e-4c00-7000-8000-000000000693",
	}})
	pending := func() int {
		t.Helper()
		var count int
		if err := database.QueryRow(`SELECT count(*) FROM google_sync_jobs WHERE task_id=$1 AND action='SYNC_TASK' AND claimed_at IS NULL`, task).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	link, err := repo.UpsertMemoLink(ctx, userID, task, "https://memo.kokage-studio.com/memo/a", "議事録")
	if err != nil {
		t.Fatal(err)
	}
	if got := pending(); got != 1 {
		t.Fatalf("after link: pending=%d", got)
	}
	// 同期中（クレーム済み）のジョブがあっても、後から加えた変更のジョブは別に積む
	if _, err := database.Exec(`UPDATE google_sync_jobs SET claimed_at=now(), claimed_by='w' WHERE task_id=$1`, task); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteMemoLink(ctx, userID, task, link.ID); err != nil {
		t.Fatal(err)
	}
	if got := pending(); got != 1 {
		t.Fatalf("after unlink while syncing: pending=%d", got)
	}
}
