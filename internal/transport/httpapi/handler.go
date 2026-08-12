package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	ImpactTokens  application.ImpactTokens
}

type transitionRequest struct {
	ToStatus        domain.Status `json:"toStatus"`
	ExpectedVersion int64         `json:"expectedVersion"`
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
	router.Post("/v1/tasks", handler.createTask)
	router.Get("/v1/tasks", handler.searchTasks)
	router.Get("/v1/tasks/{taskId}/tree", handler.getTaskTree)
	router.Get("/v1/tasks/{taskId}/dependencies", handler.listDependencies)
	router.Post("/v1/tasks/{taskId}/dependencies", handler.addDependency)
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
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})
	return router
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
	output, err := h.Tasks.Transition(r.Context(), userID, chi.URLParam(r, "taskId"), input.ExpectedVersion, input.ToStatus, "WEB")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(output, time.Now().UTC()))
}

type taskResponse struct {
	ID                string           `json:"id"`
	GenreID           string           `json:"genreId"`
	TaskTypeID        *string          `json:"taskTypeId"`
	ParentTaskID      *string          `json:"parentTaskId"`
	Title             string           `json:"title"`
	Body              *string          `json:"body"`
	Status            domain.Status    `json:"status"`
	Progress          *int             `json:"progress"`
	StartAt           *time.Time       `json:"startAt"`
	EndAt             *time.Time       `json:"endAt"`
	IsAllDay          bool             `json:"isAllDay"`
	ManualPriority    *domain.Priority `json:"manualPriority"`
	EffectivePriority domain.Priority  `json:"effectivePriority"`
	IsOverdue         bool             `json:"isOverdue"`
	ShowFire          bool             `json:"showFire"`
	Version           int64            `json:"version"`
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
