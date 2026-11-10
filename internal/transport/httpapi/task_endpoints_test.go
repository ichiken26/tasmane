package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
)

// 内部認証付きリクエストの送信
func doInternal(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("X-Tasmane-Internal-Binding", "secret")
	request.Header.Set("X-Tasmane-Authenticated-User", "u")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// PUT /v1/tasks/{taskId}/progress は進行中の葉だけを5刻みで更新
func TestSetTaskProgressEndpoint(t *testing.T) {
	store := repository.NewMemory(domain.Task{ID: "leaf", UserID: "u", GenreID: "g", Title: "葉", Status: domain.InProgress, Version: 1})
	handler := InternalAuth("secret", NewRouter(Handler{Tasks: application.Tasks{Repo: store}}))
	if response := doInternal(handler, http.MethodPut, "/v1/tasks/leaf/progress", `{"progress":33,"expectedVersion":1}`); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid step status=%d body=%s", response.Code, response.Body.String())
	}
	response := doInternal(handler, http.MethodPut, "/v1/tasks/leaf/progress", `{"progress":60,"expectedVersion":1}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"progress":60`) || !strings.Contains(response.Body.String(), `"version":2`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response := doInternal(handler, http.MethodPut, "/v1/tasks/leaf/progress", `{"progress":65,"expectedVersion":1}`); response.Code != http.StatusConflict {
		t.Fatalf("stale version status=%d", response.Code)
	}
}

// 未完了前提タスクへの着手はHTTPでも409で確認を要求
func TestTransitionEndpointWarnsUnmetPrerequisite(t *testing.T) {
	store := repository.NewMemory(
		domain.Task{ID: "pre", UserID: "u", GenreID: "g", Title: "前提", Status: domain.NotStarted, Version: 1},
		domain.Task{ID: "task", UserID: "u", GenreID: "g", Title: "本体", Status: domain.NotStarted, Version: 1},
	)
	relations := &updateImpactRelationRepo{dependencies: []domain.Dependency{{TaskID: "task", DependsOnTaskID: "pre"}}}
	handler := InternalAuth("secret", NewRouter(Handler{
		Tasks: application.Tasks{Repo: store}, Relations: application.Relations{Repo: relations},
		ImpactTokens: application.ImpactTokens{Secret: []byte("01234567890123456789012345678901")},
	}))
	response := doInternal(handler, http.MethodPost, "/v1/tasks/task/transition", `{"toStatus":"IN_PROGRESS","expectedVersion":1}`)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "UNMET_DEPENDENCY") || !strings.Contains(response.Body.String(), `"title":"前提"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

// 不正な解決方法は400
func TestResolveGoogleConflictRejectsUnknownResolution(t *testing.T) {
	handler := InternalAuth("secret", NewRouter(Handler{}))
	if response := doInternal(handler, http.MethodPost, "/v1/google/conflicts/t/resolve", `{"resolution":"DROP_EVERYTHING"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

// アクセス時自動同期キュー
type dueQueue struct{ calls chan string }

func (q dueQueue) Enqueue(_ context.Context, job domain.GoogleJob) error {
	q.calls <- job.UserID
	return nil
}
func (dueQueue) AcquireDueGoogleSync(context.Context, string, time.Time, time.Duration) (bool, error) {
	return true, nil
}

// 認証済みGETで同期間隔超過ならプル同期を非同期投入（ユーザー単位で間引き）
func TestAutoGoogleSyncOnAccess(t *testing.T) {
	queue := dueQueue{calls: make(chan string, 4)}
	store := repository.NewMemory(domain.Task{ID: "t", UserID: "auto-user", GenreID: "g", Title: "T", Status: domain.NotStarted, Version: 1})
	handler := InternalAuth("secret", NewRouter(Handler{Tasks: application.Tasks{Repo: store}, GoogleJobs: application.GoogleJobs{Queue: queue}}))
	request := func() {
		req := httptest.NewRequest(http.MethodGet, "/v1/tasks/t", nil)
		req.Header.Set("X-Tasmane-Internal-Binding", "secret")
		req.Header.Set("X-Tasmane-Authenticated-User", "auto-user")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	request()
	request()
	select {
	case user := <-queue.calls:
		if user != "auto-user" {
			t.Fatalf("user=%s", user)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pull sync was not enqueued on access")
	}
	select {
	case <-queue.calls:
		t.Fatal("auto sync must be throttled per user")
	case <-time.After(200 * time.Millisecond):
	}
}
