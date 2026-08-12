package domain

import (
	"errors"
	"testing"
	"time"
)

func TestConfirmationFSM(t *testing.T) {
	c := Confirmation{Status: ConfirmationUnconfirmed}
	checking, err := c.Transition(ConfirmationChecking, time.Time{})
	if err != nil || checking.Status != ConfirmationChecking || checking.ConfirmedAt != nil {
		t.Fatal(checking, err)
	}
	confirmedAt := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	confirmed, err := checking.Transition(ConfirmationConfirmed, confirmedAt)
	if err != nil || confirmed.ConfirmedAt == nil || !confirmed.ConfirmedAt.Equal(confirmedAt) {
		t.Fatal(confirmed, err)
	}
	back, err := confirmed.Transition(ConfirmationChecking, time.Time{})
	if err != nil || back.ConfirmedAt != nil {
		t.Fatal(back, err)
	}
	if _, err = c.Transition(ConfirmationConfirmed, confirmedAt); !errors.Is(err, ErrInvalidConfirmationTransition) {
		t.Fatal(err)
	}
	if _, err = confirmed.Transition(ConfirmationUnconfirmed, confirmedAt); !errors.Is(err, ErrInvalidConfirmationTransition) {
		t.Fatal(err)
	}
}

func TestConfirmationFSMRejectsZeroConfirmedAt(t *testing.T) {
	checking := Confirmation{Status: ConfirmationChecking}
	if _, err := checking.Transition(ConfirmationConfirmed, time.Time{}); !errors.Is(err, ErrInvalidConfirmationInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestConfirmationFSMTransitionTable(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	statuses := []ConfirmationStatus{ConfirmationUnconfirmed, ConfirmationChecking, ConfirmationConfirmed}
	allowed := map[[2]ConfirmationStatus]bool{
		{ConfirmationUnconfirmed, ConfirmationChecking}: true,
		{ConfirmationChecking, ConfirmationUnconfirmed}: true,
		{ConfirmationChecking, ConfirmationConfirmed}:   true,
		{ConfirmationConfirmed, ConfirmationChecking}:   true,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				_, err := (Confirmation{Status: from}).Transition(to, now)
				if allowed[[2]ConfirmationStatus{from, to}] && err != nil {
					t.Fatalf("allowed transition rejected: %v", err)
				}
				if !allowed[[2]ConfirmationStatus{from, to}] && !errors.Is(err, ErrInvalidConfirmationTransition) {
					t.Fatalf("forbidden transition error = %v", err)
				}
			})
		}
	}
}

func TestConfirmationScoreBoundariesAndStatus(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	unconfirmed, ok, err := CalculateConfirmationScore(ConfirmationScoreInput{Status: ConfirmationUnconfirmed, ParentTaskScore: 50, Now: now})
	if err != nil || !ok || unconfirmed != 29 {
		t.Fatal(unconfirmed, ok, err)
	}
	checking, ok, err := CalculateConfirmationScore(ConfirmationScoreInput{Status: ConfirmationChecking, ParentTaskScore: 50, Now: now})
	if err != nil || !ok || checking != 43 || checking <= unconfirmed {
		t.Fatal(checking, ok, err)
	}
	due := now.Add(24 * time.Hour)
	urgent, ok, err := CalculateConfirmationScore(ConfirmationScoreInput{Status: ConfirmationChecking, DueAt: &due, ParentTaskScore: 100, Now: now})
	if err != nil || !ok || urgent != 100 {
		t.Fatal(urgent, ok, err)
	}
	if _, ok, err = CalculateConfirmationScore(ConfirmationScoreInput{Status: ConfirmationConfirmed, Now: now}); err != nil || ok {
		t.Fatal("confirmed must be excluded")
	}
}

func TestConfirmationScoreRejectsInvalidInput(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	zero := time.Time{}
	for _, input := range []ConfirmationScoreInput{
		{Status: "UNKNOWN", Now: now},
		{Status: ConfirmationChecking, ParentTaskScore: -1, Now: now},
		{Status: ConfirmationChecking, ParentTaskScore: 101, Now: now},
		{Status: ConfirmationChecking},
		{Status: ConfirmationChecking, DueAt: &zero, Now: now},
	} {
		if _, _, err := CalculateConfirmationScore(input); !errors.Is(err, ErrInvalidConfirmationInput) {
			t.Fatalf("input=%+v err=%v", input, err)
		}
	}
}
