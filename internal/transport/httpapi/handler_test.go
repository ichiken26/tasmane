package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
)

// カタログHTTPのメモリスタブ
type catalogHTTPStub struct {
	genres []domain.Genre
}

// ジャンル作成の記録
func (s *catalogHTTPStub) CreateGenre(_ context.Context, userID, name string) (domain.Genre, error) {
	genre := domain.Genre{ID: "g1", UserID: userID, Name: name}
	s.genres = append(s.genres, genre)
	return genre, nil
}

// 所有権付きジャンル取得
func (s *catalogHTTPStub) GetGenre(_ context.Context, userID, id string) (domain.Genre, error) {
	for _, genre := range s.genres {
		if genre.ID == id && genre.UserID == userID {
			return genre, nil
		}
	}
	return domain.Genre{}, domain.ErrForbidden
}

// ジャンル一覧の返却
func (s *catalogHTTPStub) ListGenres(context.Context, string) ([]domain.Genre, error) {
	return s.genres, nil
}

// ジャンル改名の固定返却
func (s *catalogHTTPStub) RenameGenre(_ context.Context, userID, id, name string) (domain.Genre, error) {
	return domain.Genre{ID: id, UserID: userID, Name: name}, nil
}

// ジャンル削除の成功
func (*catalogHTTPStub) DeleteGenre(context.Context, string, string) error { return nil }

// タスク種別作成の空返却
func (*catalogHTTPStub) CreateTaskType(context.Context, string, string) (domain.TaskType, error) {
	return domain.TaskType{}, nil
}

// タスク種別取得の空返却
func (*catalogHTTPStub) GetTaskType(context.Context, string, string) (domain.TaskType, error) {
	return domain.TaskType{}, nil
}

// タスク種別一覧の空返却
func (*catalogHTTPStub) ListTaskTypes(context.Context, string) ([]domain.TaskType, error) {
	return []domain.TaskType{}, nil
}

// ジャンル紐付けタスク種別一覧の空返却
func (*catalogHTTPStub) ListGenreTaskTypes(context.Context, string, string) ([]domain.TaskType, error) {
	return []domain.TaskType{}, nil
}

// タスク種別改名の空返却
func (*catalogHTTPStub) RenameTaskType(context.Context, string, string, string) (domain.TaskType, error) {
	return domain.TaskType{}, nil
}

// タスク種別削除の成功
func (*catalogHTTPStub) DeleteTaskType(context.Context, string, string) error { return nil }

// ジャンルへのタスク種別紐付けの成功
func (*catalogHTTPStub) AttachGenreTaskType(context.Context, string, string, string) error {
	return nil
}

// ジャンルからのタスク種別解除の成功
func (*catalogHTTPStub) DetachGenreTaskType(context.Context, string, string, string) error {
	return nil
}

// 認証付きテストサーバの組み立て
func testServer() http.Handler {
	r := repository.NewMemory(domain.Task{ID: "t1", UserID: "u1", Status: domain.NotStarted, Version: 1})
	return InternalAuth("secret", Handler{Tasks: application.Tasks{Repo: r}})
}

// 認証・身元隔離・版衝突の確認
func TestAuthenticationIdentityIsolationAndVersion(t *testing.T) {
	handler := testServer()
	for _, tc := range []struct {
		name, user, secret, body, query string
		want                            int
	}{
		{"missing binding", "u1", "", `{"toStatus":"IN_PROGRESS","expectedVersion":1}`, "", 401},
		{"body override", "u1", "secret", `{"toStatus":"IN_PROGRESS","expectedVersion":1,"userId":"u2"}`, "", 400},
		{"query override", "u1", "secret", `{"toStatus":"IN_PROGRESS","expectedVersion":1}`, "?userId=u2", 400},
		{"cross user", "u2", "secret", `{"toStatus":"IN_PROGRESS","expectedVersion":1}`, "", 404},
		{"success", "u1", "secret", `{"toStatus":"IN_PROGRESS","expectedVersion":1}`, "", 200},
		{"stale", "u1", "secret", `{"toStatus":"NOT_STARTED","expectedVersion":1}`, "", 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/tasks/t1/transition"+tc.query, strings.NewReader(tc.body))
			req.Header.Set("X-Tasmane-Authenticated-User", tc.user)
			if tc.secret != "" {
				req.Header.Set("X-Tasmane-Internal-Binding", tc.secret)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// 注入身元とcamelCase DTOのジャンルルート確認
func TestChiGenreRoutesUseInjectedIdentityAndCamelCaseDTO(t *testing.T) {
	stub := &catalogHTTPStub{}
	handler := InternalAuth("secret", NewRouter(Handler{Catalog: application.Catalog{Repo: stub}}))
	// ジャンル作成リクエストの送信
	request := httptest.NewRequest(http.MethodPost, "/v1/genres", strings.NewReader(`{"name":"Work"}`))
	request.Header.Set("X-Tasmane-Internal-Binding", "secret")
	request.Header.Set("X-Tasmane-Authenticated-User", "u1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created["id"] != "g1" || created["name"] != "Work" || created["UserID"] != nil {
		t.Fatalf("unexpected DTO: %#v", created)
	}

	// ジャンル一覧リクエストの送信
	request = httptest.NewRequest(http.MethodGet, "/v1/genres", nil)
	request.Header.Set("X-Tasmane-Internal-Binding", "secret")
	request.Header.Set("X-Tasmane-Authenticated-User", "u1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"Work"`) {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
}
