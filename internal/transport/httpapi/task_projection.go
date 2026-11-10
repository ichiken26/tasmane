package httpapi

import (
	"net/http"

	"tasmane/internal/application"
	"tasmane/internal/domain"
)

// 投影ユースケースの組み立て
func (h Handler) projections() application.Projections {
	return application.Projections{Tasks: h.Tasks, Relations: h.Relations, Confirmations: h.Confirmations}
}

// ドメイン投影からタスク応答への変換
func toProjectedResponse(value domain.TaskProjection) taskResponse {
	return application.NewTaskView(value)
}

// 投影列の応答変換
func toProjectedResponses(values []domain.TaskProjection) []taskResponse {
	return application.NewTaskViews(values)
}

// 単一タスクの算出値付き応答
func (h Handler) writeTask(w http.ResponseWriter, r *http.Request, status int, task domain.Task) {
	userID, _ := UserID(r.Context())
	projected, err := h.projections().ProjectTask(r.Context(), userID, task)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, status, toProjectedResponse(projected))
}
