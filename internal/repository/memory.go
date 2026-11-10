package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"tasmane/internal/domain"
	"time"
)

const OptimisticUpdateSQL = `UPDATE tasks SET status = $1, leaf_progress = $2, version = version + 1, updated_at = now() WHERE id = $3 AND user_id = $4 AND version = $5 AND deleted_at IS NULL`

// インメモリタスクリポジトリ
type Memory struct {
	mu        sync.Mutex
	Tasks     map[string]domain.Task
	MemoLinks []domain.MemoLink
}

// メモリリポジトリの生成
func NewMemory(tasks ...domain.Task) *Memory {
	m := &Memory{Tasks: map[string]domain.Task{}}
	for _, t := range tasks {
		m.Tasks[t.ID] = t
	}
	return m
}

// タスクの新規作成
func (m *Memory) Create(_ context.Context, userID string, draft domain.TaskDraft, _ domain.TaskCreateAuthorization) (domain.Task, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("task-%d", len(m.Tasks)+1)
	task, err := domain.NewTask(id, userID, draft)
	if err != nil {
		return domain.Task{}, err
	}
	m.Tasks[id] = task
	return task, nil
}

// タスク詳細の取得
func (m *Memory) Get(_ context.Context, userID, id string) (domain.Task, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.Tasks[id]
	if !ok || t.UserID != userID {
		return domain.Task{}, domain.ErrForbidden
	}
	t.HasChildren = t.HasChildren || m.hasChildren(userID, id)
	return t, nil
}

// タスク状態の楽観更新
func (m *Memory) UpdateStatus(_ context.Context, t domain.Task, expected int64, _ string) (domain.Task, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Tasks[t.ID]
	if !ok || current.UserID != t.UserID {
		return domain.Task{}, domain.ErrForbidden
	}
	if current.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	t.Version = expected + 1
	m.Tasks[t.ID] = t
	return t, nil
}

// 状態遷移計画のプレビュー
func (m *Memory) PreviewStatusTransition(_ context.Context, userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusTransitionPlan(userID, taskID, expected, to)
}

// 状態遷移計画の算出
func (m *Memory) statusTransitionPlan(userID, taskID string, expected int64, to domain.Status) (domain.StatusTransitionPlan, error) {
	target, ok := m.Tasks[taskID]
	if !ok || target.UserID != userID {
		return domain.StatusTransitionPlan{}, domain.ErrForbidden
	}
	if target.Version != expected {
		return domain.StatusTransitionPlan{}, domain.ErrVersionConflict
	}
	ancestors := []domain.Task{}
	for parentID := target.ParentTaskID; parentID != nil; {
		parent, exists := m.Tasks[*parentID]
		if !exists || parent.UserID != userID {
			return domain.StatusTransitionPlan{}, domain.ErrForbidden
		}
		ancestors = append(ancestors, parent)
		parentID = parent.ParentTaskID
	}
	descendants := []domain.Task{}
	queue := []string{taskID}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		for _, candidate := range m.Tasks {
			if candidate.UserID == userID && candidate.ParentTaskID != nil && *candidate.ParentTaskID == parentID {
				descendants = append(descendants, candidate)
				queue = append(queue, candidate.ID)
			}
		}
	}
	return domain.PlanStatusTransition(target, ancestors, descendants, to)
}

// 状態遷移計画の適用
func (m *Memory) ApplyStatusTransition(_ context.Context, userID, taskID string, expected int64, to domain.Status, _ string, approved domain.StatusTransitionPlan) (domain.Task, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.statusTransitionPlan(userID, taskID, expected, to)
	if err != nil {
		return domain.Task{}, err
	}
	if !sameStatusPlan(current, approved) {
		return domain.Task{}, domain.ErrStatusImpactPlanChanged
	}
	for _, change := range current.Changes {
		task := m.Tasks[change.TaskID]
		if task.Version != change.ExpectedVersion {
			return domain.Task{}, domain.ErrVersionConflict
		}
		task.Status, task.Progress, task.Version = change.ToStatus, change.ToProgress, task.Version+1
		m.Tasks[task.ID] = task
	}
	return m.Tasks[taskID], nil
}

// 状態遷移計画の一致判定
func sameStatusPlan(a, b domain.StatusTransitionPlan) bool {
	if a.TaskID != b.TaskID || a.ToStatus != b.ToStatus || a.ExpectedVersion != b.ExpectedVersion || len(a.Changes) != len(b.Changes) {
		return false
	}
	for i := range a.Changes {
		x, y := a.Changes[i], b.Changes[i]
		if x.TaskID != y.TaskID || x.FromStatus != y.FromStatus || x.ToStatus != y.ToStatus || x.ExpectedVersion != y.ExpectedVersion || !sameInt(x.FromProgress, y.FromProgress) || !sameInt(x.ToProgress, y.ToProgress) {
			return false
		}
	}
	return true
}

// 整数ポインタの一致判定
func sameInt(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

// タスク属性のパッチ更新
func (m *Memory) Update(_ context.Context, update domain.TaskUpdate, expected int64) (domain.Task, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.Tasks[update.Task.ID]
	if !ok || current.UserID != update.Task.UserID {
		return domain.Task{}, domain.ErrForbidden
	}
	if current.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	update.Task.Version = expected + 1
	m.Tasks[update.Task.ID] = update.Task
	return update.Task, nil
}

// タスク削除計画の適用
func (m *Memory) DeleteTask(_ context.Context, userID, taskID string, expected int64, confirmed bool) (domain.TaskDeletionPlan, error) {
	// メモリロック取得
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.Tasks[taskID]
	if !ok || task.UserID != userID {
		return domain.TaskDeletionPlan{}, domain.ErrForbidden
	}
	if task.Version != expected {
		return domain.TaskDeletionPlan{}, domain.ErrVersionConflict
	}
	plan := domain.TaskDeletionPlan{TaskIDs: []string{taskID}}
	if !confirmed {
		return plan, domain.ErrImpactConfirmationRequired
	}
	delete(m.Tasks, taskID)
	return plan, nil
}

// Google起点の完了取消
func (m *Memory) GoogleUncomplete(ctx context.Context, userID, taskID string, expected int64) (domain.Task, error) {
	task, err := m.Get(ctx, userID, taskID)
	if err != nil {
		return task, err
	}
	if task.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	next, err := task.Transition(domain.InProgress)
	if err != nil {
		return domain.Task{}, err
	}
	return m.UpdateStatus(ctx, next, expected, "GOOGLE_TASKS")
}

// 条件付きタスク検索（ジャンル・フリーワードのみ）
func (m *Memory) Search(_ context.Context, userID string, search domain.TaskSearch) ([]domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	genres := map[string]bool{}
	for _, id := range search.GenreIDs {
		genres[id] = true
	}
	result := []domain.Task{}
	for _, task := range m.Tasks {
		if task.UserID != userID || (len(genres) > 0 && !genres[task.GenreID]) {
			continue
		}
		if search.Query != "" && !strings.Contains(task.Title, search.Query) {
			continue
		}
		task.HasChildren = m.hasChildren(userID, task.ID)
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// 根から辿った階層全体
func (m *Memory) Tree(_ context.Context, userID, taskID string) ([]domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.Tasks[taskID]
	if !ok || task.UserID != userID {
		return nil, domain.ErrForbidden
	}
	for task.ParentTaskID != nil {
		parent, exists := m.Tasks[*task.ParentTaskID]
		if !exists {
			break
		}
		task = parent
	}
	result := []domain.Task{}
	queue := []domain.Task{task}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		current.HasChildren = m.hasChildren(userID, current.ID)
		result = append(result, current)
		for _, candidate := range m.Tasks {
			if candidate.UserID == userID && candidate.ParentTaskID != nil && *candidate.ParentTaskID == current.ID {
				queue = append(queue, candidate)
			}
		}
	}
	return result, nil
}

// 葉進捗の更新
func (m *Memory) UpdateLeafProgress(_ context.Context, userID, taskID string, expected int64, progress int, _ string) (domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.Tasks[taskID]
	if !ok || task.UserID != userID {
		return domain.Task{}, domain.ErrForbidden
	}
	if task.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	task.HasChildren = m.hasChildren(userID, taskID)
	if err := domain.ValidateLeafProgress(task, progress); err != nil {
		return domain.Task{}, err
	}
	task.Progress, task.Version = &progress, task.Version+1
	m.Tasks[taskID] = task
	return task, nil
}

// 子の有無
func (m *Memory) hasChildren(userID, taskID string) bool {
	for _, candidate := range m.Tasks {
		if candidate.UserID == userID && candidate.ParentTaskID != nil && *candidate.ParentTaskID == taskID {
			return true
		}
	}
	return false
}

// メモリンク一覧（所有タスクのみ）
func (m *Memory) ListMemoLinks(_ context.Context, userID, taskID string) ([]domain.MemoLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.Tasks[taskID]; !ok || t.UserID != userID {
		return nil, domain.ErrForbidden
	}
	result := []domain.MemoLink{}
	for _, link := range m.MemoLinks {
		if link.TaskID == taskID {
			result = append(result, link)
		}
	}
	return result, nil
}

// メモリンクの登録（同一URLはタイトル更新）
func (m *Memory) UpsertMemoLink(_ context.Context, userID, taskID, url, title string) (domain.MemoLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.Tasks[taskID]; !ok || t.UserID != userID {
		return domain.MemoLink{}, domain.ErrForbidden
	}
	for index, link := range m.MemoLinks {
		if link.TaskID == taskID && link.URL == url {
			m.MemoLinks[index].Title = title
			return m.MemoLinks[index], nil
		}
	}
	link := domain.MemoLink{ID: fmt.Sprintf("memo-link-%d", len(m.MemoLinks)+1), TaskID: taskID, URL: url, Title: title, MemoDate: domain.MemoLinkDate(url), CreatedAt: time.Now()}
	m.MemoLinks = append(m.MemoLinks, link)
	return link, nil
}

// メモリンクの解除
func (m *Memory) DeleteMemoLink(_ context.Context, userID, taskID, linkID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.Tasks[taskID]; !ok || t.UserID != userID {
		return domain.ErrForbidden
	}
	for index, link := range m.MemoLinks {
		if link.TaskID == taskID && link.ID == linkID {
			m.MemoLinks = append(m.MemoLinks[:index], m.MemoLinks[index+1:]...)
			return nil
		}
	}
	return domain.ErrForbidden
}
