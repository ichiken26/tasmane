package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// プロセス間 updated_at 競合の記録
func TestUpdatedAtProcessWorker(t *testing.T) {
	databaseURL := os.Getenv("TASMANE_PROCESS_DATABASE_URL")
	schema := os.Getenv("TASMANE_PROCESS_SCHEMA")
	worker := os.Getenv("TASMANE_PROCESS_WORKER")
	if databaseURL == "" || schema == "" || worker == "" {
		t.Skip("process worker environment is not set")
	}
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(schema) {
		t.Fatal("unsafe process schema")
	}
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema + ", public"
	database := stdlib.OpenDB(*config)
	defer database.Close()
	if _, err := database.Exec(`
		WITH changed AS (
			UPDATE users SET display_name=$1 WHERE google_subject='one' RETURNING updated_at
		)
		INSERT INTO process_update_results (worker, updated_at)
		SELECT $1, updated_at FROM changed`, worker); err != nil {
		t.Fatal(err)
	}
}

const advisoryLockSQL = `SELECT pg_advisory_xact_lock($1)`

// 未適用マイグレーションのトランザクション適用順序の検証
func TestMigrateAppliesPendingMigrationTransactionally(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).
		WithArgs("0001_init.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS pg_trgm").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).
		WithArgs("0001_init.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).
		WithArgs("0002_lifecycle_policies.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("CREATE OR REPLACE FUNCTION set_updated_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).
		WithArgs("0002_lifecycle_policies.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).
		WithArgs("0003_uuid_v7_variant.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("DO").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).
		WithArgs("0003_uuid_v7_variant.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0004_google_import_candidates.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("ALTER TABLE google_task_syncs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0004_google_import_candidates.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0005_google_sync_jobs.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("CREATE TABLE google_sync_jobs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0005_google_sync_jobs.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0006_advice_source_dedup.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("CREATE UNIQUE INDEX uq_ai_advices_source").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0006_advice_source_dedup.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0007_google_job_leases.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("ALTER TABLE google_sync_jobs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0007_google_job_leases.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0008_google_conflict_resolution.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("ALTER TABLE google_task_syncs ADD COLUMN conflict_remote_snapshot").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0008_google_conflict_resolution.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0009_google_task_list_recovery.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("ALTER TABLE google_integrations ADD COLUMN task_list_missing_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0009_google_task_list_recovery.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0010_memo_links.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("ALTER TABLE user_server_settings ADD COLUMN memo_integration_enabled").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0010_memo_links.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0011_memo_link_date.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("ALTER TABLE task_memo_links ADD COLUMN memo_date").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO schema_migrations (version) VALUES ($1)`)).WithArgs("0011_memo_link_date.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 適用済みマイグレーションのスキップ検証
func TestMigrateSkipsRecordedMigration(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).
		WithArgs("0001_init.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).
		WithArgs("0002_lifecycle_policies.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).
		WithArgs("0003_uuid_v7_variant.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0004_google_import_candidates.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0005_google_sync_jobs.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0006_advice_source_dedup.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0007_google_job_leases.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0008_google_conflict_resolution.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0009_google_task_list_recovery.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0010_memo_links.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`)).WithArgs("0011_memo_link_date.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// nil DB 拒否の検証
func TestMigrateRejectsNilDatabase(t *testing.T) {
	if err := Migrate(context.Background(), nil); err == nil || err.Error() != "database is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// 履歴表作成失敗の伝播検証
func TestMigrateReportsHistoryCreationFailure(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnError(errors.New("denied"))
	err = Migrate(context.Background(), database)
	if err == nil || !strings.Contains(err.Error(), "create migration history: denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// コミット前失敗時のロールバック検証
func TestApplyRollsBackEveryPreCommitFailure(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(sqlmock.Sqlmock)
		want    string
	}{
		{"begin", func(mock sqlmock.Sqlmock) { mock.ExpectBegin().WillReturnError(errors.New("boom")) }, "begin migration"},
		{"lock", func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnError(errors.New("boom"))
			mock.ExpectRollback()
		}, "lock migration"},
		{"check", func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT EXISTS").WithArgs("0001_init.sql").WillReturnError(errors.New("boom"))
			mock.ExpectRollback()
		}, "check migration"},
		{"execute", func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT EXISTS").WithArgs("0001_init.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectExec("CREATE EXTENSION").WillReturnError(errors.New("boom"))
			mock.ExpectRollback()
		}, "apply migration"},
		{"record", func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT EXISTS").WithArgs("0001_init.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectExec("CREATE EXTENSION").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("0001_init.sql").WillReturnError(errors.New("boom"))
			mock.ExpectRollback()
		}, "record migration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			tt.arrange(mock)
			err = apply(context.Background(), database, "0001_init.sql")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// コミット失敗の伝播検証
func TestApplyReportsCommitFailure(t *testing.T) {
	for _, alreadyApplied := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "already applied"}[alreadyApplied], func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(advisoryLockSQL)).WithArgs(migrationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT EXISTS").WithArgs("0001_init.sql").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(alreadyApplied))
			if !alreadyApplied {
				mock.ExpectExec("CREATE EXTENSION").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("INSERT INTO schema_migrations").WithArgs("0001_init.sql").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectCommit().WillReturnError(errors.New("boom"))
			err = apply(context.Background(), database, "0001_init.sql")
			if err == nil || !strings.Contains(err.Error(), "commit migration 0001_init.sql: boom") {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 新規 Postgres への適用とスキーマ制約の検証
func TestMigrationAppliesToFreshPostgresAndEnforcesSchema(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "migration_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
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
	defer database.Close()
	// マイグレーションの冪等適用
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("third idempotent run: %v", err)
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != 11 {
		t.Fatalf("migration history count=%d err=%v", count, err)
	}
	// 一意制約と UUIDv7 variant の拒否
	if _, err := database.Exec(`INSERT INTO users (id, google_subject, email) VALUES ('01890f3e-4c00-7000-8000-000000000001','one','one@example.com')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO genres (id,user_id,name) VALUES ('01890f3e-4c00-7000-8000-000000000002','01890f3e-4c00-7000-8000-000000000001',' Work ')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO genres (id,user_id,name) VALUES ('01890f3e-4c00-7000-8000-000000000003','01890f3e-4c00-7000-8000-000000000001','work')`); err == nil {
		t.Fatal("case-insensitive genre uniqueness was not enforced")
	}
	if _, err := database.Exec(`INSERT INTO task_types (id,user_id,name) VALUES ('00000000-0000-4000-8000-000000000004','01890f3e-4c00-7000-8000-000000000001','invalid')`); err == nil {
		t.Fatal("UUIDv4 was accepted")
	}
	if _, err := database.Exec(`INSERT INTO task_types (id,user_id,name) VALUES ('01890f3e-4c00-7000-7000-000000000004','01890f3e-4c00-7000-8000-000000000001','invalid variant')`); err == nil {
		t.Fatal("UUIDv7 with a non-RFC 9562 variant was accepted")
	}
	var before, after time.Time
	if err := database.QueryRow(`SELECT updated_at FROM users WHERE google_subject='one'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := database.QueryRow(`UPDATE users SET display_name='updated' WHERE google_subject='one' RETURNING updated_at`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.After(before) {
		t.Fatalf("updated_at did not advance: %s -> %s", before, after)
	}
	if _, err := database.Exec(`CREATE TABLE process_update_results (worker text PRIMARY KEY, updated_at timestamptz NOT NULL UNIQUE)`); err != nil {
		t.Fatal(err)
	}
	// プロセス間 updated_at の一意性検証
	const processes = 8
	processErrors := make(chan error, processes)
	var processGroup sync.WaitGroup
	for i := 0; i < processes; i++ {
		processGroup.Add(1)
		go func(worker int) {
			defer processGroup.Done()
			command := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestUpdatedAtProcessWorker$")
			command.Env = append(os.Environ(),
				"TASMANE_PROCESS_DATABASE_URL="+databaseURL,
				"TASMANE_PROCESS_SCHEMA="+schema,
				"TASMANE_PROCESS_WORKER="+strconv.Itoa(worker),
			)
			if output, err := command.CombinedOutput(); err != nil {
				processErrors <- fmt.Errorf("worker %d: %w: %s", worker, err, output)
			}
		}(i)
	}
	processGroup.Wait()
	close(processErrors)
	for err := range processErrors {
		t.Fatal(err)
	}
	var processCount, distinctTimestampCount int
	if err := database.QueryRow(`SELECT count(*), count(DISTINCT updated_at) FROM process_update_results`).Scan(&processCount, &distinctTimestampCount); err != nil {
		t.Fatal(err)
	}
	if processCount != processes || distinctTimestampCount != processes {
		t.Fatalf("process updates=%d distinct timestamps=%d", processCount, distinctTimestampCount)
	}
	// 並行更新時の updated_at 衝突検出
	const updates = 16
	timestamps := make(chan time.Time, updates)
	errors := make(chan error, updates)
	for i := 0; i < updates; i++ {
		go func(value int) {
			var timestamp time.Time
			err := database.QueryRow(`UPDATE users SET display_name=$1 WHERE google_subject='one' RETURNING updated_at`, strconv.Itoa(value)).Scan(&timestamp)
			if err != nil {
				errors <- err
				return
			}
			timestamps <- timestamp
		}(i)
	}
	seenTimestamps := map[time.Time]bool{}
	for i := 0; i < updates; i++ {
		select {
		case err := <-errors:
			t.Fatal(err)
		case timestamp := <-timestamps:
			if seenTimestamps[timestamp] {
				t.Fatalf("duplicate concurrent updated_at: %s", timestamp)
			}
			seenTimestamps[timestamp] = true
		}
	}
}
