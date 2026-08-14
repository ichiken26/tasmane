package httpapi

import (
	"net/http"

	"tasmane/internal/domain"
)

func applyTaskPatch(current domain.Task, input patchTaskRequest) (domain.Task, error) {
	next := current
	if input.Title.Set {
		if input.Title.Value == nil {
			return domain.Task{}, errBadRequest("title cannot be null")
		}
		next.Title = *input.Title.Value
	}
	if input.Body.Set {
		next.Body = input.Body.Value
	}
	if input.GenreID.Set {
		if input.GenreID.Value == nil {
			return domain.Task{}, errBadRequest("genreId cannot be null")
		}
		next.GenreID = *input.GenreID.Value
	}
	if input.TaskTypeID.Set {
		next.TaskTypeID = input.TaskTypeID.Value
	}
	if input.ParentTaskID.Set {
		next.ParentTaskID = input.ParentTaskID.Value
	}
	if input.StartAt.Set {
		next.StartAt = input.StartAt.Value
	}
	if input.EndAt.Set {
		next.EndAt = input.EndAt.Value
	}
	if input.IsAllDay.Set {
		if input.IsAllDay.Value == nil {
			return domain.Task{}, errBadRequest("isAllDay cannot be null")
		}
		next.IsAllDay = *input.IsAllDay.Value
	}
	if input.ManualPriority.Set {
		next.ManualPriority = input.ManualPriority.Value
	}
	return next, nil
}

type badRequestError struct{ message string }

func errBadRequest(message string) error { return badRequestError{message: message} }
func (e badRequestError) Error() string  { return e.message }

func toDateTasks(tree []domain.Task) []domain.DateTask {
	dateTasks := make([]domain.DateTask, 0, len(tree))
	for _, task := range tree {
		parent := ""
		if task.ParentTaskID != nil {
			parent = *task.ParentTaskID
		}
		dateTasks = append(dateTasks, domain.DateTask{ID: task.ID, ParentID: parent, StartAt: task.StartAt, EndAt: task.EndAt})
	}
	return dateTasks
}

func dateImpactRequired(taskID string, changes []domain.DateChange) bool {
	for _, change := range changes {
		if change.TaskID != taskID {
			return true
		}
	}
	return false
}

func buildUpdateImpactPlan(token string, changes []domain.DateChange, movePlan domain.GenreMovePlan, needsDateImpact, genreChanged bool) impactPlanResponse {
	entities := make([]impactEntityResponse, 0, len(changes)+len(movePlan.MovedTaskIDs)+len(movePlan.RemovedDependencies))
	warnings := []string{}
	if needsDateImpact {
		for _, change := range changes {
			entities = append(entities, impactEntityResponse{ID: change.TaskID, Type: "TASK", Reason: stringPointer("DATE_PROPAGATION")})
		}
		warnings = append(warnings, "日付変更により関連タスクの日付も変更されます。")
	}
	if genreChanged {
		for _, id := range movePlan.MovedTaskIDs {
			entities = append(entities, impactEntityResponse{ID: id, Type: "TASK", Reason: stringPointer("GENRE_MOVE")})
		}
		for _, dependency := range movePlan.RemovedDependencies {
			id := dependency.TaskID + "->" + dependency.DependsOnTaskID
			entities = append(entities, impactEntityResponse{ID: id, Type: "DEPENDENCY", Reason: stringPointer("GENRE_BOUNDARY_REMOVAL")})
		}
		warnings = append(warnings, "Genre移動により境界を跨ぐ依存関係が解除されます。")
	}
	return impactPlanResponse{
		Code: "IMPACT_CONFIRMATION_REQUIRED", ImpactToken: token, AffectedEntities: entities, Warnings: warnings,
	}
}

func (h Handler) planGenreMove(r *http.Request, userID, taskID, genreID string) (domain.GenreMovePlan, error) {
	if h.Relations.Repo == nil {
		return domain.GenreMovePlan{}, errBadRequest("relation repository unavailable")
	}
	graphTasks, dependencies, err := h.Relations.Snapshot(r.Context(), userID)
	if err != nil {
		return domain.GenreMovePlan{}, err
	}
	return domain.PlanGenreMove(graphTasks, dependencies, taskID, genreID)
}

func writeBadRequest(w http.ResponseWriter, err error) {
	message := err.Error()
	status := http.StatusBadRequest
	code := "INVALID_REQUEST"
	if message == "relation repository unavailable" {
		status = http.StatusInternalServerError
		code = "INTERNAL_ERROR"
	}
	writeError(w, status, code, message)
}
