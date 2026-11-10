package repository

import (
	"context"
	"database/sql"
	"errors"
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

// 新鮮スキーマの統合テスト用DB
func freshSchemaDatabase(t *testing.T, prefix string) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := prefix + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(schema) {
		t.Fatal("unsafe generated schema")
	}
	if _, err := admin.ExecContext(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`) })
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema + ", public"
	database := stdlib.OpenDB(*config)
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	return database
}

// 葉進捗・削除後正規化・復元・付け替え・確認事項ガードの統合検証
func TestPostgresHierarchyLifecycleAgainstFreshSchema(t *testing.T) {
	database := freshSchemaDatabase(t, "hierarchy_test_")
	ctx := context.Background()
	const (
		userID = "01890f3e-4c00-7000-8000-000000000301"
		genre  = "01890f3e-4c00-7000-8000-000000000302"
		parent = "01890f3e-4c00-7000-8000-000000000303"
		child  = "01890f3e-4c00-7000-8000-000000000304"
		done   = "01890f3e-4c00-7000-8000-000000000305"
		loose  = "01890f3e-4c00-7000-8000-000000000306"
	)
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	mustExec(`INSERT INTO users (id,google_subject,email) VALUES ($1,'h-user','h@example.com')`, userID)
	mustExec(`INSERT INTO genres (id,user_id,name) VALUES ($1,$2,'Work')`, genre, userID)
	mustExec(`INSERT INTO tasks (id,user_id,genre_id,title,status) VALUES ($1,$3,$4,'Parent','IN_PROGRESS'),($2,$3,$4,'Child','IN_PROGRESS')`, parent, child, userID, genre)
	mustExec(`UPDATE tasks SET parent_task_id=$1 WHERE id=$2`, parent, child)
	mustExec(`INSERT INTO tasks (id,user_id,genre_id,title,status,leaf_progress) VALUES ($1,$3,$4,'Done','COMPLETED',100),($2,$3,$4,'Loose','NOT_STARTED',NULL)`, done, loose, userID, genre)

	ids := &fixedIDs{values: []string{
		"01890f3e-4c00-7000-8000-000000000390", "01890f3e-4c00-7000-8000-000000000391",
		"01890f3e-4c00-7000-8000-000000000392", "01890f3e-4c00-7000-8000-000000000393",
		"01890f3e-4c00-7000-8000-000000000394", "01890f3e-4c00-7000-8000-000000000395",
		"01890f3e-4c00-7000-8000-000000000396", "01890f3e-4c00-7000-8000-000000000397",
	}}
	repo := NewPostgres(database, ids)

	// 葉進捗の更新と履歴
	updated, err := repo.UpdateLeafProgress(ctx, userID, child, 1, 40, "WEB")
	if err != nil || updated.Progress == nil || *updated.Progress != 40 || updated.Version != 2 {
		t.Fatalf("leaf progress updated=%+v err=%v", updated, err)
	}
	if _, err := repo.UpdateLeafProgress(ctx, userID, parent, 1, 40, "WEB"); !errors.Is(err, domain.ErrInvalidProgress) {
		t.Fatalf("parent progress must be rejected: %v", err)
	}
	var histories int
	if err := database.QueryRow(`SELECT count(*) FROM task_progress_histories WHERE task_id=$1 AND to_progress=40`, child).Scan(&histories); err != nil || histories != 1 {
		t.Fatalf("progress history=%d err=%v", histories, err)
	}

	// 完了済みタスクへの未確認確認事項は禁止
	if _, err := repo.CreateConfirmation(ctx, userID, done, "確認", "相手", nil); !errors.Is(err, domain.ErrConfirmationOnCompletedTask) {
		t.Fatalf("confirmation on completed task err=%v", err)
	}
	if _, err := repo.CreateConfirmation(ctx, userID, child, "確認", "相手", nil); err != nil {
		t.Fatalf("confirmation on active task err=%v", err)
	}
	batch, err := repo.ListConfirmationsForTasks(ctx, userID, []string{child, parent})
	if err != nil || len(batch[child]) != 1 || len(batch[parent]) != 0 {
		t.Fatalf("batch confirmations=%+v err=%v", batch, err)
	}

	// 付け替え: 完了済み親は禁止、葉は承認必須
	looseTask, err := repo.Get(ctx, userID, loose)
	if err != nil {
		t.Fatal(err)
	}
	looseTask.ParentTaskID = stringPointer(done)
	if _, err := repo.Update(ctx, domain.TaskUpdate{Task: looseTask}, looseTask.Version); !errors.Is(err, domain.ErrCompletedParent) {
		t.Fatalf("completed parent err=%v", err)
	}

	// 期限切れ検索は未完了のみ
	past := time.Now().UTC().Add(-time.Hour)
	mustExec(`UPDATE tasks SET end_at=$1 WHERE id IN ($2,$3)`, past, done, loose)
	overdue, err := repo.Search(ctx, userID, domain.TaskSearch{Deadline: "overdue"})
	if err != nil || len(overdue) != 1 || overdue[0].ID != loose {
		t.Fatalf("overdue=%+v err=%v", overdue, err)
	}

	// 子の削除で親は葉へ戻り IN_PROGRESS→0 に正規化
	if _, err := repo.DeleteTaskApproved(ctx, userID, child, 2, []string{child}); err != nil {
		t.Fatalf("approved delete err=%v", err)
	}
	var progress sql.NullInt64
	if err := database.QueryRow(`SELECT leaf_progress FROM tasks WHERE id=$1`, parent).Scan(&progress); err != nil || !progress.Valid || progress.Int64 != 0 {
		t.Fatalf("normalized parent progress=%+v err=%v", progress, err)
	}
	deleted, err := repo.ListDeletedTasks(ctx, userID)
	if err != nil || len(deleted) != 1 || deleted[0].ID != child {
		t.Fatalf("deleted=%+v err=%v", deleted, err)
	}

	// 復元で親は再び親になり leaf_progress は NULL
	restored, err := repo.RestoreTask(ctx, userID, child)
	if err != nil || len(restored) != 1 {
		t.Fatalf("restored=%v err=%v", restored, err)
	}
	if err := database.QueryRow(`SELECT leaf_progress FROM tasks WHERE id=$1`, parent).Scan(&progress); err != nil || progress.Valid {
		t.Fatalf("restored parent progress=%+v err=%v", progress, err)
	}
	var childDeleted bool
	if err := database.QueryRow(`SELECT deleted_at IS NOT NULL FROM tasks WHERE id=$1`, child).Scan(&childDeleted); err != nil || childDeleted {
		t.Fatalf("child restore deleted=%v err=%v", childDeleted, err)
	}
	// 承認後に削除集合が変わった場合は拒否
	if _, err := repo.DeleteTaskApproved(ctx, userID, parent, 3, []string{parent}); !errors.Is(err, domain.ErrImpactPlanChanged) {
		t.Fatalf("changed plan err=%v", err)
	}
}
