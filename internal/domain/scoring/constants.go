package scoring

// 優先度・タスク・確認スコアの加重と閾値
const (
	PriorityDeadlineWeight   = 0.50
	PriorityProgressWeight   = 0.30
	PriorityDependencyWeight = 0.20
	PriorityLowMaximum       = 34
	PriorityMediumMaximum    = 64

	TaskDeadlineWeight     = 0.30
	TaskPriorityWeight     = 0.25
	TaskDependencyWeight   = 0.20
	TaskProgressWeight     = 0.15
	TaskAgeWeight          = 0.05
	TaskConfirmationWeight = 0.05
	UnmetDependencyPenalty = 40
	CheckingPenalty        = 35
	OverdueCheckingPenalty = 10
	DependencyDecayFactor  = 0.50
	DependencyScoreCap     = 100
	TaskAgeCapDays         = 30

	ConfirmationDueWeight        = 0.50
	ConfirmationStatusWeight     = 0.20
	ConfirmationParentWeight     = 0.30
	ConfirmationNoDueUnconfirmed = 20
	ConfirmationNoDueChecking    = 40
)

// 期限緊急度の時間帯閾値
var DeadlineUrgencyThresholdHours = []float64{24, 24 * 7, 24 * 30}
