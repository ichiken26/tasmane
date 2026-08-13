package repository

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"tasmane/db"
	"tasmane/internal/domain"
)

func TestPostgresStatusRollbackPropagatesWithHistoriesInOneTransaction(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "status_transition_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(schema) {
		t.Fatal("unsafe schema")
	}
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema + ", public"
	database := stdlib.OpenDB(*config)
	t.Cleanup(func() { database.Close() })
	if err = db.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}

	const (
		userID   = "01890f3e-4c00-7000-8000-000000000201"
		genreID  = "01890f3e-4c00-7000-8000-000000000202"
		parentID = "01890f3e-4c00-7000-8000-000000000203"
		leafID   = "01890f3e-4c00-7000-8000-000000000204"
	)
	if _, err = database.Exec(`INSERT INTO users(id,google_subject,email) VALUES($1,'status-user','status@example.com')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO genres(id,user_id,name) VALUES($1,$2,'Work')`, genreID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`INSERT INTO tasks(id,user_id,genre_id,parent_task_id,title,status,leaf_progress,version) VALUES
		($1,$3,$4,NULL,'Parent','COMPLETED',NULL,2),
		($2,$3,$4,$1,'Leaf','COMPLETED',100,4)`, parentID, leafID, userID, genreID); err != nil {
		t.Fatal(err)
	}
	ids := []string{
		"01890f3e-4c00-7000-8000-000000000211", "01890f3e-4c00-7000-8000-000000000212",
		"01890f3e-4c00-7000-8000-000000000213", "01890f3e-4c00-7000-8000-000000000214",
		"01890f3e-4c00-7000-8000-000000000215", "01890f3e-4c00-7000-8000-000000000216",
	}
	repository := NewPostgres(database, &fixedIDs{values: ids})
	plan, err := repository.PreviewStatusTransition(context.Background(), userID, leafID, 4, domain.InProgress)
	if err != nil || len(plan.Changes) != 2 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	got, err := repository.ApplyStatusTransition(context.Background(), userID, leafID, 4, domain.InProgress, "WEB", plan)
	if err != nil || got.Status != domain.InProgress || got.Progress == nil || *got.Progress != 95 || got.Version != 5 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	var parentStatus string
	var parentVersion, statusHistories, progressHistories int
	if err = database.QueryRow(`SELECT status,version FROM tasks WHERE id=$1`, parentID).Scan(&parentStatus, &parentVersion); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRow(`SELECT count(*) FROM task_status_histories WHERE task_id IN ($1,$2)`, parentID, leafID).Scan(&statusHistories); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRow(`SELECT count(*) FROM task_progress_histories WHERE task_id=$1`, leafID).Scan(&progressHistories); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "IN_PROGRESS" || parentVersion != 3 || statusHistories != 2 || progressHistories != 1 {
		t.Fatalf("parent=%s/%d status histories=%d progress histories=%d", parentStatus, parentVersion, statusHistories, progressHistories)
	}
}
