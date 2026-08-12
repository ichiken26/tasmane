package domain

import (
	"errors"
	"math"
	"sort"
	"strings"
	"tasmane/internal/domain/scoring"
	"time"
)

var ErrInvalidScoringInput = errors.New("invalid scoring input")

type PriorityInput struct {
	Now                                  time.Time
	Deadline                             *time.Time
	DescendantDeadlines                  []time.Time
	Progress                             *int
	DirectDependents, IndirectDependents int
}

type PriorityResult struct {
	Score    int
	Priority Priority
	Deadline *time.Time
}

func EffectivePriority(manual *Priority, calculated Priority) (Priority, error) {
	if manual != nil {
		if _, ok := priorityValue(*manual); !ok {
			return "", ErrInvalidPriority
		}
		return *manual, nil
	}
	if _, ok := priorityValue(calculated); !ok {
		return "", ErrInvalidPriority
	}
	return calculated, nil
}

func CalculatePriority(input PriorityInput) (PriorityResult, error) {
	if input.Progress != nil && (*input.Progress < 0 || *input.Progress > 100) {
		return PriorityResult{}, ErrInvalidProgress
	}
	if input.Now.IsZero() || input.DirectDependents < 0 || input.IndirectDependents < 0 ||
		(input.Deadline != nil && input.Deadline.IsZero()) || containsZeroTime(input.DescendantDeadlines) {
		return PriorityResult{}, ErrInvalidScoringInput
	}
	deadline := effectiveDeadline(input.Deadline, input.DescendantDeadlines)
	if deadline == nil {
		return PriorityResult{Score: 50, Priority: MediumPriority}, nil
	}
	progress := 0
	if input.Progress != nil {
		progress = *input.Progress
	}
	deadlineScore := deadlineUrgency(input.Now, *deadline)
	dependencyScore := saturatingWeightedSum(100, input.DirectDependents, 30, input.IndirectDependents, 10)
	score := roundScore(float64(deadlineScore)*scoring.PriorityDeadlineWeight + float64(100-progress)*scoring.PriorityProgressWeight + float64(dependencyScore)*scoring.PriorityDependencyWeight)
	return PriorityResult{Score: score, Priority: priorityForScore(score), Deadline: deadline}, nil
}

func effectiveDeadline(own *time.Time, descendants []time.Time) *time.Time {
	if own != nil {
		value := *own
		return &value
	}
	if len(descendants) == 0 {
		return nil
	}
	value := descendants[0]
	for _, candidate := range descendants[1:] {
		if candidate.Before(value) {
			value = candidate
		}
	}
	return &value
}

func priorityForScore(score int) Priority {
	if score <= scoring.PriorityLowMaximum {
		return LowPriority
	}
	if score <= scoring.PriorityMediumMaximum {
		return MediumPriority
	}
	return HighPriority
}

type TaskScoreInput struct {
	ID                         string
	Status                     Status
	EffectivePriority          Priority
	Deadline                   *time.Time
	Progress                   *int
	CreatedAt, Now             time.Time
	DirectDependents           int
	IndirectDependentDistances []int
	UnmetDependencies          []string
	HasCheckingConfirmation    bool
}

type TaskScoreResult struct {
	ID       string
	Score    int
	Eligible bool
	Deadline *time.Time
	Reasons  []string
}

func CalculateTaskScore(input TaskScoreInput) (TaskScoreResult, error) {
	if strings.TrimSpace(input.ID) == "" || !validTaskStatus(input.Status) || input.Now.IsZero() || input.CreatedAt.IsZero() ||
		(input.Deadline != nil && input.Deadline.IsZero()) || input.DirectDependents < 0 {
		return TaskScoreResult{}, ErrInvalidScoringInput
	}
	for _, distance := range input.IndirectDependentDistances {
		if distance <= 1 {
			return TaskScoreResult{}, ErrInvalidScoringInput
		}
	}
	result := TaskScoreResult{ID: input.ID, Eligible: input.Status != Completed, Deadline: input.Deadline}
	if !result.Eligible {
		return result, nil
	}
	if input.Progress != nil && (*input.Progress < 0 || *input.Progress > 100) {
		return TaskScoreResult{}, ErrInvalidProgress
	}
	priorityScore, ok := priorityValue(input.EffectivePriority)
	if !ok {
		return TaskScoreResult{}, ErrInvalidPriority
	}
	progress := 0
	if input.Progress != nil {
		progress = *input.Progress
	}
	deadlineScore, overdue := 50, false
	if input.Deadline != nil {
		deadlineScore = deadlineUrgency(input.Now, *input.Deadline)
		overdue = input.Deadline.Before(input.Now)
	}
	dependencyScore := saturatingMultiply(scoring.DependencyScoreCap, input.DirectDependents, 30)
	for _, distance := range input.IndirectDependentDistances {
		if dependencyScore < scoring.DependencyScoreCap {
			increment := int(math.Round(30 * math.Pow(scoring.DependencyDecayFactor, float64(distance-1))))
			dependencyScore = saturatingAdd(scoring.DependencyScoreCap, dependencyScore, increment)
		}
	}
	dependencyScore = minInt(scoring.DependencyScoreCap, dependencyScore)
	ageDays := input.Now.Sub(input.CreatedAt).Hours() / 24
	if ageDays < 0 {
		ageDays = 0
	}
	ageScore := minInt(100, roundScore(ageDays/float64(scoring.TaskAgeCapDays)*100))
	confirmationScore := 50
	score := roundScore(float64(deadlineScore)*scoring.TaskDeadlineWeight + float64(priorityScore)*scoring.TaskPriorityWeight + float64(dependencyScore)*scoring.TaskDependencyWeight + float64(100-progress)*scoring.TaskProgressWeight + float64(ageScore)*scoring.TaskAgeWeight + float64(confirmationScore)*scoring.TaskConfirmationWeight)
	if len(input.UnmetDependencies) > 0 {
		score -= scoring.UnmetDependencyPenalty
	}
	if input.HasCheckingConfirmation {
		if overdue {
			score -= scoring.OverdueCheckingPenalty
		} else {
			score -= scoring.CheckingPenalty
		}
	}
	result.Score = clampScore(score)
	if overdue {
		result.Reasons = append(result.Reasons, "期限切れ")
	} else if input.Deadline != nil && input.Deadline.Sub(input.Now) <= 7*24*time.Hour {
		result.Reasons = append(result.Reasons, "期限が近い")
	}
	if input.EffectivePriority == HighPriority {
		result.Reasons = append(result.Reasons, "優先度が高い")
	}
	if progress <= 25 {
		result.Reasons = append(result.Reasons, "進捗が低い")
	}
	if input.DirectDependents+len(input.IndirectDependentDistances) > 1 {
		result.Reasons = append(result.Reasons, "複数タスクをブロック中")
	}
	names := append([]string(nil), input.UnmetDependencies...)
	sort.Strings(names)
	for _, name := range names {
		result.Reasons = append(result.Reasons, "前提タスク"+name+"が未完了")
	}
	if ageDays >= scoring.TaskAgeCapDays {
		result.Reasons = append(result.Reasons, "長期間未着手")
	}
	if input.HasCheckingConfirmation {
		result.Reasons = append(result.Reasons, "確認待ち")
	}
	return result, nil
}

func SortTaskScores(results []TaskScoreResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Eligible != results[j].Eligible {
			return results[i].Eligible
		}
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Deadline == nil != (results[j].Deadline == nil) {
			return results[i].Deadline != nil
		}
		if results[i].Deadline != nil && !results[i].Deadline.Equal(*results[j].Deadline) {
			return results[i].Deadline.Before(*results[j].Deadline)
		}
		return results[i].ID < results[j].ID
	})
}

func deadlineUrgency(now, deadline time.Time) int {
	hours := deadline.Sub(now).Hours()
	if hours <= scoring.DeadlineUrgencyThresholdHours[0] {
		return 100
	}
	if hours <= scoring.DeadlineUrgencyThresholdHours[1] {
		return 75
	}
	if hours <= scoring.DeadlineUrgencyThresholdHours[2] {
		return 50
	}
	return 25
}

func priorityValue(priority Priority) (int, bool) {
	value, ok := map[Priority]int{LowPriority: 20, MediumPriority: 60, HighPriority: 100}[priority]
	return value, ok
}
func roundScore(value float64) int { return int(math.Round(value)) }
func clampScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func validTaskStatus(status Status) bool {
	return status == NotStarted || status == InProgress || status == Completed
}

func containsZeroTime(values []time.Time) bool {
	for _, value := range values {
		if value.IsZero() {
			return true
		}
	}
	return false
}

func saturatingMultiply(cap, value, multiplier int) int {
	if value >= (cap+multiplier-1)/multiplier {
		return cap
	}
	return value * multiplier
}

func saturatingAdd(cap, left, right int) int {
	if left >= cap-right {
		return cap
	}
	return left + right
}

func saturatingWeightedSum(cap, first, firstWeight, second, secondWeight int) int {
	return saturatingAdd(cap, saturatingMultiply(cap, first, firstWeight), saturatingMultiply(cap, second, secondWeight))
}
