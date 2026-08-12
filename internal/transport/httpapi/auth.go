package httpapi

import (
	"context"
	"net/http"
)

type contextKey string

const userKey contextKey = "authenticated-user"

func InternalAuth(bindingSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bindingSecret == "" || r.Header.Get("X-Tasmane-Internal-Binding") != bindingSecret || r.Header.Get("X-Tasmane-Authenticated-User") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, r.Header.Get("X-Tasmane-Authenticated-User"))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func UserID(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(userKey).(string)
	return value, ok && value != ""
}
