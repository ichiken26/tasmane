package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"tasmane/internal/application"
	"tasmane/internal/domain"
)

type Handler struct {
	Tasks         application.Tasks
	Catalog       application.Catalog
	Relations     application.Relations
	Confirmations application.Confirmations
	Settings      application.Settings
	ImpactTokens  application.ImpactTokens
	Advices       application.Advices
	GoogleJobs    application.GoogleJobs
	GoogleQueries application.GoogleQueries
}

type transitionRequest struct {
	ToStatus        domain.Status `json:"toStatus"`
	ExpectedVersion int64         `json:"expectedVersion"`
	ImpactToken     *string       `json:"impactToken,omitempty"`
	UserID          string        `json:"userId,omitempty"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(handler Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(identityOverrideGuard)
	router.Post("/v1/tasks/{taskId}/transition", handler.transitionTask)
	router.Get("/v1/tasks/{taskId}", handler.getTask)
	router.Patch("/v1/tasks/{taskId}", handler.updateTask)
	router.Delete("/v1/tasks/{taskId}", handler.deleteTask)
	router.Post("/v1/tasks", handler.createTask)
	router.Get("/v1/tasks", handler.searchTasks)
	router.Get("/v1/tasks/{taskId}/tree", handler.getTaskTree)
	router.Get("/v1/tasks/{taskId}/dependencies", handler.listDependencies)
	router.Post("/v1/tasks/{taskId}/dependencies", handler.addDependency)
	router.Delete("/v1/tasks/{taskId}/dependencies/{dependsOnTaskId}", handler.removeDependency)
	router.Get("/v1/settings", handler.getSettings)
	router.Patch("/v1/settings", handler.updateSettings)
	router.Get("/v1/tasks/{taskId}/confirmations", handler.listConfirmations)
	router.Post("/v1/tasks/{taskId}/confirmations", handler.createConfirmation)
	router.Patch("/v1/confirmations/{confirmationId}", handler.updateConfirmation)
	router.Get("/v1/genres", handler.listGenres)
	router.Post("/v1/genres", handler.createGenre)
	router.Get("/v1/genres/{genreId}", handler.getGenre)
	router.Patch("/v1/genres/{genreId}", handler.renameGenre)
	router.Delete("/v1/genres/{genreId}", handler.deleteGenre)
	router.Get("/v1/genres/{genreId}/task-types", handler.listGenreTaskTypes)
	router.Post("/v1/genres/{genreId}/task-types", handler.attachGenreTaskType)
	router.Get("/v1/task-types", handler.listTaskTypes)
	router.Post("/v1/task-types", handler.createTaskType)
	router.Get("/v1/task-types/{taskTypeId}", handler.getTaskType)
	router.Patch("/v1/task-types/{taskTypeId}", handler.renameTaskType)
	router.Delete("/v1/task-types/{taskTypeId}", handler.deleteTaskType)
	router.Get("/v1/dashboard", handler.getDashboard)
	router.Get("/v1/dashboard/genres/{genreId}", handler.getGenreDashboard)
	router.Post("/v1/google/sync", handler.triggerGoogleSync)
	router.Get("/v1/google/conflicts", handler.listGoogleConflicts)
	router.Get("/v1/google/import-candidates", handler.listGoogleImportCandidates)
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})
	return router
}

type dashboardResponse struct {
	Tasks         []taskResponse                  `json:"tasks"`
	Confirmations []dashboardConfirmationResponse `json:"confirmations"`
	Advice        *adviceResponse                 `json:"advice"`
}

type adviceResponse struct {
	Content     string    `json:"content"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func (h Handler) getDashboard(w http.ResponseWriter, r *http.Request) { h.writeDashboard(w, r, nil) }
func (h Handler) getGenreDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genreId")
	h.writeDashboard(w, r, []string{id})
}
func (h Handler) writeDashboard(w http.ResponseWriter, r *http.Request, genres []string) {
	userID, _ := UserID(r.Context())
	now := time.Now().UTC()
	tasks, err := h.Tasks.Search(r.Context(), userID, domain.TaskSearch{GenreIDs: genres})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	_, dependencies, err := h.Relations.Snapshot(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	confirmations := make(map[string][]domain.Confirmation, len(tasks))
	for _, task := range tasks {
		values, listErr := h.Confirmations.List(r.Context(), userID, task.ID)
		if listErr != nil {
			writeDomainError(w, listErr)
			return
		}
		confirmations[task.ID] = values
	}
	projected, err := projectDashboardResponses(tasks, dependencies, confirmations, now)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	titles := make(map[string]string, len(tasks))
	for _, task := range tasks {
		titles[task.ID] = task.Title
	}
	recommendedConfirmations, err := projectDashboardConfirmations(projected, confirmations, titles, now)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var genreID *string
	if len(genres) == 1 {
		genreID = &genres[0]
	}
	var advice *adviceResponse
	if h.Advices.Repo != nil {
		current, adviceErr := h.Advices.Current(r.Context(), userID, genreID)
		if adviceErr == nil {
			advice = &adviceResponse{Content: current.Content, GeneratedAt: current.GeneratedAt}
		} else if !errors.Is(adviceErr, domain.ErrForbidden) {
			writeDomainError(w, adviceErr)
			return
		}
	}
	writeJSON(w, http.StatusOK, dashboardResponse{Tasks: projected, Confirmations: recommendedConfirmations, Advice: advice})
}

type dependencyRequest struct {
	DependsOnTaskID string `json:"dependsOnTaskId"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

type dependencyResponse struct {
	TaskID          string `json:"taskId"`
	DependsOnTaskID string `json:"dependsOnTaskId"`
}

func (h Handler) listDependencies(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.Relations.ListDependencies(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]dependencyResponse, 0, len(values))
	for _, value := range values {
		result = append(result, dependencyResponse{TaskID: value.TaskID, DependsOnTaskID: value.DependsOnTaskID})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h Handler) addDependency(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input dependencyRequest
	if decodeJSON(w, r, &input) != nil || input.DependsOnTaskID == "" || input.ExpectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	if err := h.Relations.AddDependency(r.Context(), userID, chi.URLParam(r, "taskId"), input.DependsOnTaskID, input.ExpectedVersion); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h Handler) removeDependency(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	expectedVersion, err := strconv.ParseInt(r.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil || expectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	taskID := chi.URLParam(r, "taskId")
	current, err := h.Tasks.Get(r.Context(), userID, taskID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if current.Version != expectedVersion {
		writeDomainError(w, domain.ErrVersionConflict)
		return
	}
	if err := h.Relations.RemoveDependency(r.Context(), userID, taskID, chi.URLParam(r, "dependsOnTaskId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type settingsResponse struct {
	Timezone                  string `json:"timezone"`
	GoogleSyncIntervalMinutes int    `json:"googleSyncIntervalMinutes"`
}

type patchSettingsRequest struct {
	Timezone                  *string `json:"timezone"`
	GoogleSyncIntervalMinutes *int    `json:"googleSyncIntervalMinutes"`
}

func (h Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	settings, err := h.Settings.Get(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsResponse{
		Timezone: settings.Timezone, GoogleSyncIntervalMinutes: settings.GoogleSyncIntervalMinutes,
	})
}

func (h Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	current, err := h.Settings.Get(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var input patchSettingsRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	if input.Timezone != nil {
		current.Timezone = *input.Timezone
	}
	if input.GoogleSyncIntervalMinutes != nil {
		current.GoogleSyncIntervalMinutes = *input.GoogleSyncIntervalMinutes
	}
	saved, err := h.Settings.Update(r.Context(), userID, current)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsResponse{
		Timezone: saved.Timezone, GoogleSyncIntervalMinutes: saved.GoogleSyncIntervalMinutes,
	})
}

type createConfirmationRequest struct {
	Content    string     `json:"content"`
	TargetName string     `json:"targetName"`
	DueAt      *time.Time `json:"dueAt"`
}

type updateConfirmationRequest struct {
	Status domain.ConfirmationStatus `json:"status"`
}

type confirmationResponse struct {
	ID         string                    `json:"id"`
	TaskID     string                    `json:"taskId"`
	Content    string                    `json:"content"`
	TargetName string                    `json:"targetName"`
	Status     domain.ConfirmationStatus `json:"status"`
	DueAt      *time.Time                `json:"dueAt"`
}

func toConfirmationResponse(value domain.Confirmation) confirmationResponse {
	return confirmationResponse{
		ID: value.ID, TaskID: value.TaskID, Content: value.Content,
		TargetName: value.TargetName, Status: value.Status, DueAt: value.DueAt,
	}
}

func (h Handler) listConfirmations(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.Confirmations.List(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]confirmationResponse, 0, len(values))
	for _, value := range values {
		result = append(result, toConfirmationResponse(value))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h Handler) createConfirmation(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input createConfirmationRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.Confirmations.Create(r.Context(), userID, chi.URLParam(r, "taskId"), input.Content, input.TargetName, input.DueAt)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toConfirmationResponse(value))
}

func (h Handler) updateConfirmation(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input updateConfirmationRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.Confirmations.Transition(
		r.Context(), userID, chi.URLParam(r, "confirmationId"), input.Status, time.Now().UTC(),
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toConfirmationResponse(value))
}

type nameRequest struct {
	Name string `json:"name"`
}

type attachTaskTypeRequest struct {
	TaskTypeID string `json:"taskTypeId"`
}

type catalogResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func genreResponse(value domain.Genre) catalogResponse {
	return catalogResponse{ID: value.ID, Name: value.Name}
}

func taskTypeResponse(value domain.TaskType) catalogResponse {
	return catalogResponse{ID: value.ID, Name: value.Name}
}

func (h Handler) listGenres(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.Catalog.ListGenres(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]catalogResponse, 0, len(values))
	for _, value := range values {
		result = append(result, genreResponse(value))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h Handler) createGenre(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.Catalog.CreateGenre(r.Context(), userID, input.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, genreResponse(value))
}

func (h Handler) getGenre(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	value, err := h.Catalog.GetGenre(r.Context(), userID, chi.URLParam(r, "genreId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, genreResponse(value))
}

func (h Handler) renameGenre(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.Catalog.RenameGenre(r.Context(), userID, chi.URLParam(r, "genreId"), input.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, genreResponse(value))
}

func (h Handler) deleteGenre(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	if err := h.Catalog.DeleteGenre(r.Context(), userID, chi.URLParam(r, "genreId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) listTaskTypes(w http.ResponseWriter, r *http.Request) {
	h.writeTaskTypes(w, r, "")
}

func (h Handler) listGenreTaskTypes(w http.ResponseWriter, r *http.Request) {
	h.writeTaskTypes(w, r, chi.URLParam(r, "genreId"))
}

func (h Handler) writeTaskTypes(w http.ResponseWriter, r *http.Request, genreID string) {
	userID, _ := UserID(r.Context())
	var values []domain.TaskType
	var err error
	if genreID == "" {
		values, err = h.Catalog.ListTaskTypes(r.Context(), userID)
	} else {
		values, err = h.Catalog.ListGenreTaskTypes(r.Context(), userID, genreID)
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]catalogResponse, 0, len(values))
	for _, value := range values {
		result = append(result, taskTypeResponse(value))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h Handler) createTaskType(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.Catalog.CreateTaskType(r.Context(), userID, input.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, taskTypeResponse(value))
}

func (h Handler) getTaskType(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	value, err := h.Catalog.GetTaskType(r.Context(), userID, chi.URLParam(r, "taskTypeId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskTypeResponse(value))
}

func (h Handler) renameTaskType(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.Catalog.RenameTaskType(r.Context(), userID, chi.URLParam(r, "taskTypeId"), input.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskTypeResponse(value))
}

func (h Handler) deleteTaskType(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	if err := h.Catalog.DeleteTaskType(r.Context(), userID, chi.URLParam(r, "taskTypeId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) attachGenreTaskType(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input attachTaskTypeRequest
	if decodeJSON(w, r, &input) != nil || input.TaskTypeID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	if err := h.Catalog.AttachGenreTaskType(r.Context(), userID, chi.URLParam(r, "genreId"), input.TaskTypeID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	NewRouter(h).ServeHTTP(w, r)
}

func identityOverrideGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserID(r.Context()); !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}
		if r.URL.Query().Has("userId") || r.Header.Get("X-User-ID") != "" {
			writeError(w, http.StatusBadRequest, "IDENTITY_OVERRIDE_FORBIDDEN", "identity override forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h Handler) transitionTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input transitionRequest
	if err := decodeJSON(w, r, &input); err != nil || input.UserID != "" || input.ExpectedVersion < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	taskID := chi.URLParam(r, "taskId")
	impactToken := ""
	if input.ImpactToken != nil {
		impactToken = *input.ImpactToken
	}
	output, plan, err := h.Tasks.TransitionWithImpact(r.Context(), userID, taskID, input.ExpectedVersion, input.ToStatus, "WEB", impactToken, h.ImpactTokens)
	if errors.Is(err, domain.ErrImpactConfirmationRequired) {
		claims := application.ImpactClaims{UserID: userID, Operation: application.StatusTransitionImpactOperation(plan), ResourceID: taskID, Version: input.ExpectedVersion}
		token, issueErr := h.ImpactTokens.Issue(claims)
		if issueErr != nil {
			writeDomainError(w, issueErr)
			return
		}
		entities := make([]impactEntityResponse, 0, len(plan.Changes))
		for _, change := range plan.Changes {
			reason := "STATUS_ROLLBACK"
			if change.TaskID != taskID && change.ToStatus == domain.InProgress {
				reason = "ANCESTOR_REOPEN"
			}
			if change.TaskID != taskID && change.ToStatus == domain.NotStarted {
				reason = "DESCENDANT_RESET"
			}
			entities = append(entities, impactEntityResponse{ID: change.TaskID, Type: "TASK", Reason: stringPointer(reason)})
		}
		writeJSON(w, http.StatusConflict, impactPlanResponse{Code: "IMPACT_CONFIRMATION_REQUIRED", ImpactToken: token, AffectedEntities: entities, Warnings: []string{"ステータスを戻すと関連タスクの状態も変更される場合があります。"}})
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(output, time.Now().UTC()))
}

type taskResponse struct {
	ID                    string           `json:"id"`
	GenreID               string           `json:"genreId"`
	TaskTypeID            *string          `json:"taskTypeId"`
	ParentTaskID          *string          `json:"parentTaskId"`
	Title                 string           `json:"title"`
	Body                  *string          `json:"body"`
	Status                domain.Status    `json:"status"`
	Progress              *int             `json:"progress"`
	StartAt               *time.Time       `json:"startAt"`
	EndAt                 *time.Time       `json:"endAt"`
	IsAllDay              bool             `json:"isAllDay"`
	ManualPriority        *domain.Priority `json:"manualPriority"`
	EffectivePriority     domain.Priority  `json:"effectivePriority"`
	IsOverdue             bool             `json:"isOverdue"`
	ShowFire              bool             `json:"showFire"`
	Version               int64            `json:"version"`
	TaskScore             *int             `json:"taskScore,omitempty"`
	RecommendationReasons []string         `json:"recommendationReasons,omitempty"`
	Recommended           bool             `json:"recommended"`
}

func toTaskResponse(task domain.Task, now time.Time) taskResponse {
	effectivePriority := domain.MediumPriority
	if task.ManualPriority != nil {
		effectivePriority = *task.ManualPriority
	}
	overdue := task.Status != domain.Completed && task.EndAt != nil && task.EndAt.Before(now)
	return taskResponse{
		ID: task.ID, GenreID: task.GenreID, TaskTypeID: task.TaskTypeID,
		ParentTaskID: task.ParentTaskID, Title: task.Title, Body: task.Body,
		Status: task.Status, Progress: task.Progress, StartAt: task.StartAt, EndAt: task.EndAt,
		IsAllDay: task.IsAllDay, ManualPriority: task.ManualPriority,
		EffectivePriority: effectivePriority, IsOverdue: overdue, ShowFire: overdue, Version: task.Version,
	}
}

func (h Handler) getTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	value, err := h.Tasks.Get(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(value, time.Now().UTC()))
}

func (h Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	taskID := chi.URLParam(r, "taskId")
	expected, err := strconv.ParseInt(r.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil || expected < 1 {
		writeError(w, 400, "INVALID_REQUEST", "invalid expectedVersion")
		return
	}
	token := r.URL.Query().Get("impactToken")
	confirmed := false
	claims := application.ImpactClaims{UserID: userID, Operation: "DELETE_TASK", ResourceID: taskID, Version: expected}
	if token != "" {
		if h.ImpactTokens.Verify(token, claims) != nil {
			writeError(w, 409, "IMPACT_TOKEN_INVALID", "impact token is invalid or stale")
			return
		}
		confirmed = true
	}
	plan, deleteErr := h.Tasks.Delete(r.Context(), userID, taskID, expected, confirmed)
	if errors.Is(deleteErr, domain.ErrImpactConfirmationRequired) {
		issued, issueErr := h.ImpactTokens.Issue(claims)
		if issueErr != nil {
			writeDomainError(w, issueErr)
			return
		}
		entities := make([]impactEntityResponse, 0, len(plan.TaskIDs))
		for _, id := range plan.TaskIDs {
			entities = append(entities, impactEntityResponse{ID: id, Type: "TASK", Reason: stringPointer("SOFT_DELETE")})
		}
		writeJSON(w, 409, impactPlanResponse{Code: "IMPACT_CONFIRMATION_REQUIRED", ImpactToken: issued, AffectedEntities: entities, Warnings: []string{"これらのタスクも削除されます。よろしいですか？"}})
		return
	}
	if deleteErr != nil {
		writeDomainError(w, deleteErr)
		return
	}
	w.WriteHeader(204)
}

func (h Handler) searchTasks(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var priority *domain.Priority
	if value := strings.ToUpper(r.URL.Query().Get("priority")); value != "" {
		candidate := domain.Priority(value)
		if candidate != domain.HighPriority && candidate != domain.MediumPriority && candidate != domain.LowPriority {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid priority")
			return
		}
		priority = &candidate
	}
	genres := []string{}
	if value := r.URL.Query().Get("genres"); value != "" {
		for _, genre := range strings.Split(value, ",") {
			if genre = strings.TrimSpace(genre); genre != "" {
				genres = append(genres, genre)
			}
		}
	}
	values, err := h.Tasks.Search(r.Context(), userID, domain.TaskSearch{
		Query: r.URL.Query().Get("q"), GenreIDs: genres, Priority: priority,
		Deadline: r.URL.Query().Get("deadline"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectTaskResponses(values, time.Now().UTC()))
}

func (h Handler) getTaskTree(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.Tasks.Tree(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectTaskResponses(values, time.Now().UTC()))
}

func projectTaskResponses(tasks []domain.Task, now time.Time) []taskResponse {
	deadlineTasks := make([]domain.DeadlineTask, 0, len(tasks))
	for _, task := range tasks {
		parentID := ""
		if task.ParentTaskID != nil {
			parentID = *task.ParentTaskID
		}
		deadlineTasks = append(deadlineTasks, domain.DeadlineTask{ID: task.ID, ParentID: parentID, Status: task.Status, EndAt: task.EndAt})
	}
	deadlineStates, _ := domain.ProjectDeadlineState(deadlineTasks, now)
	byParent := map[string][]domain.Task{}
	for _, task := range tasks {
		if task.ParentTaskID != nil {
			byParent[*task.ParentTaskID] = append(byParent[*task.ParentTaskID], task)
		}
	}
	progressMemo := map[string]int{}
	var progress func(domain.Task) int
	progress = func(task domain.Task) int {
		if value, ok := progressMemo[task.ID]; ok {
			return value
		}
		children := byParent[task.ID]
		if len(children) == 0 {
			if task.Progress != nil {
				return *task.Progress
			}
			return 0
		}
		contributions := make([]domain.ProgressContribution, 0, len(children))
		for _, child := range children {
			value := progress(child)
			priority := domain.MediumPriority
			if child.ManualPriority != nil {
				priority = *child.ManualPriority
			}
			contributions = append(contributions, domain.ProgressContribution{Progress: &value, EffectivePriority: priority})
		}
		value, _ := domain.ParentProgress(task.Status, contributions)
		progressMemo[task.ID] = value
		return value
	}
	result := make([]taskResponse, 0, len(tasks))
	for _, task := range tasks {
		response := toTaskResponse(task, now)
		if task.HasChildren {
			value := progress(task)
			response.Progress = &value
		}
		state := deadlineStates[task.ID]
		response.IsOverdue, response.ShowFire = state.IsOverdue, state.ShowFire
		result = append(result, response)
	}
	return result
}

type createTaskRequest struct {
	Title          string           `json:"title"`
	Body           *string          `json:"body"`
	GenreID        string           `json:"genreId"`
	TaskTypeID     *string          `json:"taskTypeId"`
	ParentTaskID   *string          `json:"parentTaskId"`
	StartAt        *time.Time       `json:"startAt"`
	EndAt          *time.Time       `json:"endAt"`
	IsAllDay       *bool            `json:"isAllDay"`
	ManualPriority *domain.Priority `json:"manualPriority"`
	ImpactToken    *string          `json:"impactToken"`
}

type patchTaskRequest struct {
	ExpectedVersion int64                       `json:"expectedVersion"`
	Title           patchField[string]          `json:"title"`
	Body            patchField[string]          `json:"body"`
	GenreID         patchField[string]          `json:"genreId"`
	TaskTypeID      patchField[string]          `json:"taskTypeId"`
	ParentTaskID    patchField[string]          `json:"parentTaskId"`
	StartAt         patchField[time.Time]       `json:"startAt"`
	EndAt           patchField[time.Time]       `json:"endAt"`
	IsAllDay        patchField[bool]            `json:"isAllDay"`
	ManualPriority  patchField[domain.Priority] `json:"manualPriority"`
	ImpactToken     *string                     `json:"impactToken"`
}

// patchField distinguishes an omitted property from an explicit JSON null.
// This is required for nullable Task fields such as parentTaskId and endAt.
type patchField[T any] struct {
	Set   bool
	Value *T
}

func (field *patchField[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	if string(data) == "null" {
		field.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}

func (h Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	taskID := chi.URLParam(r, "taskId")
	var input patchTaskRequest
	if decodeJSON(w, r, &input) != nil || input.ExpectedVersion < 1 {
		writeError(w, 400, "INVALID_REQUEST", "invalid request")
		return
	}
	current, err := h.Tasks.Get(r.Context(), userID, taskID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if input.ExpectedVersion != current.Version {
		writeDomainError(w, domain.ErrVersionConflict)
		return
	}
	next, err := applyTaskPatch(current, input)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	tree, err := h.Tasks.Tree(r.Context(), userID, taskID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	changes, err := domain.PlanDateChange(toDateTasks(tree), taskID, next.StartAt, next.EndAt)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	needsDateImpact := dateImpactRequired(taskID, changes)
	genreChanged := next.GenreID != current.GenreID
	movePlan := domain.GenreMovePlan{}
	if genreChanged {
		movePlan, err = h.planGenreMove(r, userID, taskID, next.GenreID)
		if err != nil {
			var bad badRequestError
			if errors.As(err, &bad) {
				writeBadRequest(w, err)
				return
			}
			writeDomainError(w, err)
			return
		}
	}
	needsImpact := needsDateImpact || genreChanged
	operation, operationErr := taskUpdateImpactOperation(next.GenreID, movePlan, changes, genreChanged, needsDateImpact)
	if operationErr != nil {
		writeDomainError(w, operationErr)
		return
	}
	claims := application.ImpactClaims{UserID: userID, Operation: operation, ResourceID: taskID, Version: current.Version}
	if needsImpact && input.ImpactToken == nil {
		token, issueErr := h.ImpactTokens.Issue(claims)
		if issueErr != nil {
			writeDomainError(w, issueErr)
			return
		}
		writeJSON(w, 409, buildUpdateImpactPlan(token, changes, movePlan, needsDateImpact, genreChanged))
		return
	}
	if needsImpact && h.ImpactTokens.Verify(*input.ImpactToken, claims) != nil {
		writeError(w, 409, "IMPACT_TOKEN_INVALID", "impact token is invalid or stale")
		return
	}
	update := domain.TaskUpdate{Task: next, DateChanges: changes}
	if genreChanged {
		update.ApprovedGenreMove = &movePlan
	}
	updated, err := h.Tasks.Update(r.Context(), userID, update, input.ExpectedVersion)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, 200, toTaskResponse(updated, time.Now().UTC()))
}

func taskUpdateImpactOperation(genreID string, move domain.GenreMovePlan, dates []domain.DateChange, includeMove, includeDates bool) (string, error) {
	payload := struct {
		GenreID string               `json:"genreId,omitempty"`
		Move    domain.GenreMovePlan `json:"move,omitempty"`
		Dates   []domain.DateChange  `json:"dates,omitempty"`
	}{GenreID: genreID}
	if includeMove {
		payload.Move = move
	}
	if includeDates {
		payload.Dates = dates
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "UPDATE_TASK_IMPACT:" + hex.EncodeToString(digest[:]), nil
}

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
	authorization := domain.TaskCreateAuthorization{}
	if input.ParentTaskID != nil && input.ImpactToken != nil {
		parent, getErr := h.Tasks.Get(r.Context(), userID, *input.ParentTaskID)
		if getErr != nil {
			writeDomainError(w, getErr)
			return
		}
		claims := application.ImpactClaims{UserID: userID, Operation: "ADD_FIRST_CHILD", ResourceID: parent.ID, Version: parent.Version}
		if verifyErr := h.ImpactTokens.Verify(*input.ImpactToken, claims); verifyErr != nil {
			writeError(w, http.StatusConflict, "IMPACT_TOKEN_INVALID", "impact token is invalid or stale")
			return
		}
		authorization = domain.TaskCreateAuthorization{AllowLeafConversion: true, ParentVersion: parent.Version}
	}
	value, err := h.Tasks.Create(r.Context(), userID, draft, authorization)
	if err != nil {
		if errors.Is(err, domain.ErrImpactConfirmationRequired) && input.ParentTaskID != nil {
			parent, getErr := h.Tasks.Get(r.Context(), userID, *input.ParentTaskID)
			if getErr != nil {
				writeDomainError(w, getErr)
				return
			}
			token, issueErr := h.ImpactTokens.Issue(application.ImpactClaims{
				UserID: userID, Operation: "ADD_FIRST_CHILD", ResourceID: parent.ID, Version: parent.Version,
			})
			if issueErr != nil {
				writeDomainError(w, issueErr)
				return
			}
			writeJSON(w, http.StatusConflict, impactPlanResponse{
				Code: "IMPACT_CONFIRMATION_REQUIRED", ImpactToken: token,
				AffectedEntities: []impactEntityResponse{{ID: parent.ID, Type: "TASK", Title: &parent.Title, Reason: stringPointer("LEAF_TO_PARENT")}},
				Warnings:         []string{"進捗率は自動で再計算されます。子タスクを追加しますか？"},
			})
			return
		}
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTaskResponse(value, time.Now().UTC()))
}

type impactEntityResponse struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Title  *string `json:"title"`
	Reason *string `json:"reason"`
}

type impactPlanResponse struct {
	Code             string                 `json:"code"`
	ImpactToken      string                 `json:"impactToken"`
	AffectedEntities []impactEntityResponse `json:"affectedEntities"`
	Warnings         []string               `json:"warnings"`
}

func stringPointer(value string) *string { return &value }

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeDomainError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL_ERROR"
	switch {
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrInvalidConfirmationTransition),
		errors.Is(err, domain.ErrInvalidProgress), errors.Is(err, domain.ErrInvalidDateRange),
		errors.Is(err, domain.ErrNameRequired), errors.Is(err, domain.ErrInvalidConfirmationInput):
		status, code = http.StatusUnprocessableEntity, "DOMAIN_VALIDATION_FAILED"
	case errors.Is(err, domain.ErrVersionConflict), errors.Is(err, domain.ErrNotCompletable),
		errors.Is(err, domain.ErrImpactConfirmationRequired),
		errors.Is(err, domain.ErrResourceInUse), errors.Is(err, domain.ErrConflict),
		errors.Is(err, domain.ErrHierarchyCycle), errors.Is(err, domain.ErrDependencyCycle),
		errors.Is(err, domain.ErrGraphOwnership), errors.Is(err, domain.ErrGraphGenre):
		status, code = http.StatusConflict, "CONFLICT"
	case errors.Is(err, application.ErrInvalidImpactToken):
		status, code = http.StatusConflict, "IMPACT_TOKEN_INVALID"
	case errors.Is(err, domain.ErrImpactPlanChanged):
		status, code = http.StatusConflict, "IMPACT_TOKEN_INVALID"
	case errors.Is(err, application.ErrInvalidSettings):
		status, code = http.StatusBadRequest, "INVALID_REQUEST"
	}
	writeError(w, status, code, err.Error())
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
