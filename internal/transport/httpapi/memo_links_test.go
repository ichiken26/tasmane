package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
)

// メモリンクの登録・一覧・削除（同一URLは重複させずタイトルだけ更新）
func TestTaskMemoLinkEndpoints(t *testing.T) {
	store := repository.NewMemory(
		domain.Task{ID: "task", UserID: "u", GenreID: "g", Title: "本体", Status: domain.NotStarted, Version: 1},
		domain.Task{ID: "other", UserID: "someone", GenreID: "g", Title: "他人", Status: domain.NotStarted, Version: 1},
	)
	handler := InternalAuth("secret", NewRouter(Handler{Tasks: application.Tasks{Repo: store}}))

	created := doInternal(handler, http.MethodPost, "/v1/tasks/task/memo-links", `{"url":"https://memo.kokage-studio.com/memo/a","title":"初版"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var link struct{ ID, TaskID, URL, Title string }
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil || link.ID == "" || link.TaskID != "task" || link.Title != "初版" {
		t.Fatalf("link=%+v err=%v", link, err)
	}
	again := doInternal(handler, http.MethodPost, "/v1/tasks/task/memo-links", `{"url":"https://memo.kokage-studio.com/memo/a","title":"改題"}`)
	if again.Code != http.StatusCreated || !strings.Contains(again.Body.String(), link.ID) || !strings.Contains(again.Body.String(), "改題") {
		t.Fatalf("upsert status=%d body=%s", again.Code, again.Body.String())
	}
	listed := doInternal(handler, http.MethodGet, "/v1/tasks/task/memo-links", "")
	if listed.Code != http.StatusOK || strings.Count(listed.Body.String(), `"id"`) != 1 || !strings.Contains(listed.Body.String(), "改題") {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}
	if response := doInternal(handler, http.MethodPost, "/v1/tasks/task/memo-links", `{"url":"javascript:alert(1)"}`); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid url status=%d", response.Code)
	}
	if response := doInternal(handler, http.MethodPost, "/v1/tasks/other/memo-links", `{"url":"https://memo.kokage-studio.com/memo/b"}`); response.Code != http.StatusNotFound && response.Code != http.StatusForbidden {
		t.Fatalf("other user's task status=%d", response.Code)
	}
	if response := doInternal(handler, http.MethodDelete, "/v1/tasks/task/memo-links/"+link.ID, ""); response.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	if listed := doInternal(handler, http.MethodGet, "/v1/tasks/task/memo-links", ""); strings.TrimSpace(listed.Body.String()) != "[]" {
		t.Fatalf("after delete body=%s", listed.Body.String())
	}
}

// 設定のメモアプリ連携フラグは部分更新でき、他の設定を変えない
func TestSettingsMemoIntegrationFlag(t *testing.T) {
	repo := &settingsMemoStub{value: domain.UserSettings{Timezone: "Asia/Tokyo", GoogleSyncIntervalMinutes: 10}}
	handler := InternalAuth("secret", NewRouter(Handler{Settings: application.Settings{Repo: repo}}))
	if response := doInternal(handler, http.MethodGet, "/v1/settings", ""); !strings.Contains(response.Body.String(), `"memoIntegrationEnabled":false`) {
		t.Fatalf("get body=%s", response.Body.String())
	}
	response := doInternal(handler, http.MethodPatch, "/v1/settings", `{"memoIntegrationEnabled":true}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"memoIntegrationEnabled":true`) || !repo.value.MemoIntegrationEnabled || repo.value.GoogleSyncIntervalMinutes != 10 {
		t.Fatalf("patch status=%d body=%s value=%+v", response.Code, response.Body.String(), repo.value)
	}
}

type settingsMemoStub struct{ value domain.UserSettings }

func (s *settingsMemoStub) GetUserSettings(context.Context, string) (domain.UserSettings, error) {
	return s.value, nil
}
func (s *settingsMemoStub) UpdateUserSettings(_ context.Context, _ string, next domain.UserSettings) (domain.UserSettings, error) {
	s.value = next
	return next, nil
}
