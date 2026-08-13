package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGoogleQueueClaimUsesSkipLockedLeaseAndAckOwnership(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := NewPostgres(database, nil)
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`WITH candidate AS (
    SELECT id FROM google_sync_jobs
    WHERE available_at <= $1 AND (claimed_at IS NULL OR claimed_at < $1-$3::interval)
    ORDER BY available_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1
  ) UPDATE google_sync_jobs job SET claimed_at=$1,claimed_by=$2
    FROM candidate WHERE job.id=candidate.id
    RETURNING job.id,job.user_id,job.task_id,job.action,job.attempt`)).
		WithArgs(now, "worker-1", "120.000000 seconds").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "task_id", "action", "attempt"}).AddRow("j1", "u1", nil, "PULL_USER", 2))
	job, ok, err := store.Claim(context.Background(), "worker-1", now, 2*time.Minute)
	if err != nil || !ok || job.ID != "j1" || job.UserID != "u1" || job.Attempt != 2 {
		t.Fatalf("job=%#v ok=%v err=%v", job, ok, err)
	}
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM google_sync_jobs WHERE id=$1 AND claimed_by=$2`)).WithArgs("j1", "worker-1").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.Ack(context.Background(), "j1", "worker-1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
