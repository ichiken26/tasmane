package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
)

// 指定数の読み出しが同時に走ったときだけ進める待ち合わせ（直列ならタイムアウト）
type meeting struct {
	wg      sync.WaitGroup
	release chan struct{}
}

func newMeeting(n int) *meeting {
	m := &meeting{release: make(chan struct{})}
	m.wg.Add(n)
	go func() { m.wg.Wait(); close(m.release) }()
	return m
}

func (m *meeting) arrive() error {
	m.wg.Done()
	select {
	case <-m.release:
		return nil
	case <-time.After(2 * time.Second):
		return context.DeadlineExceeded
	}
}

type meetingTasks struct {
	*repository.Memory
	meet *meeting
}

func (r meetingTasks) Search(ctx context.Context, userID string, search domain.TaskSearch) ([]domain.Task, error) {
	if err := r.meet.arrive(); err != nil {
		return nil, err
	}
	return r.Memory.Search(ctx, userID, search)
}

type meetingAdvice struct {
	application.AdviceRepository
	meet *meeting
}

func (r meetingAdvice) LatestAdvice(context.Context, string, *string) (domain.Advice, error) {
	return domain.Advice{Content: "先に見積書を", GeneratedAt: time.Now()}, r.meet.arrive()
}

// ダッシュボードの助言はタスク投影と独立なので同時に読み出す
func TestDashboardReadsAdviceConcurrently(t *testing.T) {
	meet := newMeeting(2)
	store := repository.NewMemory(domain.Task{ID: "t", UserID: "u", GenreID: "g", Title: "見積書", Status: domain.NotStarted, Version: 1})
	handler := InternalAuth("secret", NewRouter(Handler{
		Tasks:   application.Tasks{Repo: meetingTasks{Memory: store, meet: meet}},
		Advices: application.Advices{Repo: meetingAdvice{meet: meet}},
	}))
	response := doInternal(handler, http.MethodGet, "/v1/dashboard", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "先に見積書を") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
