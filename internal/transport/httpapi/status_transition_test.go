package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
)

func TestStatusRollbackHTTPPreviewConfirmAndStaleToken(t *testing.T) {
	parentID := "parent"
	store := repository.NewMemory(
		domain.Task{ID: parentID, UserID: "u", Status: domain.Completed, HasChildren: true, Version: 2},
		domain.Task{ID: "leaf", UserID: "u", ParentTaskID: &parentID, Status: domain.Completed, Progress: httpStatusInt(100), Version: 4},
	)
	handler := InternalAuth("secret", NewRouter(Handler{
		Tasks:        application.Tasks{Repo: store},
		ImpactTokens: application.ImpactTokens{Secret: []byte("01234567890123456789012345678901")},
	}))
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/tasks/leaf/transition", strings.NewReader(body))
		req.Header.Set("X-Tasmane-Internal-Binding", "secret")
		req.Header.Set("X-Tasmane-Authenticated-User", "u")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	preview := request(`{"toStatus":"IN_PROGRESS","expectedVersion":4}`)
	if preview.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", preview.Code, preview.Body.String())
	}
	var impact impactPlanResponse
	if err := json.Unmarshal(preview.Body.Bytes(), &impact); err != nil {
		t.Fatal(err)
	}
	if impact.Code != "IMPACT_CONFIRMATION_REQUIRED" || impact.ImpactToken == "" || len(impact.AffectedEntities) != 2 {
		t.Fatalf("impact=%+v", impact)
	}
	confirmed := request(`{"toStatus":"IN_PROGRESS","expectedVersion":4,"impactToken":"` + impact.ImpactToken + `"}`)
	if confirmed.Code != http.StatusOK || store.Tasks[parentID].Status != domain.InProgress {
		t.Fatalf("status=%d body=%s parent=%+v", confirmed.Code, confirmed.Body.String(), store.Tasks[parentID])
	}
	stale := request(`{"toStatus":"IN_PROGRESS","expectedVersion":4,"impactToken":"` + impact.ImpactToken + `"}`)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"IMPACT_TOKEN_INVALID"`) {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}
}

func httpStatusInt(value int) *int { return &value }
