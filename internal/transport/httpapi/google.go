package httpapi

import (
	"net/http"
)

func (h Handler) triggerGoogleSync(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	if err := h.GoogleJobs.Trigger(r.Context(), userID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h Handler) listGoogleConflicts(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.GoogleQueries.Conflicts(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (h Handler) listGoogleImportCandidates(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.GoogleQueries.ImportCandidates(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}
