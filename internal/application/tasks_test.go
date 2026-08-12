package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
)

func TestTransitionOwnershipOptimisticLockAndGoogleInvariant(t *testing.T) {
	r := repository.NewMemory(domain.Task{ID: "t1", UserID: "u1", Status: domain.NotStarted, Version: 1})
	u := Tasks{Repo: r}
	if _, err := u.Transition(context.Background(), "u2", "t1", 1, domain.InProgress, "MCP"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	got, err := u.Transition(context.Background(), "u1", "t1", 1, domain.InProgress, "MCP")
	if err != nil || got.Version != 2 {
		t.Fatal(got, err)
	}
	if _, err = u.Transition(context.Background(), "u1", "t1", 1, domain.NotStarted, "WEB"); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatal(err)
	}
	r2 := repository.NewMemory(domain.Task{ID: "blocked", UserID: "u1", Status: domain.NotStarted, Version: 1, IncompleteDescendants: 1})
	g := Tasks{Repo: r2}
	if _, err = g.GoogleComplete(context.Background(), "u1", "blocked", 1); !errors.Is(err, domain.ErrNotCompletable) {
		t.Fatal(err)
	}
}
