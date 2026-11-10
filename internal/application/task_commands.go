package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"tasmane/internal/domain"
)

// 影響対象エンティティ（Impact Plan）
type ImpactEntity struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Title  *string `json:"title"`
	Reason *string `json:"reason"`
}

// 影響確認が必要な操作の結果（HTTP 409 / MCP confirmation_required）
type ImpactRequiredError struct {
	ImpactToken      string
	AffectedEntities []ImpactEntity
	Warnings         []string
}

// エラーメッセージ
func (e *ImpactRequiredError) Error() string { return domain.ErrImpactConfirmationRequired.Error() }

// errors.Is(domain.ErrImpactConfirmationRequired)との互換
func (e *ImpactRequiredError) Is(target error) bool {
	return target == domain.ErrImpactConfirmationRequired
}

// 葉進捗更新の永続化境界
type LeafProgressRepository interface {
	UpdateLeafProgress(context.Context, string, string, int64, int, string) (domain.Task, error)
}

// 承認集合付き削除の永続化境界
type ApprovedDeleteRepository interface {
	DeleteTaskApproved(context.Context, string, string, int64, []string) (domain.TaskDeletionPlan, error)
}

// 削除済みタスク復元の永続化境界
type RestoreRepository interface {
	ListDeletedTasks(context.Context, string) ([]domain.DeletedTask, error)
	RestoreTask(context.Context, string, string) ([]string, error)
}

// Impact Plan付きタスク操作（HTTP/MCP/WebMCP共通のユースケース）
type TaskCommands struct {
	Tasks     Tasks
	Relations Relations
	Tokens    ImpactTokens
}

// タスク更新の結果（保存は成功した上での日程矛盾警告を含む）
type TaskCommandResult struct {
	Task     domain.Task
	Warnings []string
}

// 部分更新の適用関数
type TaskMutation func(domain.Task) (domain.Task, error)

// 影響確認付きタスク更新（日付伝播・Genre移動・付け替え先の葉→親変換）
func (c TaskCommands) Update(ctx context.Context, userID, taskID string, expected int64, mutate TaskMutation, impactToken string) (TaskCommandResult, error) {
	if userID == "" || taskID == "" || expected < 1 || mutate == nil {
		return TaskCommandResult{}, domain.ErrForbidden
	}
	current, err := c.Tasks.Get(ctx, userID, taskID)
	if err != nil {
		return TaskCommandResult{}, err
	}
	if current.Version != expected {
		return TaskCommandResult{}, domain.ErrVersionConflict
	}
	next, err := mutate(current)
	if err != nil {
		return TaskCommandResult{}, err
	}
	next.ID, next.UserID, next.Version = current.ID, current.UserID, current.Version
	draft := domain.TaskDraft{GenreID: next.GenreID, Title: next.Title, StartAt: next.StartAt, EndAt: next.EndAt, IsAllDay: next.IsAllDay, ManualPriority: next.ManualPriority}
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return TaskCommandResult{}, err
	}
	next.Title = draft.Title
	genreChanged := next.GenreID != current.GenreID
	parentChanged := !genreChanged && stringValue(next.ParentTaskID) != stringValue(current.ParentTaskID)

	// 日付伝播計画（付け替え時は新しい親の木へ接続した仮想木で算出）
	dateTree, err := c.dateTreeFor(ctx, userID, current, next, parentChanged)
	if err != nil {
		return TaskCommandResult{}, err
	}
	changes, err := domain.PlanDateChange(dateTree, taskID, next.StartAt, next.EndAt)
	if err != nil {
		return TaskCommandResult{}, err
	}
	needsDateImpact := false
	for _, change := range changes {
		needsDateImpact = needsDateImpact || change.TaskID != taskID
	}
	movePlan := domain.GenreMovePlan{}
	if genreChanged {
		if c.Relations.Repo == nil {
			return TaskCommandResult{}, errors.New("relation repository unavailable")
		}
		graphTasks, dependencies, snapshotErr := c.Relations.Snapshot(ctx, userID)
		if snapshotErr != nil {
			return TaskCommandResult{}, snapshotErr
		}
		if movePlan, err = domain.PlanGenreMove(graphTasks, dependencies, taskID, next.GenreID); err != nil {
			return TaskCommandResult{}, err
		}
	}
	// 付け替え先が葉ならLeaf→Parent変換の確認
	var leafParent *domain.Task
	if parentChanged && next.ParentTaskID != nil {
		parent, getErr := c.Tasks.Get(ctx, userID, *next.ParentTaskID)
		if getErr != nil {
			return TaskCommandResult{}, getErr
		}
		if parent.Status == domain.Completed {
			return TaskCommandResult{}, domain.ErrCompletedParent
		}
		if !parent.HasChildren {
			leafParent = &parent
		}
	}
	needsImpact := needsDateImpact || genreChanged || leafParent != nil
	operation, err := updateImpactOperation(next, movePlan, changes, genreChanged, needsDateImpact, leafParent)
	if err != nil {
		return TaskCommandResult{}, err
	}
	claims := ImpactClaims{UserID: userID, Operation: operation, ResourceID: taskID, Version: current.Version}
	if needsImpact {
		if impactToken == "" {
			return TaskCommandResult{}, c.impactRequired(ctx, userID, claims, updateImpactEntities(changes, movePlan, needsDateImpact, genreChanged, leafParent), updateImpactWarnings(needsDateImpact, genreChanged, leafParent))
		}
		if err := c.Tokens.Verify(impactToken, claims); err != nil {
			return TaskCommandResult{}, ErrInvalidImpactToken
		}
	}
	update := domain.TaskUpdate{Task: next, DateChanges: changes}
	if genreChanged {
		update.ApprovedGenreMove = &movePlan
	}
	if leafParent != nil {
		version := leafParent.Version
		update.ApprovedLeafConversion = &version
	}
	updated, err := c.Tasks.Update(ctx, userID, update, expected)
	if errors.Is(err, domain.ErrImpactPlanChanged) {
		return TaskCommandResult{}, ErrInvalidImpactToken
	}
	if err != nil {
		return TaskCommandResult{}, err
	}
	return TaskCommandResult{Task: updated, Warnings: c.scheduleWarnings(ctx, userID, updated)}, nil
}

// 影響確認付きタスク作成（葉→親変換・祖先日付伝播）
func (c TaskCommands) Create(ctx context.Context, userID string, draft domain.TaskDraft, impactToken string) (domain.Task, error) {
	if userID == "" {
		return domain.Task{}, domain.ErrForbidden
	}
	if err := domain.ValidateTaskDraft(&draft); err != nil {
		return domain.Task{}, err
	}
	authorization := domain.TaskCreateAuthorization{}
	if draft.ParentTaskID == nil {
		return c.Tasks.Create(ctx, userID, draft, authorization)
	}
	parent, err := c.Tasks.Get(ctx, userID, *draft.ParentTaskID)
	if err != nil {
		return domain.Task{}, err
	}
	if parent.Status == domain.Completed {
		return domain.Task{}, domain.ErrCompletedParent
	}
	// 新規子を親の木へ仮想接続して祖先日付伝播を算出
	tree, err := c.Tasks.Tree(ctx, userID, parent.ID)
	if err != nil && !errors.Is(err, ErrTaskReadUnavailable) {
		return domain.Task{}, err
	}
	const virtualID = "00000000-0000-7000-8000-000000000000"
	dateTasks := toDateTasks(tree)
	if len(dateTasks) == 0 {
		dateTasks = []domain.DateTask{{ID: parent.ID, StartAt: parent.StartAt, EndAt: parent.EndAt}}
	}
	dateTasks = append(dateTasks, domain.DateTask{ID: virtualID, ParentID: parent.ID})
	changes, err := domain.PlanDateChange(dateTasks, virtualID, draft.StartAt, draft.EndAt)
	if err != nil {
		return domain.Task{}, err
	}
	ancestorChanges := []domain.DateChange{}
	for _, change := range changes {
		if change.TaskID != virtualID {
			ancestorChanges = append(ancestorChanges, change)
		}
	}
	leafConversion := !parent.HasChildren
	if !leafConversion && len(ancestorChanges) == 0 {
		return c.Tasks.Create(ctx, userID, draft, authorization)
	}
	digest, err := json.Marshal(struct {
		Leaf  bool                `json:"leaf"`
		Dates []domain.DateChange `json:"dates"`
	}{leafConversion, ancestorChanges})
	if err != nil {
		return domain.Task{}, err
	}
	hash := sha256.Sum256(digest)
	operation := "ADD_CHILD:" + hex.EncodeToString(hash[:])
	if leafConversion && len(ancestorChanges) == 0 {
		// 既存トークン互換（Leaf→Parent変換のみ）
		operation = "ADD_FIRST_CHILD"
	}
	claims := ImpactClaims{UserID: userID, Operation: operation, ResourceID: parent.ID, Version: parent.Version}
	if impactToken == "" {
		entities := []ImpactEntity{}
		warnings := []string{}
		if leafConversion {
			entities = append(entities, ImpactEntity{ID: parent.ID, Type: "TASK", Title: stringRef(parent.Title), Reason: stringRef("LEAF_TO_PARENT")})
			warnings = append(warnings, "進捗率は自動で再計算されます。子タスクを追加しますか？")
		}
		for _, change := range ancestorChanges {
			entities = append(entities, ImpactEntity{ID: change.TaskID, Type: "TASK", Reason: stringRef("DATE_PROPAGATION")})
		}
		if len(ancestorChanges) > 0 {
			warnings = append(warnings, "子タスクの日付に合わせて上位タスクの日付も変更されます。")
		}
		return domain.Task{}, c.impactRequired(ctx, userID, claims, entities, warnings)
	}
	if err := c.Tokens.Verify(impactToken, claims); err != nil {
		return domain.Task{}, ErrInvalidImpactToken
	}
	authorization = domain.TaskCreateAuthorization{AllowLeafConversion: leafConversion, ParentVersion: parent.Version, DateChanges: ancestorChanges}
	created, err := c.Tasks.Create(ctx, userID, draft, authorization)
	if errors.Is(err, domain.ErrImpactPlanChanged) {
		return domain.Task{}, ErrInvalidImpactToken
	}
	return created, err
}

// 影響確認付き状態遷移（出戻り伝播・未完了前提タスクへの着手警告）
func (c TaskCommands) Transition(ctx context.Context, userID, taskID string, expected int64, to domain.Status, via, impactToken string) (domain.Task, error) {
	plan, err := c.Tasks.PreviewTransition(ctx, userID, taskID, expected, to)
	if err != nil {
		if impactToken != "" && (errors.Is(err, domain.ErrVersionConflict) || errors.Is(err, domain.ErrInvalidTransition)) {
			return domain.Task{}, ErrInvalidImpactToken
		}
		return domain.Task{}, err
	}
	// 未完了の前提タスク（着手時のみ警告確認）
	unmet := []domain.Task{}
	if to == domain.InProgress {
		current, getErr := c.Tasks.Get(ctx, userID, taskID)
		if getErr != nil {
			return domain.Task{}, getErr
		}
		if current.Status == domain.NotStarted && c.Relations.Repo != nil {
			if unmet, err = c.unmetPrerequisites(ctx, userID, current); err != nil {
				return domain.Task{}, err
			}
		}
	}
	operation := StatusTransitionImpactOperation(plan)
	if len(unmet) > 0 {
		ids := make([]string, 0, len(unmet))
		for _, task := range unmet {
			ids = append(ids, task.ID)
		}
		operation += ":unmet:" + strings.Join(ids, ",")
	}
	if plan.RequiresConfirmation || len(unmet) > 0 {
		claims := ImpactClaims{UserID: userID, Operation: operation, ResourceID: taskID, Version: expected}
		if impactToken == "" {
			entities := []ImpactEntity{}
			warnings := []string{}
			for _, change := range plan.Changes {
				if !plan.RequiresConfirmation {
					break
				}
				reason := "STATUS_ROLLBACK"
				if change.TaskID != taskID && change.ToStatus == domain.InProgress {
					reason = "ANCESTOR_REOPEN"
				}
				if change.TaskID != taskID && change.ToStatus == domain.NotStarted {
					reason = "DESCENDANT_RESET"
				}
				entities = append(entities, ImpactEntity{ID: change.TaskID, Type: "TASK", Reason: stringRef(reason)})
			}
			if plan.RequiresConfirmation {
				warnings = append(warnings, "ステータスを戻すと関連タスクの状態も変更される場合があります。")
			}
			for _, task := range unmet {
				entities = append(entities, ImpactEntity{ID: task.ID, Type: "TASK", Title: stringRef(task.Title), Reason: stringRef("UNMET_DEPENDENCY")})
			}
			if len(unmet) > 0 {
				warnings = append(warnings, "前提タスクが未完了です。このまま着手しますか？")
			}
			return domain.Task{}, c.impactRequired(ctx, userID, claims, entities, warnings)
		}
		if err := c.Tokens.Verify(impactToken, claims); err != nil {
			return domain.Task{}, ErrInvalidImpactToken
		}
	}
	repository, ok := c.Tasks.Repo.(StatusTransitionRepository)
	if !ok {
		return domain.Task{}, errors.New("status transition repository unavailable")
	}
	result, err := repository.ApplyStatusTransition(ctx, userID, taskID, expected, to, via, plan)
	if errors.Is(err, domain.ErrStatusImpactPlanChanged) {
		return domain.Task{}, ErrInvalidImpactToken
	}
	return result, err
}

// 影響確認付き削除（人間UI/APIのみ。MCP/WebMCPへは公開しない）
func (c TaskCommands) Delete(ctx context.Context, userID, taskID string, expected int64, impactToken string) error {
	plan, err := c.Tasks.Delete(ctx, userID, taskID, expected, false)
	if !errors.Is(err, domain.ErrImpactConfirmationRequired) {
		if err == nil {
			return errors.New("delete preview unexpectedly applied")
		}
		return err
	}
	ids := append([]string(nil), plan.TaskIDs...)
	sort.Strings(ids)
	hash := sha256.Sum256([]byte(strings.Join(ids, ",")))
	claims := ImpactClaims{UserID: userID, Operation: "DELETE_TASK:" + hex.EncodeToString(hash[:]), ResourceID: taskID, Version: expected}
	if impactToken == "" {
		entities := make([]ImpactEntity, 0, len(plan.TaskIDs))
		for _, id := range plan.TaskIDs {
			entities = append(entities, ImpactEntity{ID: id, Type: "TASK", Reason: stringRef("SOFT_DELETE")})
		}
		return c.impactRequired(ctx, userID, claims, entities, []string{"これらのタスクも削除されます。よろしいですか？"})
	}
	if err := c.Tokens.Verify(impactToken, claims); err != nil {
		return ErrInvalidImpactToken
	}
	if approved, ok := c.Tasks.Repo.(ApprovedDeleteRepository); ok {
		_, err = approved.DeleteTaskApproved(ctx, userID, taskID, expected, ids)
	} else {
		_, err = c.Tasks.Delete(ctx, userID, taskID, expected, true)
	}
	if errors.Is(err, domain.ErrImpactPlanChanged) {
		return ErrInvalidImpactToken
	}
	return err
}

// 葉タスク進捗の更新（進行中の葉のみ、0..95・5刻み）
func (c TaskCommands) SetProgress(ctx context.Context, userID, taskID string, expected int64, progress int, via string) (domain.Task, error) {
	if userID == "" || taskID == "" || expected < 1 {
		return domain.Task{}, domain.ErrForbidden
	}
	current, err := c.Tasks.Get(ctx, userID, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if current.Version != expected {
		return domain.Task{}, domain.ErrVersionConflict
	}
	if err := domain.ValidateLeafProgress(current, progress); err != nil {
		return domain.Task{}, err
	}
	repository, ok := c.Tasks.Repo.(LeafProgressRepository)
	if !ok {
		return domain.Task{}, errors.New("leaf progress repository unavailable")
	}
	return repository.UpdateLeafProgress(ctx, userID, taskID, expected, progress, via)
}

// 復元可能な削除済みタスク一覧
func (c TaskCommands) Deleted(ctx context.Context, userID string) ([]domain.DeletedTask, error) {
	if userID == "" {
		return nil, domain.ErrForbidden
	}
	repository, ok := c.Tasks.Repo.(RestoreRepository)
	if !ok {
		return nil, errors.New("restore repository unavailable")
	}
	return repository.ListDeletedTasks(ctx, userID)
}

// 削除済みタスクの復元（同一削除操作の集合ごと）
func (c TaskCommands) Restore(ctx context.Context, userID, taskID string) ([]string, error) {
	if userID == "" || taskID == "" {
		return nil, domain.ErrForbidden
	}
	repository, ok := c.Tasks.Repo.(RestoreRepository)
	if !ok {
		return nil, errors.New("restore repository unavailable")
	}
	return repository.RestoreTask(ctx, userID, taskID)
}

// 影響確認エラーの組み立て（タイトル補完・トークン発行）
func (c TaskCommands) impactRequired(ctx context.Context, userID string, claims ImpactClaims, entities []ImpactEntity, warnings []string) error {
	token, err := c.Tokens.Issue(claims)
	if err != nil {
		return err
	}
	// Taskエンティティのタイトル補完（フロントでIDだけを見せない）
	if tasks, searchErr := c.Tasks.Search(ctx, userID, domain.TaskSearch{}); searchErr == nil {
		titles := make(map[string]string, len(tasks))
		for _, task := range tasks {
			titles[task.ID] = task.Title
		}
		for index := range entities {
			if entities[index].Title == nil && entities[index].Type == "TASK" {
				if title, ok := titles[entities[index].ID]; ok {
					entities[index].Title = stringRef(title)
				}
			}
		}
	}
	return &ImpactRequiredError{ImpactToken: token, AffectedEntities: entities, Warnings: warnings}
}

// 日程矛盾の警告算出（失敗しても更新結果は変えない）
func (c TaskCommands) scheduleWarnings(ctx context.Context, userID string, task domain.Task) []string {
	if c.Relations.Repo == nil {
		return nil
	}
	_, dependencies, err := c.Relations.Snapshot(ctx, userID)
	if err != nil || len(dependencies) == 0 {
		return nil
	}
	tasks, err := c.Tasks.Search(ctx, userID, domain.TaskSearch{GenreIDs: []string{task.GenreID}})
	if err != nil {
		return nil
	}
	return domain.DependencyScheduleWarnings(task, tasks, dependencies)
}

// 未完了の前提タスク
func (c TaskCommands) unmetPrerequisites(ctx context.Context, userID string, task domain.Task) ([]domain.Task, error) {
	_, dependencies, err := c.Relations.Snapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	hasEdge := false
	for _, edge := range dependencies {
		hasEdge = hasEdge || edge.TaskID == task.ID
	}
	if !hasEdge {
		return nil, nil
	}
	tasks, err := c.Tasks.Search(ctx, userID, domain.TaskSearch{GenreIDs: []string{task.GenreID}})
	if err != nil {
		return nil, err
	}
	return domain.UnmetPrerequisites(task.ID, tasks, dependencies), nil
}

// 日付伝播計画用の木（付け替え時は新しい親の木＋自分の部分木）
func (c TaskCommands) dateTreeFor(ctx context.Context, userID string, current, next domain.Task, parentChanged bool) ([]domain.DateTask, error) {
	tree, err := c.Tasks.Tree(ctx, userID, current.ID)
	if err != nil {
		return nil, err
	}
	if !parentChanged {
		return toDateTasks(tree), nil
	}
	// 自分の部分木の抽出
	children := map[string][]domain.Task{}
	for _, task := range tree {
		if task.ParentTaskID != nil {
			children[*task.ParentTaskID] = append(children[*task.ParentTaskID], task)
		}
	}
	subtree := []domain.Task{}
	queue := []domain.Task{current}
	for len(queue) > 0 {
		task := queue[0]
		queue = queue[1:]
		if task.ID == current.ID {
			task.ParentTaskID = next.ParentTaskID
		}
		subtree = append(subtree, task)
		queue = append(queue, children[task.ID]...)
	}
	if next.ParentTaskID == nil {
		return toDateTasks(subtree), nil
	}
	parentTree, err := c.Tasks.Tree(ctx, userID, *next.ParentTaskID)
	if err != nil {
		return nil, err
	}
	inSubtree := map[string]bool{}
	for _, task := range subtree {
		inSubtree[task.ID] = true
	}
	if inSubtree[*next.ParentTaskID] {
		return nil, domain.ErrHierarchyCycle
	}
	combined := append([]domain.Task(nil), subtree...)
	for _, task := range parentTree {
		if !inSubtree[task.ID] {
			combined = append(combined, task)
		}
	}
	return toDateTasks(combined), nil
}

// 日付伝播計画用タスク列への変換
func toDateTasks(tree []domain.Task) []domain.DateTask {
	dateTasks := make([]domain.DateTask, 0, len(tree))
	for _, task := range tree {
		dateTasks = append(dateTasks, domain.DateTask{ID: task.ID, ParentID: stringValue(task.ParentTaskID), StartAt: task.StartAt, EndAt: task.EndAt})
	}
	return dateTasks
}

// 更新影響トークン用操作識別子
func updateImpactOperation(next domain.Task, move domain.GenreMovePlan, dates []domain.DateChange, includeMove, includeDates bool, leafParent *domain.Task) (string, error) {
	payload := struct {
		GenreID    string               `json:"genreId,omitempty"`
		Move       domain.GenreMovePlan `json:"move,omitempty"`
		Dates      []domain.DateChange  `json:"dates,omitempty"`
		LeafParent string               `json:"leafParent,omitempty"`
		LeafVer    int64                `json:"leafVersion,omitempty"`
	}{GenreID: next.GenreID}
	if includeMove {
		payload.Move = move
	}
	if includeDates {
		payload.Dates = dates
	}
	if leafParent != nil {
		payload.LeafParent, payload.LeafVer = leafParent.ID, leafParent.Version
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "UPDATE_TASK_IMPACT:" + hex.EncodeToString(digest[:]), nil
}

// 更新影響の対象列挙
func updateImpactEntities(changes []domain.DateChange, move domain.GenreMovePlan, dates, genre bool, leafParent *domain.Task) []ImpactEntity {
	entities := []ImpactEntity{}
	if dates {
		for _, change := range changes {
			entities = append(entities, ImpactEntity{ID: change.TaskID, Type: "TASK", Reason: stringRef("DATE_PROPAGATION")})
		}
	}
	if genre {
		for _, id := range move.MovedTaskIDs {
			entities = append(entities, ImpactEntity{ID: id, Type: "TASK", Reason: stringRef("GENRE_MOVE")})
		}
		for _, dependency := range move.RemovedDependencies {
			entities = append(entities, ImpactEntity{ID: dependency.TaskID + "->" + dependency.DependsOnTaskID, Type: "DEPENDENCY", Reason: stringRef("GENRE_BOUNDARY_REMOVAL")})
		}
	}
	if leafParent != nil {
		entities = append(entities, ImpactEntity{ID: leafParent.ID, Type: "TASK", Title: stringRef(leafParent.Title), Reason: stringRef("LEAF_TO_PARENT")})
	}
	return entities
}

// 更新影響の警告文
func updateImpactWarnings(dates, genre bool, leafParent *domain.Task) []string {
	warnings := []string{}
	if dates {
		warnings = append(warnings, "日付変更により関連タスクの日付も変更されます。")
	}
	if genre {
		warnings = append(warnings, "Genre移動により以下の依存関係が解除されます。")
	}
	if leafParent != nil {
		warnings = append(warnings, "進捗率は自動で再計算されます。子タスクを追加しますか？")
	}
	return warnings
}

// 文字列ポインタの生成
func stringRef(value string) *string { return &value }

// nil文字列の空文字化
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
