package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 認証ブートストラップの固定応答スタブ
type fakeAuthBootstrapStore struct {
	input authBootstrapRequest
	err   error
}

// 投入内容の記録と固定ユーザーID返却
func (f *fakeAuthBootstrapStore) Bootstrap(_ context.Context, input authBootstrapRequest) (string, error) {
	f.input = input
	return "01989a84-b454-7abc-8def-0123456789ab", f.err
}

// バインディング必須と身元検証の確認
func TestAuthBootstrapRequiresBindingAndValidatedIdentity(t *testing.T) {
	store := &fakeAuthBootstrapStore{}
	handler := newAuthBootstrapHandler(store, "01234567890123456789012345678901")

	// バインディング欠落の拒否
	unauthorized := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"user@example.com"}`))
	unauthorizedResult := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResult, unauthorized)
	if unauthorizedResult.Code != http.StatusUnauthorized {
		t.Fatalf("missing binding status = %d", unauthorizedResult.Code)
	}

	// 不正メールの拒否
	invalid := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"not-an-email"}`))
	invalid.Header.Set("X-Tasmane-Internal-Binding", "01234567890123456789012345678901")
	invalidResult := httptest.NewRecorder()
	handler.ServeHTTP(invalidResult, invalid)
	if invalidResult.Code != http.StatusBadRequest {
		t.Fatalf("invalid identity status = %d", invalidResult.Code)
	}

	// 正規身元のユーザー払い出し
	valid := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"user@example.com","refreshToken":"secret"}`))
	valid.Header.Set("X-Tasmane-Internal-Binding", "01234567890123456789012345678901")
	validResult := httptest.NewRecorder()
	handler.ServeHTTP(validResult, valid)
	if validResult.Code != http.StatusOK || store.input.RefreshToken != "secret" || strings.Contains(validResult.Body.String(), "secret") {
		t.Fatalf("valid response = %d %s", validResult.Code, validResult.Body.String())
	}
}

// ストア内部エラーの非漏洩
func TestAuthBootstrapDoesNotLeakStoreErrors(t *testing.T) {
	store := &fakeAuthBootstrapStore{err: errors.New("database password in internal error")}
	handler := newAuthBootstrapHandler(store, "01234567890123456789012345678901")
	request := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"user@example.com"}`))
	request.Header.Set("X-Tasmane-Internal-Binding", "01234567890123456789012345678901")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusInternalServerError || strings.Contains(result.Body.String(), "password") {
		t.Fatalf("response leaks internal error: %d %s", result.Code, result.Body.String())
	}
}
