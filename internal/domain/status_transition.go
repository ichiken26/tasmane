package domain

import "errors"

var ErrStatusImpactPlanChanged = errors.New("status impact plan changed")

// ロールバック計画の原子的な状態変更
type StatusTransitionChange struct {
	TaskID          string
	FromStatus      Status
	ToStatus        Status
	FromProgress    *int
	ToProgress      *int
	ExpectedVersion int64
	HasChildren     bool
}

// 状態遷移の影響計画
type StatusTransitionPlan struct {
	TaskID               string
	ToStatus             Status
	ExpectedVersion      int64
	RequiresConfirmation bool
	Changes              []StatusTransitionChange
}

// 全経路共通の状態遷移ロールバック計画
func PlanStatusTransition(target Task, ancestors, descendants []Task, to Status) (StatusTransitionPlan, error) {
	// 対象タスクの遷移可否判定
	next, err := target.Transition(to)
	if err != nil {
		return StatusTransitionPlan{}, err
	}
	plan := StatusTransitionPlan{TaskID: target.ID, ToStatus: to, ExpectedVersion: target.Version}
	plan.Changes = append(plan.Changes, transitionChange(target, next))

	// 完了巻き戻しと未着手リセットの伝播分岐
	switch {
	case target.Status == Completed && to == InProgress:
		plan.RequiresConfirmation = true
		// 親の完了巻き戻し時の子孫と祖先の維持
		if target.HasChildren {
			return plan, nil
		}
		// 完了祖先の再開適用
		changes, transitionErr := reopenCompletedAncestors(ancestors)
		if transitionErr != nil {
			return StatusTransitionPlan{}, transitionErr
		}
		plan.Changes = append(plan.Changes, changes...)
	case target.Status == InProgress && to == NotStarted:
		plan.RequiresConfirmation = true
		if !target.HasChildren {
			return plan, nil
		}
		// 進行中子孫の未着手リセット
		changes, transitionErr := resetInProgressDescendants(descendants)
		if transitionErr != nil {
			return StatusTransitionPlan{}, transitionErr
		}
		plan.Changes = append(plan.Changes, changes...)
	}
	return plan, nil
}

// 完了祖先の進行中再開
func reopenCompletedAncestors(ancestors []Task) ([]StatusTransitionChange, error) {
	changes := make([]StatusTransitionChange, 0, len(ancestors))
	for _, ancestor := range ancestors {
		if ancestor.Status != Completed {
			continue
		}
		reopened, err := ancestor.Transition(InProgress)
		if err != nil {
			return nil, err
		}
		changes = append(changes, transitionChange(ancestor, reopened))
	}
	return changes, nil
}

// 進行中子孫の未着手リセット
func resetInProgressDescendants(descendants []Task) ([]StatusTransitionChange, error) {
	changes := make([]StatusTransitionChange, 0, len(descendants))
	for _, descendant := range descendants {
		if descendant.Status != InProgress {
			continue
		}
		reset, err := descendant.Transition(NotStarted)
		if err != nil {
			return nil, err
		}
		changes = append(changes, transitionChange(descendant, reset))
	}
	return changes, nil
}

// 遷移前後の差分レコード生成
func transitionChange(from, to Task) StatusTransitionChange {
	return StatusTransitionChange{
		TaskID: from.ID, FromStatus: from.Status, ToStatus: to.Status,
		FromProgress: cloneInt(from.Progress), ToProgress: cloneInt(to.ForPersistence().Progress),
		ExpectedVersion: from.Version, HasChildren: from.HasChildren,
	}
}

// 整数ポインタの複製
func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
