package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"tasmane/internal/application"
	"tasmane/internal/domain"
)

type Handler struct{ Tasks application.Tasks }
type transitionRequest struct {
	ToStatus        domain.Status `json:"toStatus"`
	ExpectedVersion int64         `json:"expectedVersion"`
	UserID          string        `json:"userId,omitempty"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.URL.Query().Has("userId") || r.Header.Get("X-User-ID") != "" {
		http.Error(w, "identity override forbidden", 400)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method != http.MethodPost || len(parts) != 4 || parts[0] != "v1" || parts[1] != "tasks" || parts[3] != "transition" {
		http.NotFound(w, r)
		return
	}
	var in transitionRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || in.UserID != "" || in.ExpectedVersion < 1 {
		http.Error(w, "invalid request", 400)
		return
	}
	out, err := h.Tasks.Transition(r.Context(), userID, parts[2], in.ExpectedVersion, in.ToStatus, "WEB")
	if err != nil {
		status := 500
		if errors.Is(err, domain.ErrVersionConflict) || errors.Is(err, domain.ErrNotCompletable) {
			status = 409
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			status = 422
		}
		if errors.Is(err, domain.ErrForbidden) {
			status = 404
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
