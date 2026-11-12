package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// 同期状態（連携・件数・失敗一覧）は遠隔DBへ1往復で読む
func TestGoogleSyncStatusReadsEverythingInOneQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	finished := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT integration.last_sync_finished_at").WithArgs("u").WillReturnRows(
		sqlmock.NewRows([]string{"finished", "missing", "conflicts", "jobs", "failed"}).
			AddRow(finished, nil, 2, 3, []byte(`[{"taskId":"t1","title":"見積","message":"403"}]`)))
	status, err := NewPostgres(database, nil).GoogleSyncStatus(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.TaskListMissing || status.LastSyncFinishedAt == nil || !status.LastSyncFinishedAt.Equal(finished) ||
		status.ConflictCount != 2 || status.PendingJobs != 3 || len(status.Failed) != 1 || status.Failed[0].Title != "見積" {
		t.Fatalf("status=%+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 未連携なら空の状態（失敗一覧は空配列）
func TestGoogleSyncStatusWithoutIntegration(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery("SELECT integration.last_sync_finished_at").WithArgs("u").WillReturnRows(sqlmock.NewRows([]string{"finished", "missing", "conflicts", "jobs", "failed"}))
	status, err := NewPostgres(database, nil).GoogleSyncStatus(context.Background(), "u")
	if err != nil || status.Connected || status.Failed == nil || len(status.Failed) != 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}
