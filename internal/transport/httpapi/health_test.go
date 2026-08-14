package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzDoesNotRequireBindingAuth(t *testing.T) {
	mux := http.NewServeMux()
	(Health{}).Register(mux)
	mux.Handle("/", InternalAuth("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
