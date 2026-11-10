package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"tasmane/internal/domain"
)

// overdueは未完了かつ現在時刻より前の期限のみ
func TestPostgresSearchOverdueExcludesCompletedTasks(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	mock.ExpectQuery(regexp.QuoteMeta(`AND task.end_at < $2 AND task.status <> 'COMPLETED'::task_status`)).
		WithArgs("user-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := repository.Search(context.Background(), "user-1", domain.TaskSearch{Deadline: "overdue"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// OpenAPI enum外の期限区分は入力エラー
func TestPostgresSearchRejectsUnknownDeadline(t *testing.T) {
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if _, err := NewPostgres(database, &fixedIDs{}).Search(context.Background(), "user-1", domain.TaskSearch{Deadline: "yesterday"}); err != domain.ErrInvalidTaskInput {
		t.Fatalf("err=%v", err)
	}
}
