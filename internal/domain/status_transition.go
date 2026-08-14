package domain

import "errors"

var ErrStatusImpactPlanChanged = errors.New("status impact plan changed")

// StatusTransitionChange is one atomic status mutation in a rollback plan.
// Progress is the persisted leaf_progress value; parent progress remains derived.
type StatusTransitionChange struct {
	TaskID          string
	FromStatus      Status
	ToStatus        Status
	FromProgress    *int
	ToProgress      *int
	ExpectedVersion int64
	HasChildren     bool
}

type StatusTransitionPlan struct {
	TaskID               string
	ToStatus             Status
	ExpectedVersion      int64
	RequiresConfirmation bool
	Changes              []StatusTransitionChange
}

// PlanStatusTransition centralizes rollback propagation for every transport.
// ancestors must be direct-parent first; descendants may be in any stable order.
func PlanStatusTransition(target Task, ancestors, descendants []Task, to Status) (StatusTransitionPlan, error) {
	next, err := target.Transition(to)
	if err != nil {
		return StatusTransitionPlan{}, err
	}
	plan := StatusTransitionPlan{TaskID: target.ID, ToStatus: to, ExpectedVersion: target.Version}
	plan.Changes = append(plan.Changes, transitionChange(target, next))

	switch {
	case target.Status == Completed && to == InProgress:
		plan.RequiresConfirmation = true
		// A parent rollback only changes the parent. Completed descendants and
		// ancestors are deliberately preserved.
		if target.HasChildren {
			return plan, nil
		}
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
		changes, transitionErr := resetInProgressDescendants(descendants)
		if transitionErr != nil {
			return StatusTransitionPlan{}, transitionErr
		}
		plan.Changes = append(plan.Changes, changes...)
	}
	return plan, nil
}

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

func transitionChange(from, to Task) StatusTransitionChange {
	return StatusTransitionChange{
		TaskID: from.ID, FromStatus: from.Status, ToStatus: to.Status,
		FromProgress: cloneInt(from.Progress), ToProgress: cloneInt(to.ForPersistence().Progress),
		ExpectedVersion: from.Version, HasChildren: from.HasChildren,
	}
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
