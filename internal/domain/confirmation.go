package domain

import (
	"errors"
	"tasmane/internal/domain/scoring"
	"time"
)

type ConfirmationStatus string

const (
	ConfirmationUnconfirmed ConfirmationStatus = "UNCONFIRMED"
	ConfirmationChecking    ConfirmationStatus = "CHECKING"
	ConfirmationConfirmed   ConfirmationStatus = "CONFIRMED"
)

var (
	ErrInvalidConfirmationTransition = errors.New("invalid confirmation transition")
	ErrInvalidConfirmationInput      = errors.New("invalid confirmation input")
)

type Confirmation struct {
	Status      ConfirmationStatus
	ConfirmedAt *time.Time
}

func (c Confirmation) Transition(to ConfirmationStatus, at time.Time) (Confirmation, error) {
	valid := (c.Status == ConfirmationUnconfirmed && to == ConfirmationChecking) ||
		(c.Status == ConfirmationChecking && (to == ConfirmationUnconfirmed || to == ConfirmationConfirmed)) ||
		(c.Status == ConfirmationConfirmed && to == ConfirmationChecking)
	if !valid {
		return Confirmation{}, ErrInvalidConfirmationTransition
	}
	if to == ConfirmationConfirmed && at.IsZero() {
		return Confirmation{}, ErrInvalidConfirmationInput
	}
	c.Status = to
	if to == ConfirmationConfirmed {
		value := at
		c.ConfirmedAt = &value
	} else {
		c.ConfirmedAt = nil
	}
	return c, nil
}

type ConfirmationScoreInput struct {
	Status          ConfirmationStatus
	DueAt           *time.Time
	ParentTaskScore int
	Now             time.Time
}

func CalculateConfirmationScore(input ConfirmationScoreInput) (int, bool, error) {
	if input.Now.IsZero() || (input.DueAt != nil && input.DueAt.IsZero()) || input.ParentTaskScore < 0 || input.ParentTaskScore > 100 {
		return 0, false, ErrInvalidConfirmationInput
	}
	if input.Status == ConfirmationConfirmed {
		return 0, false, nil
	}
	if input.Status != ConfirmationUnconfirmed && input.Status != ConfirmationChecking {
		return 0, false, ErrInvalidConfirmationInput
	}
	parent := input.ParentTaskScore
	statusScore := 0
	if input.Status == ConfirmationChecking {
		statusScore = 100
	}
	if input.DueAt == nil {
		fixed := scoring.ConfirmationNoDueUnconfirmed
		if input.Status == ConfirmationChecking {
			fixed = scoring.ConfirmationNoDueChecking
		}
		return clampScore(roundScore(float64(fixed)*(scoring.ConfirmationDueWeight+scoring.ConfirmationStatusWeight) + float64(parent)*scoring.ConfirmationParentWeight)), true, nil
	}
	due := deadlineUrgency(input.Now, *input.DueAt)
	return clampScore(roundScore(float64(due)*scoring.ConfirmationDueWeight + float64(statusScore)*scoring.ConfirmationStatusWeight + float64(parent)*scoring.ConfirmationParentWeight)), true, nil
}
