package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasmane/internal/application"
	"tasmane/internal/domain"
)

// 更新影響確認用のタスクリポジトリスタブ
type updateImpactTaskRepo struct {
	tasks      map[string]domain.Task
	lastUpdate domain.TaskUpdate
}

// 所有権付きタスク取得
func (repo *updateImpactTaskRepo) Get(_ context.Context, userID, taskID string) (domain.Task, error) {
	task, ok := repo.tasks[taskID]
	if !ok || task.UserID != userID {
		return domain.Task{}, domain.ErrForbidden
	}
	return task, nil
}

// ジャンル移動承認付きタスク更新の記録
func (repo *updateImpactTaskRepo) Update(_ context.Context, update domain.TaskUpdate, expected int64) (domain.Task, error) {
	if update.Task.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	if update.ApprovedGenreMove != nil {
		update.Task.ParentTaskID = nil
	}
	repo.lastUpdate = update
	update.Task.Version++
	repo.tasks[update.Task.ID] = update.Task
	return update.Task, nil
}

// ユーザー所有タスクのツリー返却
func (repo *updateImpactTaskRepo) Tree(_ context.Context, userID, _ string) ([]domain.Task, error) {
	result := []domain.Task{}
	for _, task := range repo.tasks {
		if task.UserID == userID {
			result = append(result, task)
		}
	}
	return result, nil
}

// 検索の空返却
func (repo *updateImpactTaskRepo) Search(context.Context, string, domain.TaskSearch) ([]domain.Task, error) {
	return nil, nil
}

// 作成の禁止
func (*updateImpactTaskRepo) Create(context.Context, string, domain.TaskDraft, domain.TaskCreateAuthorization) (domain.Task, error) {
	return domain.Task{}, domain.ErrForbidden
}

// ステータス更新の禁止
func (*updateImpactTaskRepo) UpdateStatus(context.Context, domain.Task, int64, string) (domain.Task, error) {
	return domain.Task{}, domain.ErrForbidden
}

// 削除の禁止
func (*updateImpactTaskRepo) DeleteTask(context.Context, string, string, int64, bool) (domain.TaskDeletionPlan, error) {
	return domain.TaskDeletionPlan{}, domain.ErrForbidden
}

// Google未完了化の禁止
func (*updateImpactTaskRepo) GoogleUncomplete(context.Context, string, string, int64) (domain.Task, error) {
	return domain.Task{}, domain.ErrForbidden
}

// 更新影響確認用の関係リポジトリスタブ
type updateImpactRelationRepo struct {
	tasks        []domain.GraphTask
	dependencies []domain.Dependency
}

// 関係グラフの固定返却
func (repo *updateImpactRelationRepo) LoadGraph(context.Context, string) ([]domain.GraphTask, []domain.Dependency, error) {
	return repo.tasks, repo.dependencies, nil
}

// 依存関係一覧の固定返却
func (repo *updateImpactRelationRepo) ListTaskDependencies(context.Context, string, string) ([]domain.Dependency, error) {
	return repo.dependencies, nil
}

// 親設定の成功
func (*updateImpactRelationRepo) SetParent(context.Context, string, string, string) error { return nil }

// 依存追加の成功
func (*updateImpactRelationRepo) AddDependency(context.Context, string, domain.Dependency, int64) error {
	return nil
}

// 依存削除の成功
func (*updateImpactRelationRepo) RemoveDependency(context.Context, string, domain.Dependency) error {
	return nil
}

// ジャンル移動の影響トークン拘束と完全影響列挙
func TestTaskGenreMoveRequiresBoundImpactTokenAndListsCompleteImpact(t *testing.T) {
	parent := "outside"
	repo := &updateImpactTaskRepo{tasks: map[string]domain.Task{
		"outside": {ID: "outside", UserID: "u", GenreID: "old", Title: "Outside", IsAllDay: true, Status: domain.NotStarted, Version: 1},
		"root":    {ID: "root", UserID: "u", GenreID: "old", ParentTaskID: &parent, Title: "Root", IsAllDay: true, Status: domain.NotStarted, Version: 1},
		"child":   {ID: "child", UserID: "u", GenreID: "old", ParentTaskID: stringPointer("root"), Title: "Child", IsAllDay: true, Status: domain.NotStarted, Version: 1},
	}}
	relations := updateImpactRelationRepo{
		tasks: []domain.GraphTask{
			{ID: "outside", UserID: "u", GenreID: "old"},
			{ID: "root", UserID: "u", GenreID: "old", ParentID: "outside"},
			{ID: "child", UserID: "u", GenreID: "old", ParentID: "root"},
		},
		dependencies: []domain.Dependency{{TaskID: "outside", DependsOnTaskID: "child"}},
	}
	handler := InternalAuth("binding", NewRouter(Handler{
		Tasks: application.Tasks{Repo: repo}, Relations: application.Relations{Repo: &relations},
		ImpactTokens: application.ImpactTokens{Secret: []byte("01234567890123456789012345678901")},
	}))
	// PATCHリクエストの送信
	do := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPatch, "/v1/tasks/root", strings.NewReader(body))
		request.Header.Set("X-Tasmane-Internal-Binding", "binding")
		request.Header.Set("X-Tasmane-Authenticated-User", "u")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	// 影響確認プレビューの取得
	preview := do(`{"expectedVersion":1,"genreId":"new"}`)
	if preview.Code != http.StatusConflict || !strings.Contains(preview.Body.String(), `"id":"child"`) || !strings.Contains(preview.Body.String(), `"type":"DEPENDENCY"`) {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	var impact struct {
		ImpactToken string `json:"impactToken"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &impact); err != nil || impact.ImpactToken == "" {
		t.Fatalf("impact=%+v err=%v", impact, err)
	}
	// 依存グラフ変化による旧プレビュー失効
	relations.dependencies = append(relations.dependencies, domain.Dependency{TaskID: "child", DependsOnTaskID: "outside"})
	stale := do(`{"expectedVersion":1,"genreId":"new","impactToken":"` + impact.ImpactToken + `"}`)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "IMPACT_TOKEN_INVALID") {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}
	repreview := do(`{"expectedVersion":1,"genreId":"new"}`)
	if err := json.Unmarshal(repreview.Body.Bytes(), &impact); err != nil || impact.ImpactToken == "" {
		t.Fatalf("repreview=%s err=%v", repreview.Body.String(), err)
	}
	// 新トークンによる確定更新
	confirmed := do(`{"expectedVersion":1,"genreId":"new","impactToken":"` + impact.ImpactToken + `"}`)
	if confirmed.Code != http.StatusOK || repo.lastUpdate.ApprovedGenreMove == nil || repo.lastUpdate.Task.ParentTaskID != nil {
		t.Fatalf("confirmed status=%d body=%s update=%+v", confirmed.Code, confirmed.Body.String(), repo.lastUpdate)
	}
}

// 明示的nullによる親タスク切り離し
func TestTaskPatchExplicitNullDetachesParent(t *testing.T) {
	parent := "parent"
	repo := &updateImpactTaskRepo{tasks: map[string]domain.Task{
		"parent": {ID: "parent", UserID: "u", GenreID: "g", Title: "Parent", IsAllDay: true, Status: domain.NotStarted, Version: 1},
		"task":   {ID: "task", UserID: "u", GenreID: "g", ParentTaskID: &parent, Title: "Task", IsAllDay: true, Status: domain.NotStarted, Version: 1},
	}}
	handler := InternalAuth("binding", NewRouter(Handler{Tasks: application.Tasks{Repo: repo}}))
	request := httptest.NewRequest(http.MethodPatch, "/v1/tasks/task", strings.NewReader(`{"expectedVersion":1,"parentTaskId":null}`))
	request.Header.Set("X-Tasmane-Internal-Binding", "binding")
	request.Header.Set("X-Tasmane-Authenticated-User", "u")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || repo.lastUpdate.Task.ParentTaskID != nil {
		t.Fatalf("status=%d body=%s update=%+v", response.Code, response.Body.String(), repo.lastUpdate)
	}
}
