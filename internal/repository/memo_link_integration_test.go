package repository

import (
	"context"
	"errors"
	"testing"

	"tasmane/internal/domain"
)

// メモリンクと連携設定の永続化（実PostgreSQL）
func TestPostgresMemoLinksAgainstFreshSchema(t *testing.T) {
	database := freshSchemaDatabase(t, "memo_link_test_")
	ctx := context.Background()
	const (
		userID = "01890f3e-4c00-7000-8000-000000000501"
		other  = "01890f3e-4c00-7000-8000-000000000502"
		genre  = "01890f3e-4c00-7000-8000-000000000503"
		task   = "01890f3e-4c00-7000-8000-000000000504"
	)
	for _, query := range []string{
		`INSERT INTO users (id,google_subject,email) VALUES ('` + userID + `','m-user','m@example.com'),('` + other + `','o-user','o@example.com')`,
		`INSERT INTO user_server_settings (user_id) VALUES ('` + userID + `') ON CONFLICT DO NOTHING`,
		`INSERT INTO genres (id,user_id,name) VALUES ('` + genre + `','` + userID + `','Work')`,
		`INSERT INTO tasks (id,user_id,genre_id,title,status) VALUES ('` + task + `','` + userID + `','` + genre + `','Task','NOT_STARTED')`,
	} {
		if _, err := database.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	repo := NewPostgres(database, &fixedIDs{values: []string{"01890f3e-4c00-7000-8000-000000000590", "01890f3e-4c00-7000-8000-000000000591", "01890f3e-4c00-7000-8000-000000000592", "01890f3e-4c00-7000-8000-000000000593", "01890f3e-4c00-7000-8000-000000000594", "01890f3e-4c00-7000-8000-000000000595", "01890f3e-4c00-7000-8000-000000000596", "01890f3e-4c00-7000-8000-000000000597", "01890f3e-4c00-7000-8000-000000000598", "01890f3e-4c00-7000-8000-000000000599"}})

	first, err := repo.UpsertMemoLink(ctx, userID, task, "https://memo.kokage-studio.com/memo/a", "初版")
	if err != nil || first.ID == "" {
		t.Fatalf("insert: %+v %v", first, err)
	}
	again, err := repo.UpsertMemoLink(ctx, userID, task, "https://memo.kokage-studio.com/memo/a", "改題")
	if err != nil || again.ID != first.ID || again.Title != "改題" {
		t.Fatalf("upsert must keep id and update title: %+v %v", again, err)
	}
	// メモアプリのメモIDに含まれる日付の保持
	dated, err := repo.UpsertMemoLink(ctx, userID, task, "https://memo.kokage-studio.com/memo/memo-20260520-%E3%83%86%E3%82%B9%E3%83%88-2", "テスト-2")
	if err != nil || dated.MemoDate == nil || dated.MemoDate.Format("2006-01-02") != "2026-05-20" {
		t.Fatalf("memo date: %+v %v", dated, err)
	}
	if err := repo.DeleteMemoLink(ctx, userID, task, dated.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertMemoLink(ctx, other, task, "https://memo.kokage-studio.com/memo/b", "x"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other user's task must be forbidden: %v", err)
	}
	links, err := repo.ListMemoLinks(ctx, userID, task)
	if err != nil || len(links) != 1 || links[0].Title != "改題" {
		t.Fatalf("list: %+v %v", links, err)
	}
	if err := repo.DeleteMemoLink(ctx, other, task, first.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other user cannot unlink: %v", err)
	}
	if err := repo.DeleteMemoLink(ctx, userID, task, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	settings, err := repo.GetUserSettings(ctx, userID)
	if err != nil || settings.MemoIntegrationEnabled {
		t.Fatalf("default settings: %+v %v", settings, err)
	}
	settings.MemoIntegrationEnabled = true
	if _, err := repo.UpdateUserSettings(ctx, userID, settings); err != nil {
		t.Fatal(err)
	}
	if saved, _ := repo.GetUserSettings(ctx, userID); !saved.MemoIntegrationEnabled {
		t.Fatalf("flag not persisted: %+v", saved)
	}
}
