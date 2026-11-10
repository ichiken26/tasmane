package application

import (
	"time"

	"tasmane/internal/domain"
)

// OpenAPI Task スキーマに対応する算出値付き表現（HTTP/MCP/WebMCP共通）
type TaskView struct {
	ID                    string           `json:"id"`
	GenreID               string           `json:"genreId"`
	TaskTypeID            *string          `json:"taskTypeId"`
	ParentTaskID          *string          `json:"parentTaskId"`
	Title                 string           `json:"title"`
	Body                  *string          `json:"body"`
	Status                domain.Status    `json:"status"`
	Progress              *int             `json:"progress"`
	StartAt               *time.Time       `json:"startAt"`
	EndAt                 *time.Time       `json:"endAt"`
	IsAllDay              bool             `json:"isAllDay"`
	ManualPriority        *domain.Priority `json:"manualPriority"`
	EffectivePriority     domain.Priority  `json:"effectivePriority"`
	IsOverdue             bool             `json:"isOverdue"`
	ShowFire              bool             `json:"showFire"`
	Version               int64            `json:"version"`
	TaskScore             *int             `json:"taskScore,omitempty"`
	RecommendationReasons []string         `json:"recommendationReasons,omitempty"`
	Recommended           bool             `json:"recommended"`
	CanComplete           bool             `json:"canComplete"`
	IsLeaf                bool             `json:"isLeaf"`
	Warnings              []string         `json:"warnings,omitempty"`
}

// ドメイン投影からの変換
func NewTaskView(value domain.TaskProjection) TaskView {
	task := value.Task
	score := value.TaskScore
	return TaskView{
		ID: task.ID, GenreID: task.GenreID, TaskTypeID: task.TaskTypeID,
		ParentTaskID: task.ParentTaskID, Title: task.Title, Body: task.Body,
		Status: task.Status, Progress: value.Progress, StartAt: task.StartAt, EndAt: task.EndAt,
		IsAllDay: task.IsAllDay, ManualPriority: task.ManualPriority,
		EffectivePriority: value.EffectivePriority, IsOverdue: value.IsOverdue, ShowFire: value.ShowFire,
		Version: task.Version, TaskScore: &score, RecommendationReasons: value.Reasons,
		Recommended: value.Recommended, CanComplete: value.CanComplete, IsLeaf: !task.HasChildren,
	}
}

// 投影列の変換
func NewTaskViews(values []domain.TaskProjection) []TaskView {
	result := make([]TaskView, 0, len(values))
	for _, value := range values {
		result = append(result, NewTaskView(value))
	}
	return result
}

// OpenAPI Genre / TaskType スキーマ表現（user_idは外部へ出さない）
type NamedView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OpenAPI Confirmation スキーマ表現
type ConfirmationView struct {
	ID          string                    `json:"id"`
	TaskID      string                    `json:"taskId"`
	Content     string                    `json:"content"`
	TargetName  string                    `json:"targetName"`
	Status      domain.ConfirmationStatus `json:"status"`
	DueAt       *time.Time                `json:"dueAt"`
	ConfirmedAt *time.Time                `json:"confirmedAt"`
}

// 依存関係の表現
type DependencyView struct {
	TaskID          string `json:"taskId"`
	DependsOnTaskID string `json:"dependsOnTaskId"`
}

// 確認事項の変換
func NewConfirmationView(value domain.Confirmation) ConfirmationView {
	return ConfirmationView{ID: value.ID, TaskID: value.TaskID, Content: value.Content, TargetName: value.TargetName, Status: value.Status, DueAt: value.DueAt, ConfirmedAt: value.ConfirmedAt}
}
