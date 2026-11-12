package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"tasmane/internal/application"
)

// メモリンクのユースケース（Tasksのリポジトリが実装していれば有効）
func (h Handler) memoLinks() application.MemoLinks {
	repository, _ := h.Tasks.Repo.(application.MemoLinkRepository)
	return application.MemoLinks{Repo: repository}
}

// メモリンク一覧
func (h Handler) listMemoLinks(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	links, err := h.memoLinks().List(r.Context(), userID, chi.URLParam(r, "taskId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]application.MemoLinkView, 0, len(links))
	for _, link := range links {
		result = append(result, application.NewMemoLinkView(link))
	}
	writeJSON(w, http.StatusOK, result)
}

// メモリンクの登録
func (h Handler) addMemoLink(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	var input struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if decodeJSON(w, r, &input) != nil {
		badRequest(w)
		return
	}
	link, err := h.memoLinks().Add(r.Context(), userID, chi.URLParam(r, "taskId"), input.URL, input.Title)
	reply(w, http.StatusCreated, application.NewMemoLinkView(link), err)
}

// メモリンクの解除
func (h Handler) deleteMemoLink(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserID(r.Context())
	reply(w, http.StatusNoContent, nil, h.memoLinks().Remove(r.Context(), userID, chi.URLParam(r, "taskId"), chi.URLParam(r, "linkId")))
}
