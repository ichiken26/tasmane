package httpapi

import (
	"testing"
	"time"

	"tasmane/internal/domain"
)

func TestDashboardProjectionUsesDomainPriorityScoreAndReasons(t *testing.T) {
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	soon := now.Add(12 * time.Hour)
	old := now.AddDate(0, 0, -30)
	tasks := []domain.Task{
		{ID: "blocker", UserID: "u", GenreID: "g", Title: "Prerequisite", Status: domain.NotStarted, EndAt: &soon, Version: 1, CreatedAt: old},
		{ID: "dependent", UserID: "u", GenreID: "g", Title: "Dependent", Status: domain.NotStarted, Version: 1, CreatedAt: now},
	}
	responses, err := projectDashboardResponses(tasks, []domain.Dependency{{TaskID: "dependent", DependsOnTaskID: "blocker"}}, map[string][]domain.Confirmation{
		"dependent": {{Status: domain.ConfirmationChecking}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 2 || responses[0].ID != "blocker" || responses[0].TaskScore == nil || !responses[0].Recommended {
		t.Fatalf("unexpected ranking: %+v", responses)
	}
	if responses[0].EffectivePriority != domain.HighPriority {
		t.Fatalf("calculated priority=%s", responses[0].EffectivePriority)
	}
	found := false
	for _, reason := range responses[1].RecommendationReasons {
		if reason == "前提タスクPrerequisiteが未完了" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing deterministic dependency reason: %+v", responses[1])
	}
}
