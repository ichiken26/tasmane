package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"tasmane/internal/application"
	"tasmane/internal/domain"
)

// Google同期ジョブの投入
func (h Handler) triggerGoogleSync(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// 同期トリガーユースケースの呼び出し
	if err := h.GoogleJobs.Trigger(r.Context(), userID); err != nil {
		writeDomainError(w, err)
		return
	}
	// 受理レスポンスの書き出し
	w.WriteHeader(http.StatusAccepted)
}

// Google同期衝突一覧の取得
func (h Handler) listGoogleConflicts(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// 衝突一覧ユースケースの呼び出し
	values, err := h.GoogleQueries.Conflicts(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// 衝突一覧レスポンスの書き出し
	writeJSON(w, http.StatusOK, values)
}

// Google取込候補一覧の取得
func (h Handler) listGoogleImportCandidates(w http.ResponseWriter, r *http.Request) {
	// 認証ユーザーの取得
	userID, _ := UserID(r.Context())
	// 取込候補ユースケースの呼び出し
	values, err := h.GoogleQueries.ImportCandidates(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// 取込候補レスポンスの書き出し
	writeJSON(w, http.StatusOK, values)
}

// コンフリクト解決のリクエスト本体
type resolveConflictRequest struct {
	Resolution domain.GoogleResolution `json:"resolution"`
}

// Google同期コンフリクトの解決
func (h Handler) resolveGoogleConflict(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input resolveConflictRequest
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	if err := h.GoogleQueries.Resolve(r.Context(), userID, chi.URLParam(r, "taskId"), input.Resolution); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// 取込候補取込のリクエスト本体
type importCandidateRequest struct {
	GoogleTaskListID string `json:"googleTaskListId"`
	GoogleTaskID     string `json:"googleTaskId"`
	GenreID          string `json:"genreId"`
}

// Google-only Taskの取込
func (h Handler) importGoogleCandidate(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input importCandidateRequest
	if decodeJSON(w, r, &input) != nil || input.GoogleTaskListID == "" || input.GoogleTaskID == "" || input.GenreID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	repository, ok := h.GoogleQueries.Repo.(application.GoogleImportRepository)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "google import unavailable")
		return
	}
	imports := application.GoogleImports{Tasks: h.Tasks, Settings: h.Settings, Repo: repository}
	task, err := imports.Import(r.Context(), userID, input.GoogleTaskListID, input.GoogleTaskID, input.GenreID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	h.writeTask(w, r, http.StatusCreated, task)
}

// アクセス時自動同期の判定間隔（ユーザー単位・プロセス内）
const autoSyncCheckInterval = time.Minute

var autoSyncChecks = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

// 認証済みアクセス時、最終同期から設定間隔以上ならGoogle→タスマネ同期を非同期投入
func (h Handler) autoGoogleSync(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserID(r.Context())
		if ok && r.Method == http.MethodGet && h.GoogleJobs.Queue != nil && shouldCheckAutoSync(userID, time.Now()) {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
			go func() {
				defer cancel()
				if _, err := h.GoogleJobs.TriggerIfDue(ctx, userID); err != nil {
					log.Printf("auto google sync: %v", err)
				}
			}()
		}
		next.ServeHTTP(w, r)
	})
}

// 判定間隔内の重複チェック抑止
func shouldCheckAutoSync(userID string, now time.Time) bool {
	autoSyncChecks.Lock()
	defer autoSyncChecks.Unlock()
	if last, ok := autoSyncChecks.last[userID]; ok && now.Sub(last) < autoSyncCheckInterval {
		return false
	}
	autoSyncChecks.last[userID] = now
	return true
}

// Google連携の同期状態
func (h Handler) getGoogleStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	status, err := h.GoogleQueries.Status(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// 専用 Task List の作り直し（復旧フロー）
func (h Handler) recreateGoogleTaskList(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	if err := h.GoogleQueries.ResetTaskList(r.Context(), userID); err != nil {
		writeDomainError(w, err)
		return
	}
	// 新しいリストの作成と再作成ジョブの消費を促す
	if err := h.GoogleJobs.Trigger(r.Context(), userID); err != nil && !errors.Is(err, domain.ErrConflict) {
		log.Printf("trigger google sync after list reset: %v", err)
	}
	w.WriteHeader(http.StatusAccepted)
}
