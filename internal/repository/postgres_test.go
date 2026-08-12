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

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"tasmane/db"
	"tasmane/internal/application"
	"tasmane/internal/domain"
)

type fixedIDs struct{ values []string }

func (f *fixedIDs) New() (string, error) {
	if len(f.values) == 0 {
		return "", errors.New("no test id")
	}
	value := f.values[0]
	f.values = f.values[1:]
	return value, nil
}

func TestPostgresTaskGetDerivesParentAndCompletionGuards(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	mock.ExpectQuery("WITH RECURSIVE descendants").WithArgs("task-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "leaf_progress", "version", "has_children",
			"incomplete_descendants", "unresolved_confirmations",
		}).AddRow("task-1", "user-1", "IN_PROGRESS", nil, 4, true, 2, 1))

	got, err := NewPostgres(database, &fixedIDs{}).Get(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "task-1" || !got.HasChildren || got.Progress != nil || got.Version != 4 || got.IncompleteDescendants != 2 || got.UnresolvedConfirmations != 1 {
		t.Fatalf("unexpected task: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTaskUpdateIsAtomicAndClearsParentProgress(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{values: []string{"status-history"}})

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(taskForUpdateSQL)).WithArgs("task-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"status", "leaf_progress", "version", "has_children"}).AddRow("NOT_STARTED", nil, 1, true))
	mock.ExpectQuery("UPDATE tasks").WithArgs(domain.InProgress, nil, "task-1", "user-1", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(2))
	mock.ExpectExec("INSERT INTO task_status_histories").
		WithArgs("status-history", "user-1", "task-1", domain.NotStarted, domain.InProgress, "user-1", "WEB").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	progress := 95
	got, err := repository.UpdateStatus(context.Background(), domain.Task{
		ID: "task-1", UserID: "user-1", Status: domain.InProgress,
		Progress: &progress, HasChildren: true,
	}, 1, "WEB")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || got.Progress != nil || !got.HasChildren {
		t.Fatalf("unexpected persisted parent: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTaskUpdateRollsBackHistoryFailure(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{values: []string{"status-history", "progress-history"}})

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(taskForUpdateSQL)).WithArgs("task-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"status", "leaf_progress", "version", "has_children"}).AddRow("NOT_STARTED", nil, 1, false))
	mock.ExpectQuery("UPDATE tasks").WithArgs(domain.InProgress, 0, "task-1", "user-1", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(2))
	mock.ExpectExec("INSERT INTO task_status_histories").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO task_progress_histories").WillReturnError(errors.New("history unavailable"))
	mock.ExpectRollback()

	progress := 0
	_, err = repository.UpdateStatus(context.Background(), domain.Task{
		ID: "task-1", UserID: "user-1", Status: domain.InProgress, Progress: &progress,
	}, 1, "MCP")
	if err == nil || !regexp.MustCompile("history unavailable").MatchString(err.Error()) {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTaskUpdateReportsVersionConflict(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(taskForUpdateSQL)).WithArgs("task-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"status", "leaf_progress", "version", "has_children"}).AddRow("IN_PROGRESS", 20, 3, false))
	mock.ExpectRollback()
	_, err = repository.UpdateStatus(context.Background(), domain.Task{ID: "task-1", UserID: "user-1"}, 2, "WEB")
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTaskGetHidesMissingAndDeletedRows(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	mock.ExpectQuery("WITH RECURSIVE descendants").WithArgs("task-1", "user-1").WillReturnError(sql.ErrNoRows)
	_, err = NewPostgres(database, &fixedIDs{}).Get(context.Background(), "user-1", "task-1")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresTaskRepositoryAgainstFreshSchema(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "task_repository_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
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

	const (
		userID         = "01890f3e-4c00-7000-8000-000000000101"
		otherUserID    = "01890f3e-4c00-7000-8000-000000000102"
		genreID        = "01890f3e-4c00-7000-8000-000000000103"
		parentID       = "01890f3e-4c00-7000-8000-000000000104"
		childID        = "01890f3e-4c00-7000-8000-000000000105"
		confirmationID = "01890f3e-4c00-7000-8000-000000000106"
		historyID      = "01890f3e-4c00-7000-8000-000000000107"
		secondGenreID  = "01890f3e-4c00-7000-8000-000000000108"
		duplicateID    = "01890f3e-4c00-7000-8000-000000000109"
		taskTypeID     = "01890f3e-4c00-7000-8000-000000000110"
	)
	if _, err := database.Exec(`INSERT INTO users (id, google_subject, email) VALUES
		($1, 'repo-user', 'repo@example.com'), ($2, 'other-user', 'other@example.com')`, userID, otherUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO genres (id, user_id, name) VALUES ($1, $2, 'Work')`, genreID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO tasks (id, user_id, genre_id, title) VALUES
		($1, $2, $3, 'Parent'), ($4, $2, $3, 'Child')`, parentID, userID, genreID, childID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE tasks SET parent_task_id=$1 WHERE id=$2`, parentID, childID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO task_confirmations
		(id, user_id, task_id, content, target_name) VALUES ($1, $2, $3, 'Confirm', 'Owner')`, confirmationID, userID, parentID); err != nil {
		t.Fatal(err)
	}

	repository := NewPostgres(database, &fixedIDs{values: []string{historyID}})
	loaded, err := repository.Get(context.Background(), userID, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasChildren || loaded.IncompleteDescendants != 1 || loaded.UnresolvedConfirmations != 1 {
		t.Fatalf("derived task state: %+v", loaded)
	}
	if _, err := repository.Get(context.Background(), otherUserID, parentID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-user read: %v", err)
	}
	updated, err := (application.Tasks{Repo: repository}).Transition(context.Background(), userID, parentID, 1, domain.InProgress, "WEB")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Progress != nil || updated.Version != 2 {
		t.Fatalf("updated parent: %+v", updated)
	}
	var status string
	var progress sql.NullInt64
	var version, historyCount int
	if err := database.QueryRow(`SELECT status, leaf_progress, version FROM tasks WHERE id=$1`, parentID).Scan(&status, &progress, &version); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM task_status_histories WHERE task_id=$1`, parentID).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if status != "IN_PROGRESS" || progress.Valid || version != 2 || historyCount != 1 {
		t.Fatalf("persisted status=%s progress=%+v version=%d history=%d", status, progress, version, historyCount)
	}
	confirmation, err := (application.Confirmations{Repo: repository}).Transition(
		context.Background(), userID, confirmationID, domain.ConfirmationChecking, time.Now().UTC(),
	)
	if err != nil || confirmation.Status != domain.ConfirmationChecking || confirmation.ExpectedStatus != domain.ConfirmationUnconfirmed {
		t.Fatalf("confirmation transition: %+v %v", confirmation, err)
	}
	confirmations, err := repository.ListConfirmations(context.Background(), userID, parentID)
	if err != nil || len(confirmations) != 1 || confirmations[0].Status != domain.ConfirmationChecking {
		t.Fatalf("confirmation list: %+v %v", confirmations, err)
	}
	stale := confirmation
	stale.ExpectedStatus = domain.ConfirmationUnconfirmed
	if err := repository.UpdateConfirmation(context.Background(), userID, confirmationID, stale); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("stale confirmation transition: %v", err)
	}

	catalog := NewPostgres(database, &fixedIDs{values: []string{secondGenreID, duplicateID, taskTypeID}})
	genre, err := catalog.CreateGenre(context.Background(), userID, " Personal ")
	if err != nil || genre.Name != "Personal" {
		t.Fatalf("create genre: %+v %v", genre, err)
	}
	if _, err := catalog.CreateGenre(context.Background(), userID, "personal"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("case-insensitive duplicate: %v", err)
	}
	genres, err := catalog.ListGenres(context.Background(), userID)
	if err != nil || len(genres) != 2 {
		t.Fatalf("list genres: %+v %v", genres, err)
	}
	if err := catalog.DeleteGenre(context.Background(), userID, secondGenreID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetGenre(context.Background(), userID, secondGenreID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("soft-deleted genre remained visible: %v", err)
	}
	taskType, err := catalog.CreateTaskType(context.Background(), userID, "Build")
	if err != nil || taskType.ID != taskTypeID {
		t.Fatalf("create task type: %+v %v", taskType, err)
	}
	if err := catalog.AttachGenreTaskType(context.Background(), userID, genreID, taskTypeID); err != nil {
		t.Fatal(err)
	}
	if err := catalog.AttachGenreTaskType(context.Background(), otherUserID, genreID, taskTypeID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-user attach: %v", err)
	}
	if _, err := database.Exec(`UPDATE tasks SET task_type_id=$1 WHERE id=$2`, taskTypeID, childID); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DetachGenreTaskType(context.Background(), userID, genreID, taskTypeID); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("detach used task type: %v", err)
	}
	if err := catalog.DeleteTaskType(context.Background(), userID, taskTypeID); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("delete used task type: %v", err)
	}
	relations := application.Relations{Repo: repository}
	if err := relations.AddDependency(context.Background(), userID, parentID, childID); err != nil {
		t.Fatalf("add dependency: %v", err)
	}
	dependencies, err := repository.ListTaskDependencies(context.Background(), userID, parentID)
	if err != nil || len(dependencies) != 1 || dependencies[0].DependsOnTaskID != childID {
		t.Fatalf("list dependencies: %+v %v", dependencies, err)
	}
	if err := relations.AddDependency(context.Background(), userID, childID, parentID); !errors.Is(err, domain.ErrDependencyCycle) {
		t.Fatalf("dependency cycle accepted: %v", err)
	}
	if err := relations.AssignParent(context.Background(), userID, parentID, childID); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatalf("hierarchy cycle accepted: %v", err)
	}
	if err := relations.AddDependency(context.Background(), otherUserID, parentID, childID); !errors.Is(err, domain.ErrGraphTaskNotFound) {
		t.Fatalf("cross-user graph access: %v", err)
	}
}
