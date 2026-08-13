package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAuthBootstrapStore struct {
	input authBootstrapRequest
	err   error
}

func (f *fakeAuthBootstrapStore) Bootstrap(_ context.Context, input authBootstrapRequest) (string, error) {
	f.input = input
	return "01989a84-b454-7abc-8def-0123456789ab", f.err
}

func TestAuthBootstrapRequiresBindingAndValidatedIdentity(t *testing.T) {
	store := &fakeAuthBootstrapStore{}
	handler := newAuthBootstrapHandler(store, "01234567890123456789012345678901")

	unauthorized := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"user@example.com"}`))
	unauthorizedResult := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResult, unauthorized)
	if unauthorizedResult.Code != http.StatusUnauthorized {
		t.Fatalf("missing binding status = %d", unauthorizedResult.Code)
	}

	invalid := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"not-an-email"}`))
	invalid.Header.Set("X-Tasmane-Internal-Binding", "01234567890123456789012345678901")
	invalidResult := httptest.NewRecorder()
	handler.ServeHTTP(invalidResult, invalid)
	if invalidResult.Code != http.StatusBadRequest {
		t.Fatalf("invalid identity status = %d", invalidResult.Code)
	}

	valid := httptest.NewRequest(http.MethodPost, "/internal/auth/bootstrap", strings.NewReader(`{"googleSubject":"g1","email":"user@example.com","refreshToken":"secret"}`))
	valid.Header.Set("X-Tasmane-Internal-Binding", "01234567890123456789012345678901")
	validResult := httptest.NewRecorder()
	handler.ServeHTTP(validResult, valid)
	if validResult.Code != http.StatusOK || store.input.RefreshToken != "secret" || strings.Contains(validResult.Body.String(), "secret") {
		t.Fatalf("valid response = %d %s", validResult.Code, validResult.Body.String())
	}
}

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
