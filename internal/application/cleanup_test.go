package application

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHardDeleteExpiredTasksUsesThirtyDayCutoff(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(hardDeleteSoftDeletedTasksSQL)).WithArgs(now.Add(-30 * 24 * time.Hour)).WillReturnResult(sqlmock.NewResult(0, 3))
	count, err := (Cleanup{DB: db}).HardDeleteExpiredTasks(context.Background(), now)
	if err != nil || count != 3 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHardDeleteExpiredTasksValidatesAndWrapsErrors(t *testing.T) {
	if _, err := (Cleanup{}).HardDeleteExpiredTasks(context.Background(), time.Now()); err == nil {
		t.Fatal("expected nil DB error")
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := (Cleanup{DB: db, Retention: -1}).HardDeleteExpiredTasks(context.Background(), time.Now()); err == nil {
		t.Fatal("expected retention error")
	}
	mock.ExpectExec("DELETE FROM tasks").WillReturnError(errors.New("denied"))
	if _, err := (Cleanup{DB: db}).HardDeleteExpiredTasks(context.Background(), time.Now()); err == nil {
		t.Fatal("expected exec error")
	}
}
