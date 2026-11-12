package application

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/errgroup"

	"tasmane/internal/domain"
)

// 確認事項のジャンル単位一括取得（タスク一覧を待たずに並行取得できる。未実装ならタスク単位へフォールバック）
type ConfirmationGenreRepository interface {
	ListConfirmationsForGenres(context.Context, string, []string) (map[string][]domain.Confirmation, error)
}

// 算出値付きタスク投影のユースケース（HTTP/MCP/WebMCP共通）
type Projections struct {
	Tasks         Tasks
	Relations     Relations
	Confirmations Confirmations
	Now           func() time.Time
}

// 投影結果と入力（確認事項はダッシュボードの推薦にも再利用）
type ProjectionResult struct {
	Tasks         []domain.TaskProjection
	Confirmations map[string][]domain.Confirmation
	Now           time.Time
}

// 指定ジャンル（空なら全体）のタスクを投影
func (p Projections) Project(ctx context.Context, userID string, genreIDs []string) (ProjectionResult, error) {
	if userID == "" {
		return ProjectionResult{}, domain.ErrForbidden
	}
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	// 投影の入力（タスク・依存グラフ・確認事項）を並行に読み出す（遠隔DBの往復を1回分に）
	var (
		tasks         []domain.Task
		dependencies  []domain.Dependency
		confirmations map[string][]domain.Confirmation
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		tasks, err = p.Tasks.Search(groupCtx, userID, domain.TaskSearch{GenreIDs: genreIDs})
		return err
	})
	if p.Relations.Repo != nil {
		group.Go(func() (err error) {
			_, dependencies, err = p.Relations.Snapshot(groupCtx, userID)
			return err
		})
	}
	batch, batched := p.Confirmations.Repo.(ConfirmationGenreRepository)
	if batched {
		group.Go(func() (err error) {
			confirmations, err = batch.ListConfirmationsForGenres(groupCtx, userID, genreIDs)
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return ProjectionResult{}, err
	}
	if !batched {
		var err error
		if confirmations, err = p.loadConfirmations(ctx, userID, tasks); err != nil {
			return ProjectionResult{}, err
		}
	}
	// 共通ドメイン投影の実行
	projected, err := domain.ProjectTasks(domain.ProjectionInput{Tasks: tasks, Dependencies: dependencies, Confirmations: confirmations, Now: now})
	if err != nil {
		return ProjectionResult{}, err
	}
	return ProjectionResult{Tasks: projected, Confirmations: confirmations, Now: now}, nil
}

// 単一タスクの投影（同一ジャンル全体で算出し階層・依存を反映）
func (p Projections) ProjectTask(ctx context.Context, userID string, task domain.Task) (domain.TaskProjection, error) {
	result, err := p.Project(ctx, userID, []string{task.GenreID})
	if err != nil && !errors.Is(err, ErrTaskReadUnavailable) {
		return domain.TaskProjection{}, err
	}
	for _, value := range result.Tasks {
		if value.Task.ID == task.ID {
			return value, nil
		}
	}
	// 読み出し境界がない構成では単体で投影
	alone, err := domain.ProjectTasks(domain.ProjectionInput{Tasks: []domain.Task{task}, Now: result.nowOr(p.Now)})
	if err != nil {
		return domain.TaskProjection{}, err
	}
	return alone[0], nil
}

// 投影時刻の解決
func (r ProjectionResult) nowOr(now func() time.Time) time.Time {
	if !r.Now.IsZero() {
		return r.Now
	}
	if now != nil {
		return now().UTC()
	}
	return time.Now().UTC()
}

// 指定ID集合の投影（入力順を保持）
func (p Projections) ProjectIDs(ctx context.Context, userID string, genreIDs []string, ids []string) ([]domain.TaskProjection, error) {
	result, err := p.Project(ctx, userID, genreIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.TaskProjection, len(result.Tasks))
	for _, value := range result.Tasks {
		byID[value.Task.ID] = value
	}
	values := make([]domain.TaskProjection, 0, len(ids))
	for _, id := range ids {
		if value, ok := byID[id]; ok {
			values = append(values, value)
		}
	}
	return values, nil
}

// 確認事項の一括取得
func (p Projections) loadConfirmations(ctx context.Context, userID string, tasks []domain.Task) (map[string][]domain.Confirmation, error) {
	result := make(map[string][]domain.Confirmation, len(tasks))
	if p.Confirmations.Repo == nil || len(tasks) == 0 {
		return result, nil
	}
	for _, task := range tasks {
		values, err := p.Confirmations.List(ctx, userID, task.ID)
		if err != nil {
			return nil, err
		}
		result[task.ID] = values
	}
	return result, nil
}

// 検索条件（OpenAPI searchTasks / MCP search_tasks 共通）
type ProjectedSearch struct {
	Query    string
	GenreIDs []string
	Priority *domain.Priority
	Deadline string
	Sort     string
}

// OpenAPIで定義された期限区分
var SearchDeadlines = map[string]bool{"today": true, "tomorrow": true, "week": true, "month": true, "later": true, "overdue": true}

// OpenAPIで定義された並び順
var SearchSorts = map[string]bool{"taskScore": true, "priority": true, "deadline": true}

// 実効優先度・期限・TaskScoreを反映した検索
func (p Projections) Search(ctx context.Context, userID string, search ProjectedSearch) ([]domain.TaskProjection, error) {
	if search.Deadline != "" && !SearchDeadlines[search.Deadline] {
		return nil, domain.ErrInvalidTaskInput
	}
	if search.Sort != "" && !SearchSorts[search.Sort] {
		return nil, domain.ErrInvalidTaskInput
	}
	if search.Priority != nil && *search.Priority != domain.HighPriority && *search.Priority != domain.MediumPriority && *search.Priority != domain.LowPriority {
		return nil, domain.ErrInvalidPriority
	}
	// 文字列・ジャンル・期限の絞り込み（優先度は実効優先度で後段）と、階層・依存を含めた同ジャンル全体の投影を並行に
	var (
		matched []domain.Task
		result  ProjectionResult
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		matched, err = p.Tasks.Search(groupCtx, userID, domain.TaskSearch{Query: search.Query, GenreIDs: search.GenreIDs, Deadline: search.Deadline})
		return err
	})
	group.Go(func() (err error) {
		result, err = p.Project(groupCtx, userID, search.GenreIDs)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}
	byID := make(map[string]domain.TaskProjection, len(result.Tasks))
	for _, value := range result.Tasks {
		byID[value.Task.ID] = value
	}
	keep := func(value domain.TaskProjection) bool {
		return search.Priority == nil || value.EffectivePriority == *search.Priority
	}
	values := make([]domain.TaskProjection, 0, len(matched))
	if search.Sort == "taskScore" {
		ids := make(map[string]bool, len(matched))
		for _, task := range matched {
			ids[task.ID] = true
		}
		// 投影結果はおすすめ順
		for _, value := range result.Tasks {
			if ids[value.Task.ID] && keep(value) {
				values = append(values, value)
			}
		}
		return values, nil
	}
	for _, task := range matched {
		if value, ok := byID[task.ID]; ok && keep(value) {
			values = append(values, value)
		}
	}
	switch search.Sort {
	case "priority":
		domain.SortByEffectivePriority(values)
	default:
		// 既定・deadline: 期限の近い順（期限なしは後ろ）
		domain.SortByDeadline(values)
	}
	return values, nil
}
