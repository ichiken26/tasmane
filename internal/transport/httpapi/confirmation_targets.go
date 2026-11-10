package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"tasmane/internal/application"
)

// 確認先プリセットのユースケース（Confirmationsのリポジトリが実装していれば有効）
func (h Handler) confirmationTargets() application.ConfirmationTargets {
	repository, _ := h.Confirmations.Repo.(application.ConfirmationTargetRepository)
	return application.ConfirmationTargets{Repo: repository}
}

// 確認先プリセット一覧
func (h Handler) listConfirmationTargets(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	values, err := h.confirmationTargets().List(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]application.NamedView, 0, len(values))
	for _, value := range values {
		result = append(result, application.NamedView{ID: value.ID, Name: value.Name})
	}
	writeJSON(w, http.StatusOK, result)
}

// 確認先プリセットの登録
func (h Handler) createConfirmationTarget(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input struct {
		Name string `json:"name"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	value, err := h.confirmationTargets().Create(r.Context(), userID, input.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, application.NamedView{ID: value.ID, Name: value.Name})
}

// 確認先プリセットの削除
func (h Handler) deleteConfirmationTarget(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	if err := h.confirmationTargets().Delete(r.Context(), userID, chi.URLParam(r, "targetId")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
