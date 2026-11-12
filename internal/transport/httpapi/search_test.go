package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
	"time"
)

// 検索条件を記録するタスク読み出しスタブ
type searchRepositoryStub struct {
	*repository.Memory
	tasks    []domain.Task
	mu       sync.Mutex
	searches []domain.TaskSearch
}

// 検索条件の記録とジャンル絞り込み
func (s *searchRepositoryStub) Search(_ context.Context, _ string, search domain.TaskSearch) ([]domain.Task, error) {
	// 一致判定と投影は並行に呼ばれる
	s.mu.Lock()
	s.searches = append(s.searches, search)
	s.mu.Unlock()
	result := []domain.Task{}
	for _, task := range s.tasks {
		if len(search.GenreIDs) == 0 || containsString(search.GenreIDs, task.GenreID) {
			result = append(result, task)
		}
	}
	return result, nil
}

// ツリー取得の空返却
func (*searchRepositoryStub) Tree(context.Context, string, string) ([]domain.Task, error) {
	return nil, nil
}

// 依存なしのグラフスタブ
type emptyRelationStub struct{}

func (emptyRelationStub) LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error) {
	return nil, nil, nil
}
func (emptyRelationStub) ListTaskDependencies(context.Context, string, string) ([]domain.Dependency, error) {
	return nil, nil
}
func (emptyRelationStub) SetParent(context.Context, string, string, string) error { return nil }
func (emptyRelationStub) AddDependency(context.Context, string, domain.Dependency, int64) error {
	return nil
}
func (emptyRelationStub) RemoveDependency(context.Context, string, domain.Dependency) error {
	return nil
}

// 確認事項なしのスタブ
type emptyConfirmationStub struct{}

func (emptyConfirmationStub) CreateConfirmation(context.Context, string, string, string, string, *time.Time) (domain.Confirmation, error) {
	return domain.Confirmation{}, nil
}
func (emptyConfirmationStub) GetConfirmation(context.Context, string, string) (domain.Confirmation, error) {
	return domain.Confirmation{}, nil
}
func (emptyConfirmationStub) ListConfirmations(context.Context, string, string) ([]domain.Confirmation, error) {
	return []domain.Confirmation{}, nil
}
func (emptyConfirmationStub) UpdateConfirmation(context.Context, string, string, domain.Confirmation) error {
	return nil
}

// 文字列スライスの包含判定
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// 期限切れ・期限なし・手動LOWの3タスクを持つ検索サーバ
func searchTestServer() (http.Handler, *searchRepositoryStub) {
	past := time.Now().UTC().Add(-48 * time.Hour)
	soon := time.Now().UTC().Add(72 * time.Hour)
	low := domain.LowPriority
	stub := &searchRepositoryStub{Memory: repository.NewMemory(), tasks: []domain.Task{
		{ID: "no-deadline", UserID: "u1", GenreID: "g1", Title: "期限なし", Status: domain.NotStarted, Version: 1},
		{ID: "overdue", UserID: "u1", GenreID: "g1", Title: "期限切れ", Status: domain.NotStarted, EndAt: &past, Version: 1},
		{ID: "manual-low", UserID: "u1", GenreID: "g2", Title: "手動LOW", Status: domain.NotStarted, EndAt: &soon, ManualPriority: &low, Version: 1},
	}}
	handler := InternalAuth("secret", NewRouter(Handler{
		Tasks:         application.Tasks{Repo: stub},
		Relations:     application.Relations{Repo: emptyRelationStub{}},
		Confirmations: application.Confirmations{Repo: emptyConfirmationStub{}},
	}))
	return handler, stub
}

// 検索リクエストの送信
func searchRequest(t *testing.T, handler http.Handler, query string) (*httptest.ResponseRecorder, []taskResponse) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/tasks"+query, nil)
	request.Header.Set("X-Tasmane-Internal-Binding", "secret")
	request.Header.Set("X-Tasmane-Authenticated-User", "u1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var tasks []taskResponse
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &tasks); err != nil {
			t.Fatal(err)
		}
	}
	return response, tasks
}

// 応答タスクIDの抽出
func responseIDs(tasks []taskResponse) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

// OpenAPIのgenreIds（form, explode=false）を検索条件へ渡す
func TestSearchTasksReadsGenreIDsQueryParameter(t *testing.T) {
	handler, stub := searchTestServer()
	response, tasks := searchRequest(t, handler, "?genreIds=g1,g2")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, search := range stub.searches {
		if got := search.GenreIDs; !reflect.DeepEqual(got, []string{"g1", "g2"}) {
			t.Fatalf("genreIds not forwarded: %#v", got)
		}
	}
	if len(tasks) != 3 {
		t.Fatalf("expected OR match across genres, got %v", responseIDs(tasks))
	}
	_, onlyG2 := searchRequest(t, handler, "?genreIds=g2")
	if !reflect.DeepEqual(responseIDs(onlyG2), []string{"manual-low"}) {
		t.Fatalf("genre filter not applied: %v", responseIDs(onlyG2))
	}
}

// OpenAPI enum外のdeadline/sort/priorityは400
func TestSearchTasksRejectsValuesOutsideOpenAPIEnums(t *testing.T) {
	handler, _ := searchTestServer()
	for _, query := range []string{"?deadline=yesterday", "?sort=title", "?priority=urgent"} {
		response, _ := searchRequest(t, handler, query)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: status=%d body=%s", query, response.Code, response.Body.String())
		}
	}
	for _, query := range []string{"?deadline=overdue", "?deadline=later", "?sort=taskScore", "?sort=priority", "?sort=deadline"} {
		response, _ := searchRequest(t, handler, query)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", query, response.Code, response.Body.String())
		}
	}
}

// 検索結果のeffectivePriorityはmanualPriority ?? calculatedPriority
func TestSearchTasksUsesCalculatedEffectivePriority(t *testing.T) {
	handler, _ := searchTestServer()
	_, tasks := searchRequest(t, handler, "")
	got := map[string]domain.Priority{}
	for _, task := range tasks {
		got[task.ID] = task.EffectivePriority
	}
	want := map[string]domain.Priority{"overdue": domain.HighPriority, "no-deadline": domain.MediumPriority, "manual-low": domain.LowPriority}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effectivePriority=%v want %v", got, want)
	}
	_, high := searchRequest(t, handler, "?priority=HIGH")
	if !reflect.DeepEqual(responseIDs(high), []string{"overdue"}) {
		t.Fatalf("priority filter must use effectivePriority: %v", responseIDs(high))
	}
}

// sortパラメータによる並び順
func TestSearchTasksSortsByRequestedOrder(t *testing.T) {
	handler, _ := searchTestServer()
	for _, tc := range []struct {
		sort string
		want []string
	}{
		{"priority", []string{"overdue", "no-deadline", "manual-low"}},
		{"deadline", []string{"overdue", "manual-low", "no-deadline"}},
		{"taskScore", []string{"overdue", "no-deadline", "manual-low"}},
	} {
		_, tasks := searchRequest(t, handler, "?sort="+tc.sort)
		if got := responseIDs(tasks); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("sort=%s got %v want %v", tc.sort, got, tc.want)
		}
	}
}
