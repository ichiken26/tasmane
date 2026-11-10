package mcp

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"tasmane/internal/application"
	"tasmane/internal/domain"
	"time"
)

// JSON Schema 断片の型種別変換
func schemaFor(kind string) map[string]any {
	switch kind {
	case "uuid":
		return map[string]any{"type": "string", "format": "uuid"}
	case "uuid[]":
		return map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "uuid"}}
	case "int":
		return map[string]any{"type": "integer", "minimum": 1}
	case "datetime":
		return map[string]any{"type": "string", "format": "date-time"}
	case "object":
		return map[string]any{"type": "object"}
	case "progress":
		return map[string]any{"type": "integer", "minimum": 0, "maximum": 95, "multipleOf": 5}
	default:
		if strings.Contains(kind, "|") {
			return map[string]any{"type": "string", "enum": strings.Split(kind, "|")}
		}
		return map[string]any{"type": "string"}
	}
}

//go:embed tools.json
var files embed.FS

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}
type manifestTool struct {
	Name     string            `json:"name"`
	Action   string            `json:"action"`
	Input    map[string]string `json:"input,omitempty"`
	InputRef string            `json:"inputRef,omitempty"`
}
type manifest struct {
	Tools []manifestTool `json:"tools"`
}

// MCP ツールマニフェストの JSON Schema 化
func Tools() ([]Tool, error) {
	body, e := files.ReadFile("tools.json")
	if e != nil {
		return nil, e
	}
	var m manifest
	if e = json.Unmarshal(body, &m); e != nil {
		return nil, e
	}
	tools := make([]Tool, 0, len(m.Tools))
	// 各ツールの入力スキーマ組み立て
	for _, item := range m.Tools {
		properties := map[string]any{}
		required := []string{}
		if item.InputRef != "" {
			properties = map[string]any{
				"title": map[string]any{"type": "string", "minLength": 1}, "genreId": map[string]any{"type": "string", "format": "uuid"},
				"body": map[string]any{"type": []string{"string", "null"}}, "taskTypeId": map[string]any{"type": []string{"string", "null"}, "format": "uuid"},
				"parentTaskId": map[string]any{"type": []string{"string", "null"}, "format": "uuid"}, "startAt": map[string]any{"type": []string{"string", "null"}, "format": "date-time"},
				"endAt": map[string]any{"type": []string{"string", "null"}, "format": "date-time"}, "isAllDay": map[string]any{"type": "boolean"},
				"manualPriority": map[string]any{"type": []string{"string", "null"}, "enum": []string{"LOW", "MEDIUM", "HIGH"}}, "impactToken": map[string]any{"type": []string{"string", "null"}},
			}
			required = []string{"title", "genreId"}
		} else {
			for name, kind := range item.Input {
				optional := strings.HasSuffix(kind, "?")
				kind = strings.TrimSuffix(kind, "?")
				schema := schemaFor(kind)
				properties[name] = schema
				if !optional {
					required = append(required, name)
				}
			}
			sort.Strings(required)
		}
		input := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			input["required"] = required
		}
		tools = append(tools, Tool{Name: item.Name, Description: item.Action + " Tasmane data through shared domain rules", InputSchema: input})
	}
	return tools, nil
}

type Services struct {
	Tasks         application.Tasks
	Catalog       application.Catalog
	Relations     application.Relations
	Confirmations application.Confirmations
	Google        application.GoogleJobs
	GoogleQueries application.GoogleQueries
	ImpactTokens  application.ImpactTokens
}
type Server struct {
	UserID   string
	Services Services
}

var supportedTools = map[string]bool{"get_task": true, "search_tasks": true, "get_task_tree": true, "get_task_dependencies": true, "create_task": true, "update_task": true, "transition_task": true, "create_confirmation": true, "update_confirmation": true, "list_confirmations": true, "add_dependency": true, "list_genres": true, "get_genre": true, "create_genre": true, "update_genre": true, "list_task_types": true, "create_task_type": true, "update_task_type": true, "list_genre_task_types": true, "attach_genre_task_type": true, "get_dashboard": true, "get_genre_dashboard": true, "set_task_progress": true, "list_confirmation_targets": true, "create_confirmation_target": true, "trigger_google_sync": true, "list_google_conflicts": true, "list_google_import_candidates": true}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 行単位の stdio 待受
func (s Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 1024), 1<<20)
	enc := json.NewEncoder(out)
	for scan.Scan() {
		var req request
		if json.Unmarshal(scan.Bytes(), &req) != nil {
			continue
		}
		res := s.Handle(ctx, req)
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return scan.Err()
}

// JSON-RPC メソッドの振り分け
func (s Server) Handle(ctx context.Context, req request) response {
	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{"protocolVersion": "2025-03-26", "serverInfo": map[string]string{"name": "tasmane", "version": "0.1.0"}, "capabilities": map[string]any{"tools": map[string]any{}}}
	case "tools/list":
		tools, e := Tools()
		if e != nil {
			res.Error = &rpcError{-32603, e.Error()}
		} else {
			res.Result = map[string]any{"tools": tools}
		}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		// ツール呼び出し引数のデコードと実行
		if json.Unmarshal(req.Params, &p) != nil {
			res.Error = &rpcError{-32602, "invalid params"}
			break
		}
		value, e := s.call(ctx, p.Name, p.Arguments)
		if e != nil {
			res.Result = map[string]any{"content": []map[string]string{{"type": "text", "text": e.Error()}}, "isError": true}
		} else {
			body, _ := json.Marshal(value)
			res.Result = map[string]any{"content": []map[string]string{{"type": "text", "text": string(body)}}}
		}
	default:
		res.Error = &rpcError{-32601, "method not found"}
	}
	return res
}

// MCP ツール名に応じたユースケース呼び出し
func (s Server) call(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if !supportedTools[name] {
		return nil, fmt.Errorf("unsupported tool %q", name)
	}
	if s.UserID == "" {
		return nil, domain.ErrForbidden
	}
	var a map[string]json.RawMessage
	if len(raw) > 0 {
		if e := json.Unmarshal(raw, &a); e != nil {
			return nil, e
		}
	}
	str := func(k string) string { var v string; _ = json.Unmarshal(a[k], &v); return v }
	integer := func(k string) int64 { var v int64; _ = json.Unmarshal(a[k], &v); return v }
	commands := application.TaskCommands{Tasks: s.Services.Tasks, Relations: s.Services.Relations, Tokens: s.Services.ImpactTokens}
	projections := application.Projections{Tasks: s.Services.Tasks, Relations: s.Services.Relations, Confirmations: s.Services.Confirmations}
	// 影響確認が必要な操作はAgentが承認を代行せずconfirmation_requiredを返す
	confirmation := func(err error) (any, error) {
		var impact *application.ImpactRequiredError
		if errors.As(err, &impact) {
			return map[string]any{"status": "confirmation_required", "impactToken": impact.ImpactToken, "affectedEntities": impact.AffectedEntities, "warnings": impact.Warnings,
				"instruction": "ユーザーに影響範囲を提示し、明示的な承認を得た場合のみ impactToken を付けて同じ操作を再実行してください。"}, nil
		}
		return nil, err
	}
	taskView := func(task domain.Task, err error) (any, error) {
		if err != nil {
			return confirmation(err)
		}
		projected, projectErr := projections.ProjectTask(ctx, s.UserID, task)
		if projectErr != nil {
			return nil, projectErr
		}
		return application.NewTaskView(projected), nil
	}
	switch name {
	case "get_task":
		return taskView(s.Services.Tasks.Get(ctx, s.UserID, str("taskId")))
	case "search_tasks":
		var genres []string
		_ = json.Unmarshal(a["genreIds"], &genres)
		var priority *domain.Priority
		if v := str("priority"); v != "" {
			p := domain.Priority(v)
			priority = &p
		}
		values, err := projections.Search(ctx, s.UserID, application.ProjectedSearch{Query: str("q"), GenreIDs: genres, Priority: priority, Deadline: str("deadline"), Sort: str("sort")})
		if err != nil {
			return nil, err
		}
		return application.NewTaskViews(values), nil
	case "get_task_tree":
		tree, err := s.Services.Tasks.Tree(ctx, s.UserID, str("taskId"))
		if err != nil || len(tree) == 0 {
			return []application.TaskView{}, err
		}
		ids := make([]string, 0, len(tree))
		for _, task := range tree {
			ids = append(ids, task.ID)
		}
		values, err := projections.ProjectIDs(ctx, s.UserID, []string{tree[0].GenreID}, ids)
		if err != nil {
			return nil, err
		}
		return application.NewTaskViews(values), nil
	case "get_task_dependencies":
		values, err := s.Services.Relations.ListDependencies(ctx, s.UserID, str("taskId"))
		if err != nil {
			return nil, err
		}
		result := make([]application.DependencyView, 0, len(values))
		for _, value := range values {
			result = append(result, application.DependencyView{TaskID: value.TaskID, DependsOnTaskID: value.DependsOnTaskID})
		}
		return result, nil
	case "transition_task":
		return taskView(commands.Transition(ctx, s.UserID, str("taskId"), integer("expectedVersion"), domain.Status(str("toStatus")), "MCP", str("impactToken")))
	case "set_task_progress":
		return taskView(commands.SetProgress(ctx, s.UserID, str("taskId"), integer("expectedVersion"), int(integer("progress")), "MCP"))
	case "add_dependency":
		return nil, s.Services.Relations.AddDependency(ctx, s.UserID, str("taskId"), str("dependsOnTaskId"), integer("expectedVersion"))
	case "list_confirmations":
		values, err := s.Services.Confirmations.List(ctx, s.UserID, str("taskId"))
		if err != nil {
			return nil, err
		}
		result := make([]application.ConfirmationView, 0, len(values))
		for _, value := range values {
			result = append(result, application.NewConfirmationView(value))
		}
		return result, nil
	case "create_confirmation":
		var due *time.Time
		if v := str("dueAt"); v != "" {
			parsed, e := time.Parse(time.RFC3339, v)
			if e != nil {
				return nil, domain.ErrInvalidConfirmationInput
			}
			due = &parsed
		}
		value, err := s.Services.Confirmations.Create(ctx, s.UserID, str("taskId"), str("content"), str("targetName"), due)
		if err != nil {
			return nil, err
		}
		return application.NewConfirmationView(value), nil
	case "update_confirmation":
		value, err := s.Services.Confirmations.Transition(ctx, s.UserID, str("confirmationId"), domain.ConfirmationStatus(str("status")), time.Now().UTC())
		if err != nil {
			return nil, err
		}
		return application.NewConfirmationView(value), nil
	case "list_confirmation_targets", "create_confirmation_target":
		repository, _ := s.Services.Confirmations.Repo.(application.ConfirmationTargetRepository)
		targets := application.ConfirmationTargets{Repo: repository}
		if name == "create_confirmation_target" {
			value, err := targets.Create(ctx, s.UserID, str("name"))
			if err != nil {
				return nil, err
			}
			return application.NamedView{ID: value.ID, Name: value.Name}, nil
		}
		values, err := targets.List(ctx, s.UserID)
		if err != nil {
			return nil, err
		}
		result := make([]application.NamedView, 0, len(values))
		for _, value := range values {
			result = append(result, application.NamedView{ID: value.ID, Name: value.Name})
		}
		return result, nil
	case "list_genres":
		return namedGenres(s.Services.Catalog.ListGenres(ctx, s.UserID))
	case "get_genre":
		return namedGenre(s.Services.Catalog.GetGenre(ctx, s.UserID, str("genreId")))
	case "create_genre":
		return namedGenre(s.Services.Catalog.CreateGenre(ctx, s.UserID, str("name")))
	case "update_genre":
		return namedGenre(s.Services.Catalog.RenameGenre(ctx, s.UserID, str("genreId"), str("name")))
	case "list_task_types":
		return namedTaskTypes(s.Services.Catalog.ListTaskTypes(ctx, s.UserID))
	case "create_task_type":
		return namedTaskType(s.Services.Catalog.CreateTaskType(ctx, s.UserID, str("name")))
	case "update_task_type":
		return namedTaskType(s.Services.Catalog.RenameTaskType(ctx, s.UserID, str("taskTypeId"), str("name")))
	case "list_genre_task_types":
		return namedTaskTypes(s.Services.Catalog.ListGenreTaskTypes(ctx, s.UserID, str("genreId")))
	case "attach_genre_task_type":
		return nil, s.Services.Catalog.AttachGenreTaskType(ctx, s.UserID, str("genreId"), str("taskTypeId"))
	case "trigger_google_sync":
		return nil, s.Services.Google.Trigger(ctx, s.UserID)
	case "list_google_conflicts":
		return s.Services.GoogleQueries.Conflicts(ctx, s.UserID)
	case "list_google_import_candidates":
		return s.Services.GoogleQueries.ImportCandidates(ctx, s.UserID)
	case "create_task":
		var input struct {
			Title          string           `json:"title"`
			GenreID        string           `json:"genreId"`
			Body           *string          `json:"body"`
			TaskTypeID     *string          `json:"taskTypeId"`
			ParentTaskID   *string          `json:"parentTaskId"`
			StartAt        *time.Time       `json:"startAt"`
			EndAt          *time.Time       `json:"endAt"`
			IsAllDay       *bool            `json:"isAllDay"`
			ManualPriority *domain.Priority `json:"manualPriority"`
			ImpactToken    *string          `json:"impactToken"`
		}
		if json.Unmarshal(raw, &input) != nil {
			return nil, domain.ErrInvalidTaskInput
		}
		isAllDay := input.IsAllDay == nil || *input.IsAllDay
		token := ""
		if input.ImpactToken != nil {
			token = *input.ImpactToken
		}
		return taskView(commands.Create(ctx, s.UserID, domain.TaskDraft{Title: input.Title, GenreID: input.GenreID, Body: input.Body, TaskTypeID: input.TaskTypeID, ParentTaskID: input.ParentTaskID, StartAt: input.StartAt, EndAt: input.EndAt, IsAllDay: isAllDay, ManualPriority: input.ManualPriority}, token))
	case "update_task":
		var patch map[string]json.RawMessage
		if err := json.Unmarshal(a["patch"], &patch); err != nil {
			return nil, domain.ErrInvalidTaskInput
		}
		result, err := commands.Update(ctx, s.UserID, str("taskId"), integer("expectedVersion"), func(current domain.Task) (domain.Task, error) {
			return applyPatch(current, patch)
		}, str("impactToken"))
		if err != nil {
			return confirmation(err)
		}
		view, err := taskView(result.Task, nil)
		if err != nil {
			return nil, err
		}
		taskValue := view.(application.TaskView)
		taskValue.Warnings = result.Warnings
		return taskValue, nil
	case "get_dashboard", "get_genre_dashboard":
		var genres []string
		if name == "get_genre_dashboard" {
			genres = []string{str("genreId")}
		}
		result, err := projections.Project(ctx, s.UserID, genres)
		if err != nil {
			return nil, err
		}
		return map[string]any{"recommendedTasks": application.NewTaskViews(result.Tasks)}, nil
	default:
		return nil, fmt.Errorf("unsupported tool %q", name)
	}
}

// update_task パッチの適用（null は任意項目の解除）
func applyPatch(current domain.Task, patch map[string]json.RawMessage) (domain.Task, error) {
	next := current
	isNull := func(value json.RawMessage) bool { return string(value) == "null" }
	for key, value := range patch {
		var err error
		switch key {
		case "title":
			err = json.Unmarshal(value, &next.Title)
		case "body":
			next.Body = nil
			if !isNull(value) {
				err = json.Unmarshal(value, &next.Body)
			}
		case "genreId":
			err = json.Unmarshal(value, &next.GenreID)
		case "taskTypeId":
			next.TaskTypeID = nil
			if !isNull(value) {
				err = json.Unmarshal(value, &next.TaskTypeID)
			}
		case "parentTaskId":
			next.ParentTaskID = nil
			if !isNull(value) {
				err = json.Unmarshal(value, &next.ParentTaskID)
			}
		case "startAt":
			next.StartAt = nil
			if !isNull(value) {
				err = json.Unmarshal(value, &next.StartAt)
			}
		case "endAt":
			next.EndAt = nil
			if !isNull(value) {
				err = json.Unmarshal(value, &next.EndAt)
			}
		case "isAllDay":
			err = json.Unmarshal(value, &next.IsAllDay)
		case "manualPriority":
			next.ManualPriority = nil
			if !isNull(value) {
				err = json.Unmarshal(value, &next.ManualPriority)
			}
		default:
			// 進捗・状態は専用ツールでのみ変更（ドメイン規則の迂回防止）
			return domain.Task{}, domain.ErrInvalidTaskInput
		}
		if err != nil {
			return domain.Task{}, domain.ErrInvalidTaskInput
		}
	}
	return next, nil
}

// ジャンル表現への変換
func namedGenre(value domain.Genre, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return application.NamedView{ID: value.ID, Name: value.Name}, nil
}

// ジャンル一覧表現への変換
func namedGenres(values []domain.Genre, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	result := make([]application.NamedView, 0, len(values))
	for _, value := range values {
		result = append(result, application.NamedView{ID: value.ID, Name: value.Name})
	}
	return result, nil
}

// タスク種別表現への変換
func namedTaskType(value domain.TaskType, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return application.NamedView{ID: value.ID, Name: value.Name}, nil
}

// タスク種別一覧表現への変換
func namedTaskTypes(values []domain.TaskType, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	result := make([]application.NamedView, 0, len(values))
	for _, value := range values {
		result = append(result, application.NamedView{ID: value.ID, Name: value.Name})
	}
	return result, nil
}

var _ = errors.Is
