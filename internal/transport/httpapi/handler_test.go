package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
)

func testServer() http.Handler {
	r := repository.NewMemory(domain.Task{ID: "t1", UserID: "u1", Status: domain.NotStarted, Version: 1})
	return InternalAuth("secret", Handler{Tasks: application.Tasks{Repo: r}})
}
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
