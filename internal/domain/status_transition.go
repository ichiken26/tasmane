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
		if !target.HasChildren {
			for _, ancestor := range ancestors {
				if ancestor.Status != Completed {
					continue
				}
				reopened, transitionErr := ancestor.Transition(InProgress)
				if transitionErr != nil {
					return StatusTransitionPlan{}, transitionErr
				}
				plan.Changes = append(plan.Changes, transitionChange(ancestor, reopened))
			}
		}
	case target.Status == InProgress && to == NotStarted:
		plan.RequiresConfirmation = true
		if target.HasChildren {
			for _, descendant := range descendants {
				if descendant.Status != InProgress {
					continue
				}
				reset, transitionErr := descendant.Transition(NotStarted)
				if transitionErr != nil {
					return StatusTransitionPlan{}, transitionErr
				}
				plan.Changes = append(plan.Changes, transitionChange(descendant, reset))
			}
		}
	}
	return plan, nil
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
