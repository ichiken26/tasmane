package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"tasmane/internal/domain"
)

// ジャンル名トリムと所有者スコープの検証
func TestPostgresGenreCreateTrimsNameAndScopesOwner(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{values: []string{"genre-1"}})
	mock.ExpectQuery("INSERT INTO genres").WithArgs("genre-1", "user-1", "Work").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name"}).AddRow("genre-1", "user-1", "Work"))
	got, err := repository.CreateGenre(context.Background(), "user-1", "  Work  ")
	if err != nil || got.Name != "Work" || got.UserID != "user-1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 利用中ジャンル削除のロールバック
func TestPostgresGenreDeleteRejectsInUseAndRollsBack(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(catalogLockSQL("genres"))).WithArgs("genre-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("genre-1"))
	mock.ExpectQuery("SELECT EXISTS.*FROM tasks").WithArgs("genre-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()
	if err := repository.DeleteGenre(context.Background(), "user-1", "genre-1"); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 未使用ジャンルの論理削除
func TestPostgresGenreDeleteSoftDeletesUnusedGenre(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(catalogLockSQL("genres"))).WithArgs("genre-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("genre-1"))
	mock.ExpectQuery("SELECT EXISTS.*FROM tasks").WithArgs("genre-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("UPDATE genres SET deleted_at").WithArgs("genre-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repository.DeleteGenre(context.Background(), "user-1", "genre-1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 空ジャンル名の拒否
func TestPostgresGenreRejectsBlankName(t *testing.T) {
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if _, err := NewPostgres(database, &fixedIDs{}).CreateGenre(context.Background(), "user-1", " \n "); !errors.Is(err, domain.ErrNameRequired) {
		t.Fatalf("got %v", err)
	}
}

// タスク種別作成とテナントスコープ紐付け
func TestPostgresTaskTypeCreateAndAttachAreTenantScoped(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{values: []string{"type-1"}})
	mock.ExpectQuery("INSERT INTO task_types").WithArgs("type-1", "user-1", "Build").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name"}).AddRow("type-1", "user-1", "Build"))
	created, err := repository.CreateTaskType(context.Background(), "user-1", " Build ")
	if err != nil || created.ID != "type-1" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	mock.ExpectExec("INSERT INTO genre_task_types").WithArgs("user-1", "genre-1", "type-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repository.AttachGenreTaskType(context.Background(), "user-1", "genre-1", "type-1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 利用中タスク種別の紐付け解除拒否
func TestPostgresTaskTypeDetachRejectsUseByTask(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT genre_id FROM genre_task_types").WithArgs("user-1", "genre-1", "type-1").
		WillReturnRows(sqlmock.NewRows([]string{"genre_id"}).AddRow("genre-1"))
	mock.ExpectQuery("SELECT EXISTS.*FROM tasks").WithArgs("user-1", "genre-1", "type-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()
	if err := repository.DetachGenreTaskType(context.Background(), "user-1", "genre-1", "type-1"); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 利用中タスク種別の削除拒否
func TestPostgresTaskTypeDeleteRejectsUseByTask(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(catalogLockSQL("task_types"))).WithArgs("type-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("type-1"))
	mock.ExpectQuery("SELECT EXISTS.*FROM tasks").WithArgs("type-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()
	if err := repository.DeleteTaskType(context.Background(), "user-1", "type-1"); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("got %v", err)
	}
}
