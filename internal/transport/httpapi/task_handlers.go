package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"tasmane/internal/application"
	"tasmane/internal/domain"
)

// タスク操作ユースケースの組み立て
func (h Handler) taskCommands() application.TaskCommands {
	return application.TaskCommands{Tasks: h.Tasks, Relations: h.Relations, Tokens: h.ImpactTokens}
}

// 影響確認が必要な場合の409応答（書き出したらtrue）
func writeImpactRequired(w http.ResponseWriter, err error) bool {
	var impact *application.ImpactRequiredError
	if !errors.As(err, &impact) {
		return false
	}
	entities := make([]impactEntityResponse, 0, len(impact.AffectedEntities))
	for _, entity := range impact.AffectedEntities {
		entities = append(entities, impactEntityResponse{ID: entity.ID, Type: entity.Type, Title: entity.Title, Reason: entity.Reason})
	}
	writeJSON(w, http.StatusConflict, impactPlanResponse{
		Code: "IMPACT_CONFIRMATION_REQUIRED", ImpactToken: impact.ImpactToken, AffectedEntities: entities, Warnings: impact.Warnings,
	})
	return true
}

// 操作エラーの書き出し（影響確認・不正リクエスト・ドメインエラー）
func writeCommandError(w http.ResponseWriter, err error) {
	if writeImpactRequired(w, err) {
		return
	}
	var bad badRequestError
	if errors.As(err, &bad) {
		writeBadRequest(w, err)
		return
	}
	writeDomainError(w, err)
}

// 任意文字列ポインタの取り出し
func tokenValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// タスクステータスの遷移
func (h Handler) transitionTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input transitionRequest
	if err := decodeJSON(w, r, &input); err != nil || input.UserID != "" || input.ExpectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	output, err := h.taskCommands().Transition(r.Context(), userID, chi.URLParam(r, "taskId"), input.ExpectedVersion, input.ToStatus, requestVia(r), tokenValue(input.ImpactToken))
	if err != nil {
		writeCommandError(w, err)
		return
	}
	h.writeTask(w, r, http.StatusOK, output)
}

// タスクの削除（人間UI/APIのみ）
func (h Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	expected, err := strconv.ParseInt(r.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil || expected < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid expectedVersion")
		return
	}
	if err := h.taskCommands().Delete(r.Context(), userID, chi.URLParam(r, "taskId"), expected, r.URL.Query().Get("impactToken")); err != nil {
		writeCommandError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// タスクの部分更新
func (h Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input patchTaskRequest
	if decodeJSON(w, r, &input) != nil || input.ExpectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	result, err := h.taskCommands().Update(r.Context(), userID, chi.URLParam(r, "taskId"), input.ExpectedVersion, func(current domain.Task) (domain.Task, error) {
		return applyTaskPatch(current, input)
	}, tokenValue(input.ImpactToken))
	if err != nil {
		writeCommandError(w, err)
		return
	}
	projected, err := h.projections().ProjectTask(r.Context(), userID, result.Task)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	response := toProjectedResponse(projected)
	response.Warnings = result.Warnings
	writeJSON(w, http.StatusOK, response)
}

// タスクの作成
func (h Handler) createTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input createTaskRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	isAllDay := true
	if input.IsAllDay != nil {
		isAllDay = *input.IsAllDay
	}
	draft := domain.TaskDraft{
		GenreID: input.GenreID, TaskTypeID: input.TaskTypeID, ParentTaskID: input.ParentTaskID,
		Title: input.Title, Body: input.Body, StartAt: input.StartAt, EndAt: input.EndAt,
		IsAllDay: isAllDay, ManualPriority: input.ManualPriority,
	}
	created, err := h.taskCommands().Create(r.Context(), userID, draft, tokenValue(input.ImpactToken))
	if err != nil {
		writeCommandError(w, err)
		return
	}
	h.writeTask(w, r, http.StatusCreated, created)
}

// 葉進捗更新のリクエスト本体
type progressRequest struct {
	Progress        *int  `json:"progress"`
	ExpectedVersion int64 `json:"expectedVersion"`
}

// 葉タスク進捗の更新
func (h Handler) setTaskProgress(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input progressRequest
	if decodeJSON(w, r, &input) != nil || input.Progress == nil || input.ExpectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	updated, err := h.taskCommands().SetProgress(r.Context(), userID, chi.URLParam(r, "taskId"), input.ExpectedVersion, *input.Progress, requestVia(r))
	if err != nil {
		writeCommandError(w, err)
		return
	}
	h.writeTask(w, r, http.StatusOK, updated)
}

// 削除済みタスクの応答
type deletedTaskResponse struct {
	ID           string        `json:"id"`
	GenreID      string        `json:"genreId"`
	ParentTaskID *string       `json:"parentTaskId"`
	Title        string        `json:"title"`
	Status       domain.Status `json:"status"`
	EndAt        *time.Time    `json:"endAt"`
	DeletedAt    time.Time     `json:"deletedAt"`
	PurgeAt      time.Time     `json:"purgeAt"`
}

// 復元可能な削除済みタスク一覧
func (h Handler) listDeletedTasks(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.taskCommands().Deleted(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]deletedTaskResponse, 0, len(values))
	for _, value := range values {
		result = append(result, deletedTaskResponse{
			ID: value.ID, GenreID: value.GenreID, ParentTaskID: value.ParentTaskID, Title: value.Title,
			Status: value.Status, EndAt: value.EndAt, DeletedAt: value.DeletedAt, PurgeAt: value.DeletedAt.Add(30 * 24 * time.Hour),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

// 削除済みタスクの復元
func (h Handler) restoreTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	ids, err := h.taskCommands().Restore(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"restoredTaskIds": ids})
}

// 変更経路（Worker が WebMCP 由来の操作に限り付与、それ以外は WEB）
func requestVia(r *http.Request) string {
	if r.Header.Get("X-Tasmane-Via") == "WEB_MCP" {
		return "WEB_MCP"
	}
	return "WEB"
}
