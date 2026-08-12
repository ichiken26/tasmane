package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"tasmane/internal/domain"
)

func TestPostgresConfirmationCreateRequiresOwnedActiveTask(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{values: []string{"confirmation-1"}})
	due := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("INSERT INTO task_confirmations").
		WithArgs("confirmation-1", "user-1", "task-1", "Ship it", "Owner", due).
		WillReturnRows(sqlmock.NewRows(confirmationColumns()).
			AddRow("confirmation-1", "user-1", "task-1", "Ship it", "Owner", "UNCONFIRMED", due, nil))
	got, err := repository.CreateConfirmation(context.Background(), "user-1", "task-1", " Ship it ", " Owner ", &due)
	if err != nil || got.ID != "confirmation-1" || got.Status != domain.ConfirmationUnconfirmed {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestPostgresConfirmationUpdateUsesExpectedStatus(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	next := domain.Confirmation{
		ID: "confirmation-1", UserID: "user-1", Status: domain.ConfirmationChecking,
		ExpectedStatus: domain.ConfirmationUnconfirmed,
	}
	mock.ExpectExec("UPDATE task_confirmations").
		WithArgs(domain.ConfirmationChecking, nil, "confirmation-1", "user-1", domain.ConfirmationUnconfirmed).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repository.UpdateConfirmation(context.Background(), "user-1", "confirmation-1", next); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresConfirmationUpdateReportsConcurrentTransition(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repository := NewPostgres(database, &fixedIDs{})
	next := domain.Confirmation{
		ID: "confirmation-1", UserID: "user-1", Status: domain.ConfirmationConfirmed,
		ExpectedStatus: domain.ConfirmationChecking,
	}
	mock.ExpectExec("UPDATE task_confirmations").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := repository.UpdateConfirmation(context.Background(), "user-1", "confirmation-1", next); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("got %v", err)
	}
}

func confirmationColumns() []string {
	return []string{"id", "user_id", "task_id", "content", "target_name", "status", "due_at", "confirmed_at"}
}
