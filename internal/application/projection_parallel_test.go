package application

import (
	"context"
	"sync"
	"testing"
	"time"

	"tasmane/internal/domain"
)

// 3つの読み出しが同時に走っているときだけ先へ進める待ち合わせ
type rendezvous struct {
	wg      sync.WaitGroup
	release chan struct{}
	once    sync.Once
}

func newRendezvous(n int) *rendezvous {
	r := &rendezvous{release: make(chan struct{})}
	r.wg.Add(n)
	go func() { r.wg.Wait(); close(r.release) }()
	return r
}

// 到着を記録し、全員がそろうまで待つ（直列実行ならそろわずタイムアウト）
func (r *rendezvous) arrive(ctx context.Context) error {
	r.wg.Done()
	select {
	case <-r.release:
		return nil
	case <-time.After(2 * time.Second):
		return context.DeadlineExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
}

type parallelTaskRepo struct {
	Repository
	meet  *rendezvous
	tasks []domain.Task
}

func (r parallelTaskRepo) Search(ctx context.Context, _ string, _ domain.TaskSearch) ([]domain.Task, error) {
	return r.tasks, r.meet.arrive(ctx)
}
func (r parallelTaskRepo) Tree(context.Context, string, string) ([]domain.Task, error) {
	return nil, nil
}

type parallelGraphRepo struct {
	RelationRepository
	meet *rendezvous
}

func (r parallelGraphRepo) LoadGraph(ctx context.Context, userID string) ([]domain.GraphTask, []domain.Dependency, error) {
	return []domain.GraphTask{{ID: "t1", UserID: userID, GenreID: "g"}}, nil, r.meet.arrive(ctx)
}

type parallelConfirmationRepo struct {
	ConfirmationRepository
	meet  *rendezvous
	genre []string
}

func (r *parallelConfirmationRepo) ListConfirmationsForGenres(ctx context.Context, _ string, genreIDs []string) (map[string][]domain.Confirmation, error) {
	r.genre = genreIDs
	return map[string][]domain.Confirmation{"t1": {{ID: "c1", TaskID: "t1"}}}, r.meet.arrive(ctx)
}

// 投影の入力（タスク・依存グラフ・確認事項）は互いに独立なので同時に読み出す
func TestProjectReadsInputsConcurrently(t *testing.T) {
	meet := newRendezvous(3)
	confirmations := &parallelConfirmationRepo{meet: meet}
	projections := Projections{
		Tasks:         Tasks{Repo: parallelTaskRepo{meet: meet, tasks: []domain.Task{{ID: "t1", UserID: "u", GenreID: "g", Title: "A", Status: domain.NotStarted, Version: 1}}}},
		Relations:     Relations{Repo: parallelGraphRepo{meet: meet}},
		Confirmations: Confirmations{Repo: confirmations},
	}
	result, err := projections.Project(context.Background(), "u", []string{"g"})
	if err != nil {
		t.Fatalf("inputs must be read concurrently: %v", err)
	}
	if len(result.Tasks) != 1 || len(result.Confirmations["t1"]) != 1 || len(confirmations.genre) != 1 || confirmations.genre[0] != "g" {
		t.Fatalf("result=%+v genres=%v", result, confirmations.genre)
	}
}

// 検索の一致判定と、同ジャンル全体の投影は独立なので同時に読み出す
func TestSearchMatchesAndProjectsConcurrently(t *testing.T) {
	meet := newRendezvous(2)
	tasks := []domain.Task{{ID: "t1", UserID: "u", GenreID: "g", Title: "A", Status: domain.NotStarted, Version: 1}}
	projections := Projections{Tasks: Tasks{Repo: parallelTaskRepo{meet: meet, tasks: tasks}}}
	values, err := projections.Search(context.Background(), "u", ProjectedSearch{Query: "A"})
	if err != nil || len(values) != 1 {
		t.Fatalf("search must run match and projection concurrently: values=%v err=%v", values, err)
	}
}
