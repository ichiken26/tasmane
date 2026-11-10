package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// バインディング認証なしでの生存確認成功
func TestHealthzDoesNotRequireBindingAuth(t *testing.T) {
	// 認証必須ルートを後ろに置いたルータ組み立て
	mux := http.NewServeMux()
	(Health{}).Register(mux)
	mux.Handle("/", InternalAuth("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	// 生存確認リクエストの送信
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
