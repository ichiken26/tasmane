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

// ジャンル移動と階層ガードの検証
func TestPostgresTaskRepositoryAgainstFreshSchemaGenreMoveAndHierarchyGuards(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "task_move_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(schema) {
		t.Fatal("unsafe generated schema")
	}
	// テストスキーマの作成
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

	const (
		userID   = "01890f3e-4c00-7000-8000-000000000201"
		oldGenre = "01890f3e-4c00-7000-8000-000000000202"
		newGenre = "01890f3e-4c00-7000-8000-000000000203"
		outside  = "01890f3e-4c00-7000-8000-000000000204"
		root     = "01890f3e-4c00-7000-8000-000000000205"
		child    = "01890f3e-4c00-7000-8000-000000000206"
	)
	// フィクスチャの挿入
	if _, err := database.Exec(`INSERT INTO users (id,google_subject,email) VALUES ($1,'move-user','move@example.com')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO genres (id,user_id,name) VALUES ($2,$1,'Old'),($3,$1,'New')`, userID, oldGenre, newGenre); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO tasks (id,user_id,genre_id,parent_task_id,title) VALUES
	    ($3,$1,$2,NULL,'Outside'),($4,$1,$2,$3,'Root'),($5,$1,$2,$4,'Child')`, userID, oldGenre, outside, root, child); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO task_dependencies (user_id,genre_id,task_id,depends_on_task_id) VALUES ($1,$2,$3,$4)`, userID, oldGenre, outside, child); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgres(database, nil)
	cycleTask, err := repo.Get(context.Background(), userID, outside)
	if err != nil {
		t.Fatal(err)
	}
	cycleTask.ParentTaskID = stringPointer(child)
	if _, err := repo.Update(context.Background(), domain.TaskUpdate{Task: cycleTask}, 1); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatalf("hierarchy cycle err=%v", err)
	}
	rootTask, err := repo.Get(context.Background(), userID, root)
	if err != nil {
		t.Fatal(err)
	}
	rootTask.GenreID = newGenre
	if _, err := repo.Update(context.Background(), domain.TaskUpdate{Task: rootTask}, 1); !errors.Is(err, domain.ErrImpactConfirmationRequired) {
		t.Fatalf("unconfirmed move err=%v", err)
	}
	graphTasks, dependencies, err := repo.LoadGraph(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := domain.PlanGenreMove(graphTasks, dependencies, root, newGenre)
	if err != nil {
		t.Fatal(err)
	}
	staleApproval := approved
	staleApproval.RemovedDependencies = nil
	if _, err := repo.Update(context.Background(), domain.TaskUpdate{Task: rootTask, ApprovedGenreMove: &staleApproval}, 1); !errors.Is(err, domain.ErrImpactPlanChanged) {
		t.Fatalf("changed lock-time impact accepted: %v", err)
	}
	updated, err := repo.Update(context.Background(), domain.TaskUpdate{Task: rootTask, ApprovedGenreMove: &approved}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GenreID != newGenre || updated.ParentTaskID != nil || updated.Version != 2 {
		t.Fatalf("updated root=%+v", updated)
	}
	var rootGenre, childGenre string
	var rootParent sql.NullString
	var childVersion, boundaryDependencies int
	// 移動結果のスキャン
	if err := database.QueryRow(`SELECT genre_id,parent_task_id FROM tasks WHERE id=$1`, root).Scan(&rootGenre, &rootParent); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT genre_id,version FROM tasks WHERE id=$1`, child).Scan(&childGenre, &childVersion); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM task_dependencies WHERE task_id=$1 AND depends_on_task_id=$2`, outside, child).Scan(&boundaryDependencies); err != nil {
		t.Fatal(err)
	}
	if rootGenre != newGenre || childGenre != newGenre || rootParent.Valid || childVersion != 2 || boundaryDependencies != 0 {
		t.Fatalf("rootGenre=%s childGenre=%s parent=%v childVersion=%d dependencies=%d", rootGenre, childGenre, rootParent, childVersion, boundaryDependencies)
	}

	// グラフロック下でのジャンル跨ぎ親子付け拒否
	outsideTask, err := repo.Get(context.Background(), userID, outside)
	if err != nil {
		t.Fatal(err)
	}
	outsideTask.ParentTaskID = stringPointer(child)
	if _, err := repo.Update(context.Background(), domain.TaskUpdate{Task: outsideTask}, 1); !errors.Is(err, domain.ErrGraphGenre) {
		t.Fatalf("cross-genre parent err=%v", err)
	}
}
