package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
)

type contextKey string

const userKey contextKey = "authenticated-user"

func InternalAuth(bindingSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providedSecret := r.Header.Get("X-Tasmane-Internal-Binding")
		secretMatches := bindingSecret != "" && len(providedSecret) == len(bindingSecret) && subtle.ConstantTimeCompare([]byte(providedSecret), []byte(bindingSecret)) == 1
		if !secretMatches || r.Header.Get("X-Tasmane-Authenticated-User") == "" {
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
