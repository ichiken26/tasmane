package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tasmane/internal/application"
	"tasmane/internal/domain"
)

type googleHTTPStore struct{ jobs []domain.GoogleJob }

func (*googleHTTPStore) AcquireGoogleSyncLock(context.Context, string, time.Time, time.Duration) (bool, error) {
	return true, nil
}
func (*googleHTTPStore) MarkSyncing(context.Context, domain.GoogleJob) error          { return nil }
func (*googleHTTPStore) MarkSynced(context.Context, domain.GoogleJob) error           { return nil }
func (*googleHTTPStore) MarkFailed(context.Context, domain.GoogleJob, string) error   { return nil }
func (*googleHTTPStore) MarkConflict(context.Context, domain.GoogleJob, string) error { return nil }

func (s *googleHTTPStore) Enqueue(_ context.Context, job domain.GoogleJob) error {
	s.jobs = append(s.jobs, job)
	return nil
}
func (*googleHTTPStore) ListGoogleConflicts(context.Context, string) ([]domain.GoogleConflictRecord, error) {
	return []domain.GoogleConflictRecord{{TaskID: "t1", Code: "DUE_CONFLICT"}}, nil
}
func (*googleHTTPStore) ListGoogleImportCandidates(context.Context, string) ([]domain.GoogleImportCandidate, error) {
	return []domain.GoogleImportCandidate{{GoogleTaskListID: "l1", GoogleTaskID: "gt1"}}, nil
}

func TestGoogleRoutesUseAuthenticatedUser(t *testing.T) {
	store := &googleHTTPStore{}
	router := InternalAuth("secret", NewRouter(Handler{
		GoogleJobs:    application.GoogleJobs{Queue: store},
		GoogleQueries: application.GoogleQueries{Repo: store},
	}))
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-Tasmane-Internal-Binding", "secret")
		req.Header.Set("X-Tasmane-Authenticated-User", "u1")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	if response := request(http.MethodPost, "/v1/google/sync"); response.Code != http.StatusAccepted {
		t.Fatalf("sync status=%d body=%s", response.Code, response.Body.String())
	}
	if len(store.jobs) != 1 || store.jobs[0].UserID != "u1" || store.jobs[0].Action != domain.GooglePullUser {
		t.Fatalf("unexpected jobs: %#v", store.jobs)
	}
	if response := request(http.MethodGet, "/v1/google/conflicts"); response.Code != http.StatusOK {
		t.Fatalf("conflicts status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/v1/google/import-candidates"); response.Code != http.StatusOK {
		t.Fatalf("candidates status=%d body=%s", response.Code, response.Body.String())
	}
}
