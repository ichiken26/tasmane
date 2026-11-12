package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"
	"tasmane/internal/application"
	"tasmane/internal/domain"
)

// HTTP APIハンドラの依存注入
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

// ステータス遷移のリクエスト本体
type transitionRequest struct {
	ToStatus        domain.Status `json:"toStatus"`
	ExpectedVersion int64         `json:"expectedVersion"`
	ImpactToken     *string       `json:"impactToken,omitempty"`
	UserID          string        `json:"userId,omitempty"`
}

// エラー応答
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HTTP APIルータの組み立て
func NewRouter(handler Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(identityOverrideGuard)
	router.Use(handler.autoGoogleSync)
	router.Post("/v1/tasks/{taskId}/transition", handler.transitionTask)
	router.Get("/v1/tasks/{taskId}", handler.getTask)
	router.Patch("/v1/tasks/{taskId}", handler.updateTask)
	router.Delete("/v1/tasks/{taskId}", handler.deleteTask)
	router.Post("/v1/tasks", handler.createTask)
	router.Get("/v1/tasks", handler.searchTasks)
	router.Get("/v1/tasks/{taskId}/tree", handler.getTaskTree)
	router.Put("/v1/tasks/{taskId}/progress", handler.setTaskProgress)
	router.Get("/v1/tasks/{taskId}/memo-links", handler.listMemoLinks)
	router.Post("/v1/tasks/{taskId}/memo-links", handler.addMemoLink)
	router.Delete("/v1/tasks/{taskId}/memo-links/{linkId}", handler.deleteMemoLink)
	router.Get("/v1/deleted-tasks", handler.listDeletedTasks)
	router.Post("/v1/deleted-tasks/{taskId}/restore", handler.restoreTask)
	router.Get("/v1/tasks/{taskId}/dependencies", handler.listDependencies)
	router.Post("/v1/tasks/{taskId}/dependencies", handler.addDependency)
	router.Delete("/v1/tasks/{taskId}/dependencies/{dependsOnTaskId}", handler.removeDependency)
	router.Get("/v1/settings", handler.getSettings)
	router.Patch("/v1/settings", handler.updateSettings)
	router.Get("/v1/tasks/{taskId}/confirmations", handler.listConfirmations)
	router.Post("/v1/tasks/{taskId}/confirmations", handler.createConfirmation)
	router.Patch("/v1/confirmations/{confirmationId}", handler.updateConfirmation)
	router.Get("/v1/confirmation-targets", handler.listConfirmationTargets)
	router.Post("/v1/confirmation-targets", handler.createConfirmationTarget)
	router.Delete("/v1/confirmation-targets/{targetId}", handler.deleteConfirmationTarget)
	router.Get("/v1/genres", handler.listGenres)
	router.Post("/v1/genres", handler.createGenre)
	router.Get("/v1/genres/{genreId}", handler.getGenre)
	router.Patch("/v1/genres/{genreId}", handler.renameGenre)
	router.Delete("/v1/genres/{genreId}", handler.deleteGenre)
	router.Get("/v1/genres/{genreId}/task-types", handler.listGenreTaskTypes)
	router.Post("/v1/genres/{genreId}/task-types", handler.attachGenreTaskType)
	router.Delete("/v1/genres/{genreId}/task-types/{taskTypeId}", handler.detachGenreTaskType)
	router.Get("/v1/task-types", handler.listTaskTypes)
	router.Post("/v1/task-types", handler.createTaskType)
	router.Get("/v1/task-types/{taskTypeId}", handler.getTaskType)
	router.Patch("/v1/task-types/{taskTypeId}", handler.renameTaskType)
	router.Delete("/v1/task-types/{taskTypeId}", handler.deleteTaskType)
	router.Get("/v1/dashboard", handler.getDashboard)
	router.Get("/v1/dashboard/genres/{genreId}", handler.getGenreDashboard)
	router.Post("/v1/google/sync", handler.triggerGoogleSync)
	router.Get("/v1/google/conflicts", handler.listGoogleConflicts)
	router.Get("/v1/google/status", handler.getGoogleStatus)
	router.Post("/v1/google/task-list/recreate", handler.recreateGoogleTaskList)
	router.Get("/v1/google/import-candidates", handler.listGoogleImportCandidates)
	router.Post("/v1/google/conflicts/{taskId}/resolve", handler.resolveGoogleConflict)
	router.Post("/v1/google/import-candidates/import", handler.importGoogleCandidate)
	// 未定義パスの404応答
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	// 未許可メソッドの405応答
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})
	return router
}

// ダッシュボードの応答
type dashboardResponse struct {
	Tasks         []taskResponse                  `json:"tasks"`
	Confirmations []dashboardConfirmationResponse `json:"confirmations"`
	Advice        *adviceResponse                 `json:"advice"`
}

// 助言の応答
type adviceResponse struct {
	Content     string    `json:"content"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// 全体ダッシュボードの取得
func (h Handler) getDashboard(w http.ResponseWriter, r *http.Request) { h.writeDashboard(w, r, nil) }

// ジャンル限定ダッシュボードの取得
func (h Handler) getGenreDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genreId")
	h.writeDashboard(w, r, []string{id})
}

// ダッシュボード投影の組み立てと応答
func (h Handler) writeDashboard(w http.ResponseWriter, r *http.Request, genres []string) {
	userID, _ := UserID(r.Context())
	now := time.Now().UTC()
	var genreID *string
	if len(genres) == 1 {
		genreID = &genres[0]
	}
	// タスク投影と助言は独立なので並行に読み出す
	var (
		scored scoredTasks
		advice *adviceResponse
	)
	group, ctx := errgroup.WithContext(r.Context())
	group.Go(func() (err error) {
		scored, err = h.scoreTasks(ctx, userID, genres, now)
		return err
	})
	if h.Advices.Repo != nil {
		group.Go(func() error {
			current, err := h.Advices.Current(ctx, userID, genreID)
			if err == nil {
				advice = &adviceResponse{Content: current.Content, GeneratedAt: current.GeneratedAt}
			}
			if errors.Is(err, domain.ErrForbidden) {
				return nil
			}
			return err
		})
	}
	if err := group.Wait(); err != nil {
		writeDomainError(w, err)
		return
	}
	titles := make(map[string]string, len(scored.tasks))
	for _, task := range scored.tasks {
		titles[task.ID] = task.Title
	}
	recommendedConfirmations, err := projectDashboardConfirmations(scored.responses, scored.confirmations, titles, now)
	reply(w, http.StatusOK, dashboardResponse{Tasks: scored.responses, Confirmations: recommendedConfirmations, Advice: advice}, err)
}

// スコア付きタスク投影
type scoredTasks struct {
	tasks         []domain.Task
	responses     []taskResponse
	confirmations map[string][]domain.Confirmation
}

// 実効優先度・TaskScore付きタスク投影の算出（ダッシュボードと検索で共有）
func (h Handler) scoreTasks(ctx context.Context, userID string, genres []string, _ time.Time) (scoredTasks, error) {
	result, err := h.projections().Project(ctx, userID, genres)
	if err != nil {
		return scoredTasks{}, err
	}
	tasks := make([]domain.Task, 0, len(result.Tasks))
	for _, value := range result.Tasks {
		tasks = append(tasks, value.Task)
	}
	return scoredTasks{tasks: tasks, responses: toProjectedResponses(result.Tasks), confirmations: result.Confirmations}, nil
}

// 依存関係追加のリクエスト本体
type dependencyRequest struct {
	DependsOnTaskID string `json:"dependsOnTaskId"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

// 依存関係の応答
type dependencyResponse struct {
	TaskID          string `json:"taskId"`
	DependsOnTaskID string `json:"dependsOnTaskId"`
}

// タスク依存関係一覧の取得
func (h Handler) listDependencies(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// 依存関係一覧ユースケースの呼び出し
	values, err := h.Relations.ListDependencies(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]dependencyResponse, 0, len(values))
	for _, value := range values {
		result = append(result, dependencyResponse{TaskID: value.TaskID, DependsOnTaskID: value.DependsOnTaskID})
	}
	// 依存関係一覧レスポンスの書き出し
	writeJSON(w, http.StatusOK, result)
}

// タスク依存関係の追加
func (h Handler) addDependency(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input dependencyRequest
	if decodeJSON(w, r, &input) != nil || input.DependsOnTaskID == "" || input.ExpectedVersion < 1 {
		badRequest(w)
		return
	}
	// 依存関係追加ユースケースの呼び出し
	reply(w, http.StatusCreated, nil, h.Relations.AddDependency(r.Context(), userID, chi.URLParam(r, "taskId"), input.DependsOnTaskID, input.ExpectedVersion))
}

// タスク依存関係の削除
func (h Handler) removeDependency(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	expectedVersion, err := strconv.ParseInt(r.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil || expectedVersion < 1 {
		badRequest(w)
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
	// 依存関係削除ユースケースの呼び出し
	reply(w, http.StatusNoContent, nil, h.Relations.RemoveDependency(r.Context(), userID, taskID, chi.URLParam(r, "dependsOnTaskId")))
}

// ユーザー設定の応答
type settingsResponse struct {
	Timezone                  string `json:"timezone"`
	GoogleSyncIntervalMinutes int    `json:"googleSyncIntervalMinutes"`
	MemoIntegrationEnabled    bool   `json:"memoIntegrationEnabled"`
}

// ユーザー設定の部分更新リクエスト
type patchSettingsRequest struct {
	Timezone                  *string `json:"timezone"`
	GoogleSyncIntervalMinutes *int    `json:"googleSyncIntervalMinutes"`
	MemoIntegrationEnabled    *bool   `json:"memoIntegrationEnabled"`
}

// ユーザー設定の取得
func (h Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// 設定取得ユースケースの呼び出し
	settings, err := h.Settings.Get(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// 設定レスポンスの書き出し
	writeJSON(w, http.StatusOK, settingsResponse{
		Timezone: settings.Timezone, GoogleSyncIntervalMinutes: settings.GoogleSyncIntervalMinutes, MemoIntegrationEnabled: settings.MemoIntegrationEnabled,
	})
}

// ユーザー設定の更新
func (h Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	current, err := h.Settings.Get(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// リクエストJSONのデコード
	var input patchSettingsRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	if input.Timezone != nil {
		current.Timezone = *input.Timezone
	}
	if input.GoogleSyncIntervalMinutes != nil {
		current.GoogleSyncIntervalMinutes = *input.GoogleSyncIntervalMinutes
	}
	if input.MemoIntegrationEnabled != nil {
		current.MemoIntegrationEnabled = *input.MemoIntegrationEnabled
	}
	// 設定更新ユースケースの呼び出し
	saved, err := h.Settings.Update(r.Context(), userID, current)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// 設定レスポンスの書き出し
	writeJSON(w, http.StatusOK, settingsResponse{
		Timezone: saved.Timezone, GoogleSyncIntervalMinutes: saved.GoogleSyncIntervalMinutes, MemoIntegrationEnabled: saved.MemoIntegrationEnabled,
	})
}

// 確認事項作成のリクエスト本体
type createConfirmationRequest struct {
	Content    string     `json:"content"`
	TargetName string     `json:"targetName"`
	DueAt      *time.Time `json:"dueAt"`
}

// 確認事項更新のリクエスト本体
type updateConfirmationRequest struct {
	Status domain.ConfirmationStatus `json:"status"`
}

// 確認事項の応答
type confirmationResponse struct {
	ID         string                    `json:"id"`
	TaskID     string                    `json:"taskId"`
	Content    string                    `json:"content"`
	TargetName string                    `json:"targetName"`
	Status     domain.ConfirmationStatus `json:"status"`
	DueAt      *time.Time                `json:"dueAt"`
}

// 確認事項ドメインから応答への変換
func toConfirmationResponse(value domain.Confirmation) confirmationResponse {
	return confirmationResponse{
		ID: value.ID, TaskID: value.TaskID, Content: value.Content,
		TargetName: value.TargetName, Status: value.Status, DueAt: value.DueAt,
	}
}

// タスク確認事項一覧の取得
func (h Handler) listConfirmations(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// 確認事項一覧ユースケースの呼び出し
	values, err := h.Confirmations.List(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]confirmationResponse, 0, len(values))
	for _, value := range values {
		result = append(result, toConfirmationResponse(value))
	}
	// 確認事項一覧レスポンスの書き出し
	writeJSON(w, http.StatusOK, result)
}

// 確認事項の作成
func (h Handler) createConfirmation(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input createConfirmationRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	// 確認事項作成ユースケースの呼び出し
	value, err := h.Confirmations.Create(r.Context(), userID, chi.URLParam(r, "taskId"), input.Content, input.TargetName, input.DueAt)
	reply(w, http.StatusCreated, toConfirmationResponse(value), err)
}

// 確認事項ステータスの更新
func (h Handler) updateConfirmation(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input updateConfirmationRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	// 確認事項遷移ユースケースの呼び出し
	value, err := h.Confirmations.Transition(
		r.Context(), userID, chi.URLParam(r, "confirmationId"), input.Status, time.Now().UTC(),
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// 更新レスポンスの書き出し
	writeJSON(w, http.StatusOK, toConfirmationResponse(value))
}

// 名称指定のリクエスト本体
type nameRequest struct {
	Name string `json:"name"`
}

// ジャンルへのタスク種別紐付けリクエスト
type attachTaskTypeRequest struct {
	TaskTypeID string `json:"taskTypeId"`
}

// カタログ項目の応答
type catalogResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ジャンルから応答への変換
func genreResponse(value domain.Genre) catalogResponse {
	return catalogResponse{ID: value.ID, Name: value.Name}
}

// タスク種別から応答への変換
func taskTypeResponse(value domain.TaskType) catalogResponse {
	return catalogResponse{ID: value.ID, Name: value.Name}
}

// ジャンル一覧の取得
func (h Handler) listGenres(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// ジャンル一覧ユースケースの呼び出し
	values, err := h.Catalog.ListGenres(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]catalogResponse, 0, len(values))
	for _, value := range values {
		result = append(result, genreResponse(value))
	}
	// ジャンル一覧レスポンスの書き出し
	writeJSON(w, http.StatusOK, result)
}

// ジャンルの作成
func (h Handler) createGenre(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	// ジャンル作成ユースケースの呼び出し
	value, err := h.Catalog.CreateGenre(r.Context(), userID, input.Name)
	reply(w, http.StatusCreated, genreResponse(value), err)
}

// ジャンルの取得
func (h Handler) getGenre(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// ジャンル取得ユースケースの呼び出し
	value, err := h.Catalog.GetGenre(r.Context(), userID, chi.URLParam(r, "genreId"))
	reply(w, http.StatusOK, genreResponse(value), err)
}

// ジャンル名の変更
func (h Handler) renameGenre(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	// ジャンル改名ユースケースの呼び出し
	value, err := h.Catalog.RenameGenre(r.Context(), userID, chi.URLParam(r, "genreId"), input.Name)
	reply(w, http.StatusOK, genreResponse(value), err)
}

// ジャンルの削除
func (h Handler) deleteGenre(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// ジャンル削除ユースケースの呼び出し
	reply(w, http.StatusNoContent, nil, h.Catalog.DeleteGenre(r.Context(), userID, chi.URLParam(r, "genreId")))
}

// 全タスク種別一覧の取得
func (h Handler) listTaskTypes(w http.ResponseWriter, r *http.Request) {
	h.writeTaskTypes(w, r, "")
}

// ジャンル紐付けタスク種別一覧の取得
func (h Handler) listGenreTaskTypes(w http.ResponseWriter, r *http.Request) {
	h.writeTaskTypes(w, r, chi.URLParam(r, "genreId"))
}

// タスク種別一覧の取得と応答
func (h Handler) writeTaskTypes(w http.ResponseWriter, r *http.Request, genreID string) {
	// 認証ユーザーの取得
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
	// タスク種別一覧レスポンスの書き出し
	writeJSON(w, http.StatusOK, result)
}

// タスク種別の作成
func (h Handler) createTaskType(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	// タスク種別作成ユースケースの呼び出し
	value, err := h.Catalog.CreateTaskType(r.Context(), userID, input.Name)
	reply(w, http.StatusCreated, taskTypeResponse(value), err)
}

// タスク種別の取得
func (h Handler) getTaskType(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// タスク種別取得ユースケースの呼び出し
	value, err := h.Catalog.GetTaskType(r.Context(), userID, chi.URLParam(r, "taskTypeId"))
	reply(w, http.StatusOK, taskTypeResponse(value), err)
}

// タスク種別名の変更
func (h Handler) renameTaskType(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input nameRequest
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	// タスク種別改名ユースケースの呼び出し
	value, err := h.Catalog.RenameTaskType(r.Context(), userID, chi.URLParam(r, "taskTypeId"), input.Name)
	reply(w, http.StatusOK, taskTypeResponse(value), err)
}

// タスク種別の削除
func (h Handler) deleteTaskType(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// タスク種別削除ユースケースの呼び出し
	reply(w, http.StatusNoContent, nil, h.Catalog.DeleteTaskType(r.Context(), userID, chi.URLParam(r, "taskTypeId")))
}

// ジャンルへのタスク種別紐付け
func (h Handler) attachGenreTaskType(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// リクエストJSONのデコード
	var input attachTaskTypeRequest
	if decodeJSON(w, r, &input) != nil || input.TaskTypeID == "" {
		badRequest(w)
		return
	}
	// 紐付けユースケースの呼び出し
	reply(w, http.StatusNoContent, nil, h.Catalog.AttachGenreTaskType(r.Context(), userID, chi.URLParam(r, "genreId"), input.TaskTypeID))
}

// ジャンルからのタスク種別解除（使用中は禁止）
func (h Handler) detachGenreTaskType(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	reply(w, http.StatusNoContent, nil, h.Catalog.DetachGenreTaskType(r.Context(), userID, chi.URLParam(r, "genreId"), chi.URLParam(r, "taskTypeId")))
}

// ルータへのHTTP委譲
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	NewRouter(h).ServeHTTP(w, r)
}

// クエリ・ヘッダによる身元上書きの拒否
func identityOverrideGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 認証ユーザーの確認
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

// タスクステータスの遷移

// タスクの応答
type taskResponse = application.TaskView

// タスク詳細の取得
func (h Handler) getTask(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// タスク取得ユースケースの呼び出し
	value, err := h.Tasks.Get(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// タスクレスポンスの書き出し
	h.writeTask(w, r, http.StatusOK, value)
}

// タスクの削除

// タスク検索
func (h Handler) searchTasks(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	query := r.URL.Query()
	var priority *domain.Priority
	if value := strings.ToUpper(query.Get("priority")); value != "" {
		candidate := domain.Priority(value)
		priority = &candidate
	}
	// genreIds（style=form, explode=false）の解析。繰り返し指定も受理
	genres := []string{}
	for _, value := range query["genreIds"] {
		for _, genre := range strings.Split(value, ",") {
			if genre = strings.TrimSpace(genre); genre != "" {
				genres = append(genres, genre)
			}
		}
	}
	deadline, order := query.Get("deadline"), query.Get("sort")
	if (deadline != "" && !application.SearchDeadlines[deadline]) || (order != "" && !application.SearchSorts[order]) ||
		(priority != nil && *priority != domain.HighPriority && *priority != domain.MediumPriority && *priority != domain.LowPriority) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid search parameter")
		return
	}
	// 実効優先度・TaskScore反映済み検索ユースケースの呼び出し
	values, err := h.projections().Search(r.Context(), userID, application.ProjectedSearch{
		Query: query.Get("q"), GenreIDs: genres, Priority: priority, Deadline: deadline, Sort: order,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// 検索結果レスポンスの書き出し
	writeJSON(w, http.StatusOK, toProjectedResponses(values))
}

// タスクツリーの取得
func (h Handler) getTaskTree(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// ツリー取得ユースケースの呼び出し
	values, err := h.Tasks.Tree(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if len(values) == 0 {
		writeJSON(w, http.StatusOK, []taskResponse{})
		return
	}
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	// ツリーレスポンスの書き出し（同一ジャンル全体で算出）
	projected, err := h.projections().ProjectIDs(r.Context(), userID, []string{values[0].GenreID}, ids)
	reply(w, http.StatusOK, toProjectedResponses(projected), err)
}

// タスク作成のリクエスト本体
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

// タスク部分更新のリクエスト本体
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

// 省略と明示的nullを区別するPATCH欄
type patchField[T any] struct {
	Set   bool
	Value *T
}

// JSON nullと値の区別付きデコード
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

// タスクの部分更新

// 更新影響トークン用操作識別子の算出

// タスクの作成

// 影響対象エンティティの応答
type impactEntityResponse struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Title  *string `json:"title"`
	Reason *string `json:"reason"`
}

// 影響確認プランの応答
type impactPlanResponse struct {
	Code             string                 `json:"code"`
	ImpactToken      string                 `json:"impactToken"`
	AffectedEntities []impactEntityResponse `json:"affectedEntities"`
	Warnings         []string               `json:"warnings"`
}

// 文字列ポインタの生成
func stringPointer(value string) *string { return &value }

// 単一JSON値のデコード
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

// ドメインエラーのHTTP変換
func writeDomainError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL_ERROR"
	switch {
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrInvalidConfirmationTransition),
		errors.Is(err, domain.ErrInvalidProgress), errors.Is(err, domain.ErrInvalidDateRange),
		errors.Is(err, domain.ErrNameRequired), errors.Is(err, domain.ErrInvalidConfirmationInput),
		errors.Is(err, domain.ErrCompletedParent), errors.Is(err, domain.ErrInvalidPriority),
		errors.Is(err, domain.ErrConfirmationOnCompletedTask), errors.Is(err, domain.ErrInvalidMemoLink):
		status, code = http.StatusUnprocessableEntity, "DOMAIN_VALIDATION_FAILED"
	case errors.Is(err, domain.ErrInvalidTaskInput), errors.Is(err, domain.ErrInvalidDatePropagation),
		errors.Is(err, domain.ErrInvalidGraphInput):
		status, code = http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, domain.ErrGoogleHierarchyConflict):
		status, code = http.StatusConflict, "CONFLICT"
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
	// 想定外エラーの詳細（SQL等）はクライアントへ返さずログのみ
	if status == http.StatusInternalServerError {
		log.Printf("internal error: %v", err)
		writeError(w, status, code, "internal error")
		return
	}
	writeError(w, status, code, err.Error())
}

// エラー応答の書き出し
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

// JSON応答の書き出し
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// 処理結果の応答（エラーはドメインエラーへ変換。値が nil ならステータスのみ）
func reply(w http.ResponseWriter, status int, value any, err error) {
	switch {
	case err != nil:
		writeDomainError(w, err)
	case value == nil:
		w.WriteHeader(status)
	default:
		writeJSON(w, status, value)
	}
}

// 不正リクエストの応答
func badRequest(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
}
