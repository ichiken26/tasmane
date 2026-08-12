package application

import (
	"context"
	"errors"
	"tasmane/internal/domain"
	"testing"
	"time"
)

type relationRepoStub struct {
	tasks  []domain.GraphTask
	edges  []domain.Dependency
	writes int
}

func (r *relationRepoStub) LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error) {
	return r.tasks, r.edges, nil
}
func (r *relationRepoStub) SetParent(context.Context, string, string, string) error {
	r.writes++
	return nil
}
func (r *relationRepoStub) AddDependency(context.Context, string, domain.Dependency) error {
	r.writes++
	return nil
}

func TestRelationsValidateBeforePersistence(t *testing.T) {
	tasks := []domain.GraphTask{
		{ID: "a", UserID: "u", GenreID: "g", ParentID: "b"},
		{ID: "b", UserID: "u", GenreID: "g"},
		{ID: "other", UserID: "other", GenreID: "g"},
	}
	r := &relationRepoStub{tasks: tasks, edges: []domain.Dependency{{TaskID: "a", DependsOnTaskID: "b"}}}
	u := Relations{Repo: r}
	if err := u.AssignParent(context.Background(), "u", "b", "a"); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatal(err)
	}
	if err := u.AddDependency(context.Background(), "u", "b", "a"); !errors.Is(err, domain.ErrDependencyCycle) {
		t.Fatal(err)
	}
	if err := u.AssignParent(context.Background(), "u", "a", "other"); !errors.Is(err, domain.ErrGraphOwnership) {
		t.Fatal(err)
	}
	if r.writes != 0 {
		t.Fatalf("invalid relations persisted: %d", r.writes)
	}
	if err := u.AssignParent(context.Background(), "u", "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := u.AddDependency(context.Background(), "u", "a", "b"); err != nil {
		t.Fatal(err)
	}
	if r.writes != 2 {
		t.Fatalf("valid writes=%d", r.writes)
	}
}

type confirmationRepoStub struct {
	value  domain.Confirmation
	writes int
}

func (r *confirmationRepoStub) GetConfirmation(context.Context, string, string) (domain.Confirmation, error) {
	return r.value, nil
}
func (r *confirmationRepoStub) UpdateConfirmation(_ context.Context, _, _ string, value domain.Confirmation) error {
	r.value = value
	r.writes++
	return nil
}

func TestConfirmationsValidateTransitionBeforePersistence(t *testing.T) {
	r := &confirmationRepoStub{value: domain.Confirmation{Status: domain.ConfirmationUnconfirmed}}
	u := Confirmations{Repo: r}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if _, err := u.Transition(context.Background(), "u", "c", domain.ConfirmationConfirmed, now); !errors.Is(err, domain.ErrInvalidConfirmationTransition) {
		t.Fatal(err)
	}
	if r.writes != 0 {
		t.Fatal("invalid transition persisted")
	}
	got, err := u.Transition(context.Background(), "u", "c", domain.ConfirmationChecking, now)
	if err != nil || got.Status != domain.ConfirmationChecking || r.writes != 1 {
		t.Fatal(got, err, r.writes)
	}
}
