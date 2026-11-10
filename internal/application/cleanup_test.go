package application

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// 既定30日カットオフの物理削除確認
func TestHardDeleteExpiredTasksUsesThirtyDayCutoff(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	// 30日前カットオフのSQL期待
	mock.ExpectExec(regexp.QuoteMeta(hardDeleteSoftDeletedTasksSQL)).WithArgs(now.Add(-30 * 24 * time.Hour)).WillReturnResult(sqlmock.NewResult(0, 3))
	count, err := (Cleanup{DB: db}).HardDeleteExpiredTasks(context.Background(), now)
	if err != nil || count != 3 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// DB必須・負保持期間・実行失敗の検証確認
func TestHardDeleteExpiredTasksValidatesAndWrapsErrors(t *testing.T) {
	// nil DBの拒否確認
	if _, err := (Cleanup{}).HardDeleteExpiredTasks(context.Background(), time.Now()); err == nil {
		t.Fatal("expected nil DB error")
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 負の保持期間の拒否確認
	if _, err := (Cleanup{DB: db, Retention: -1}).HardDeleteExpiredTasks(context.Background(), time.Now()); err == nil {
		t.Fatal("expected retention error")
	}
	// 実行失敗のラップ確認
	mock.ExpectExec("DELETE FROM tasks").WillReturnError(errors.New("denied"))
	if _, err := (Cleanup{DB: db}).HardDeleteExpiredTasks(context.Background(), time.Now()); err == nil {
		t.Fatal("expected exec error")
	}
}
