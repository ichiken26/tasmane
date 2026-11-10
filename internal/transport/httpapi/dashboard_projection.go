package httpapi

import (
	"sort"
	"tasmane/internal/domain"
	"time"
)

// 優先度・スコア・推薦付きダッシュボードタスク投影（算出はドメイン共通投影）
func projectDashboardResponses(tasks []domain.Task, dependencies []domain.Dependency, confirmations map[string][]domain.Confirmation, now time.Time) ([]taskResponse, error) {
	projected, err := domain.ProjectTasks(domain.ProjectionInput{Tasks: tasks, Dependencies: dependencies, Confirmations: confirmations, Now: now})
	if err != nil {
		return nil, err
	}
	return toProjectedResponses(projected), nil
}

// ダッシュボード確認事項の応答
type dashboardConfirmationResponse struct {
	ID         string                    `json:"id"`
	TaskID     string                    `json:"taskId"`
	TaskTitle  string                    `json:"taskTitle"`
	Content    string                    `json:"content"`
	TargetName string                    `json:"targetName"`
	Status     domain.ConfirmationStatus `json:"status"`
	DueAt      *time.Time                `json:"dueAt"`
	Score      int                       `json:"-"`
}

// 推薦対象確認事項の投影
func projectDashboardConfirmations(tasks []taskResponse, confirmations map[string][]domain.Confirmation, titles map[string]string, now time.Time) ([]dashboardConfirmationResponse, error) {
	scores := make(map[string]int, len(tasks))
	for _, task := range tasks {
		if task.TaskScore != nil {
			scores[task.ID] = *task.TaskScore
		}
	}
	result := []dashboardConfirmationResponse{}
	for taskID, values := range confirmations {
		for _, value := range values {
			// 確認事項スコアの算出
			score, eligible, err := domain.CalculateConfirmationScore(domain.ConfirmationScoreInput{Status: value.Status, DueAt: value.DueAt, ParentTaskScore: scores[taskID], Now: now})
			if err != nil {
				return nil, err
			}
			if eligible {
				result = append(result, dashboardConfirmationResponse{ID: value.ID, TaskID: taskID, TaskTitle: titles[taskID], Content: value.Content, TargetName: value.TargetName, Status: value.Status, DueAt: value.DueAt, Score: score})
			}
		}
	}
	// 確認事項の推薦順整列
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		if result[i].DueAt == nil != (result[j].DueAt == nil) {
			return result[i].DueAt != nil
		}
		if result[i].DueAt != nil && !result[i].DueAt.Equal(*result[j].DueAt) {
			return result[i].DueAt.Before(*result[j].DueAt)
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}
