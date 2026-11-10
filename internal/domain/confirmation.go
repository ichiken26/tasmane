package domain

import (
	"errors"
	"tasmane/internal/domain/scoring"
	"time"
)

// 確認状態の列挙
type ConfirmationStatus string

const (
	ConfirmationUnconfirmed ConfirmationStatus = "UNCONFIRMED"
	ConfirmationChecking    ConfirmationStatus = "CHECKING"
	ConfirmationConfirmed   ConfirmationStatus = "CONFIRMED"
)

// 確認ドメインのエラー群
var (
	ErrInvalidConfirmationTransition = errors.New("invalid confirmation transition")
	ErrInvalidConfirmationInput      = errors.New("invalid confirmation input")
	// 完了済みタスクへ未確認の確認事項を持たせない（完了制約の不変条件）
	ErrConfirmationOnCompletedTask = errors.New("completed task cannot have unresolved confirmations")
)

// タスク確認
type Confirmation struct {
	ID             string
	UserID         string
	TaskID         string
	Content        string
	TargetName     string
	Status         ConfirmationStatus
	ExpectedStatus ConfirmationStatus
	DueAt          *time.Time
	ConfirmedAt    *time.Time
}

// 確認状態遷移のドメイン規則
func (c Confirmation) Transition(to ConfirmationStatus, at time.Time) (Confirmation, error) {
	// 許容遷移の判定
	valid := (c.Status == ConfirmationUnconfirmed && to == ConfirmationChecking) ||
		(c.Status == ConfirmationChecking && (to == ConfirmationUnconfirmed || to == ConfirmationConfirmed)) ||
		(c.Status == ConfirmationConfirmed && to == ConfirmationChecking)
	if !valid {
		return Confirmation{}, ErrInvalidConfirmationTransition
	}
	// 確定時刻の入力検証
	if to == ConfirmationConfirmed && at.IsZero() {
		return Confirmation{}, ErrInvalidConfirmationInput
	}
	// 状態と確定時刻の適用
	c.ExpectedStatus = c.Status
	c.Status = to
	if to == ConfirmationConfirmed {
		value := at
		c.ConfirmedAt = &value
	} else {
		c.ConfirmedAt = nil
	}
	return c, nil
}

// 確認スコア算出の入力
type ConfirmationScoreInput struct {
	Status          ConfirmationStatus
	DueAt           *time.Time
	ParentTaskScore int
	Now             time.Time
}

// 確認スコアの算出
func CalculateConfirmationScore(input ConfirmationScoreInput) (int, bool, error) {
	// 基準時刻と期限と親スコアの入力検証
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
	// 期限なし固定点と期限あり加重の分岐
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

// 確認先プリセット（ユーザー個人）
type ConfirmationTarget struct {
	ID   string
	Name string
}
