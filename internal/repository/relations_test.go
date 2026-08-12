package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"tasmane/internal/domain"
)

func TestPostgresSetParentRevalidatesGraphInsideLockedTransaction(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	expectGraphRows(mock, []driver.Value{"a", "user-1", "genre-1", "b"}, []driver.Value{"b", "user-1", "genre-1", ""}, nil)
	mock.ExpectRollback()
	if err := repository.SetParent(context.Background(), "user-1", "b", "a"); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresAddDependencyRevalidatesAndPersistsInsideLock(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	expectGraphRows(mock, []driver.Value{"a", "user-1", "genre-1", ""}, []driver.Value{"b", "user-1", "genre-1", ""}, nil)
	mock.ExpectQuery("SELECT version FROM tasks").WithArgs("a", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
	mock.ExpectExec("INSERT INTO task_dependencies").
		WithArgs("user-1", "genre-1", "a", "b").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE tasks SET version").WithArgs("a", "user-1", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repository.AddDependency(context.Background(), "user-1", domain.Dependency{TaskID: "a", DependsOnTaskID: "b"}, 1); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectGraphRows(mock sqlmock.Sqlmock, first, second []driver.Value, dependencies [][]driver.Value) {
	mock.ExpectQuery("SELECT id, user_id, genre_id").WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "genre_id", "parent_id"}).AddRow(first...).AddRow(second...))
	rows := sqlmock.NewRows([]string{"task_id", "depends_on_task_id"})
	for _, dependency := range dependencies {
		rows.AddRow(dependency...)
	}
	mock.ExpectQuery("SELECT dependency.task_id").WithArgs("user-1").WillReturnRows(rows)
}
