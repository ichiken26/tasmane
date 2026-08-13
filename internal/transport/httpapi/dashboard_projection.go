package httpapi

import (
	"sort"
	"tasmane/internal/domain"
	"time"
)

func projectDashboardResponses(tasks []domain.Task, dependencies []domain.Dependency, confirmations map[string][]domain.Confirmation, now time.Time) ([]taskResponse, error) {
	responses := projectTaskResponses(tasks, now)
	responseByID := make(map[string]*taskResponse, len(responses))
	taskByID := make(map[string]domain.Task, len(tasks))
	children := make(map[string][]string)
	for index := range responses {
		responseByID[responses[index].ID] = &responses[index]
	}
	for _, task := range tasks {
		taskByID[task.ID] = task
		if task.ParentTaskID != nil {
			children[*task.ParentTaskID] = append(children[*task.ParentTaskID], task.ID)
		}
	}
	descendantDeadlines := func(root string) []time.Time {
		result := []time.Time{}
		queue := append([]string(nil), children[root]...)
		seen := map[string]bool{}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			if task, ok := taskByID[id]; ok {
				if task.EndAt != nil {
					result = append(result, *task.EndAt)
				}
				queue = append(queue, children[id]...)
			}
		}
		return result
	}

	prerequisites := make(map[string][]string)
	dependents := make(map[string][]string)
	for _, edge := range dependencies {
		prerequisites[edge.TaskID] = append(prerequisites[edge.TaskID], edge.DependsOnTaskID)
		dependents[edge.DependsOnTaskID] = append(dependents[edge.DependsOnTaskID], edge.TaskID)
	}
	dependentDistances := func(root string) (int, []int) {
		direct := len(dependents[root])
		distances := []int{}
		queueIDs := append([]string(nil), dependents[root]...)
		queueDistances := make([]int, len(queueIDs))
		for index := range queueDistances {
			queueDistances[index] = 1
		}
		seen := map[string]bool{root: true}
		for len(queueIDs) > 0 {
			id, distance := queueIDs[0], queueDistances[0]
			queueIDs, queueDistances = queueIDs[1:], queueDistances[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			if distance > 1 {
				distances = append(distances, distance)
			}
			for _, next := range dependents[id] {
				queueIDs = append(queueIDs, next)
				queueDistances = append(queueDistances, distance+1)
			}
		}
		return direct, distances
	}

	for _, task := range tasks {
		response := responseByID[task.ID]
		direct, indirect := dependentDistances(task.ID)
		priority, err := domain.CalculatePriority(domain.PriorityInput{
			Now: now, Deadline: task.EndAt, DescendantDeadlines: descendantDeadlines(task.ID),
			Progress: response.Progress, DirectDependents: direct, IndirectDependents: len(indirect),
		})
		if err != nil {
			return nil, err
		}
		effective, err := domain.EffectivePriority(task.ManualPriority, priority.Priority)
		if err != nil {
			return nil, err
		}
		response.EffectivePriority = effective
		unmet := []string{}
		for _, prerequisiteID := range prerequisites[task.ID] {
			if prerequisite, ok := taskByID[prerequisiteID]; ok && prerequisite.Status != domain.Completed {
				unmet = append(unmet, prerequisite.Title)
			}
		}
		checking := false
		for _, confirmation := range confirmations[task.ID] {
			checking = checking || confirmation.Status == domain.ConfirmationChecking
		}
		createdAt := task.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		score, err := domain.CalculateTaskScore(domain.TaskScoreInput{
			ID: task.ID, Status: task.Status, EffectivePriority: effective, Deadline: priority.Deadline,
			Progress: response.Progress, CreatedAt: createdAt, Now: now, DirectDependents: direct,
			IndirectDependentDistances: indirect, UnmetDependencies: unmet, HasCheckingConfirmation: checking,
		})
		if err != nil {
			return nil, err
		}
		response.TaskScore = &score.Score
		response.RecommendationReasons = score.Reasons
		response.Recommended = score.Eligible
	}
	sort.SliceStable(responses, func(i, j int) bool {
		if responses[i].Recommended != responses[j].Recommended {
			return responses[i].Recommended
		}
		if *responses[i].TaskScore != *responses[j].TaskScore {
			return *responses[i].TaskScore > *responses[j].TaskScore
		}
		if responses[i].EndAt == nil != (responses[j].EndAt == nil) {
			return responses[i].EndAt != nil
		}
		if responses[i].EndAt != nil && !responses[i].EndAt.Equal(*responses[j].EndAt) {
			return responses[i].EndAt.Before(*responses[j].EndAt)
		}
		return responses[i].ID < responses[j].ID
	})
	return responses, nil
}

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
			score, eligible, err := domain.CalculateConfirmationScore(domain.ConfirmationScoreInput{Status: value.Status, DueAt: value.DueAt, ParentTaskScore: scores[taskID], Now: now})
			if err != nil {
				return nil, err
			}
			if eligible {
				result = append(result, dashboardConfirmationResponse{ID: value.ID, TaskID: taskID, TaskTitle: titles[taskID], Content: value.Content, TargetName: value.TargetName, Status: value.Status, DueAt: value.DueAt, Score: score})
			}
		}
	}
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
