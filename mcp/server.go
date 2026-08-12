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
	for _, item := range m.Tools {
		properties := map[string]any{}
		required := []string{}
		if item.InputRef != "" {
			properties = map[string]any{"title": map[string]any{"type": "string", "minLength": 1}, "genreId": map[string]any{"type": "string", "format": "uuid"}}
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
}
type Server struct {
	UserID   string
	Services Services
}

var supportedTools = map[string]bool{"get_task": true, "search_tasks": true, "get_task_tree": true, "get_task_dependencies": true, "create_task": true, "update_task": true, "transition_task": true, "create_confirmation": true, "update_confirmation": true, "list_confirmations": true, "add_dependency": true, "list_genres": true, "get_genre": true, "create_genre": true, "update_genre": true, "list_task_types": true, "create_task_type": true, "update_task_type": true, "list_genre_task_types": true, "attach_genre_task_type": true, "get_dashboard": true, "get_genre_dashboard": true, "trigger_google_sync": true, "list_google_conflicts": true, "list_google_import_candidates": true}

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
	switch name {
	case "get_task":
		return s.Services.Tasks.Get(ctx, s.UserID, str("taskId"))
	case "search_tasks":
		var genres []string
		_ = json.Unmarshal(a["genreIds"], &genres)
		var priority *domain.Priority
		if v := str("priority"); v != "" {
			p := domain.Priority(v)
			priority = &p
		}
		return s.Services.Tasks.Search(ctx, s.UserID, domain.TaskSearch{Query: str("q"), GenreIDs: genres, Priority: priority, Deadline: str("deadline")})
	case "get_task_tree":
		return s.Services.Tasks.Tree(ctx, s.UserID, str("taskId"))
	case "get_task_dependencies":
		return s.Services.Relations.ListDependencies(ctx, s.UserID, str("taskId"))
	case "transition_task":
		return s.Services.Tasks.Transition(ctx, s.UserID, str("taskId"), integer("expectedVersion"), domain.Status(str("toStatus")), "MCP")
	case "add_dependency":
		return nil, s.Services.Relations.AddDependency(ctx, s.UserID, str("taskId"), str("dependsOnTaskId"), integer("expectedVersion"))
	case "list_confirmations":
		return s.Services.Confirmations.List(ctx, s.UserID, str("taskId"))
	case "create_confirmation":
		var due *time.Time
		if v := str("dueAt"); v != "" {
			parsed, e := time.Parse(time.RFC3339, v)
			if e != nil {
				return nil, e
			}
			due = &parsed
		}
		return s.Services.Confirmations.Create(ctx, s.UserID, str("taskId"), str("content"), str("targetName"), due)
	case "update_confirmation":
		return s.Services.Confirmations.Transition(ctx, s.UserID, str("confirmationId"), domain.ConfirmationStatus(str("status")), time.Now().UTC())
	case "list_genres":
		return s.Services.Catalog.ListGenres(ctx, s.UserID)
	case "get_genre":
		return s.Services.Catalog.GetGenre(ctx, s.UserID, str("genreId"))
	case "create_genre":
		return s.Services.Catalog.CreateGenre(ctx, s.UserID, str("name"))
	case "update_genre":
		return s.Services.Catalog.RenameGenre(ctx, s.UserID, str("genreId"), str("name"))
	case "list_task_types":
		return s.Services.Catalog.ListTaskTypes(ctx, s.UserID)
	case "create_task_type":
		return s.Services.Catalog.CreateTaskType(ctx, s.UserID, str("name"))
	case "update_task_type":
		return s.Services.Catalog.RenameTaskType(ctx, s.UserID, str("taskTypeId"), str("name"))
	case "list_genre_task_types":
		return s.Services.Catalog.ListGenreTaskTypes(ctx, s.UserID, str("genreId"))
	case "attach_genre_task_type":
		return nil, s.Services.Catalog.AttachGenreTaskType(ctx, s.UserID, str("genreId"), str("taskTypeId"))
	case "trigger_google_sync":
		return nil, s.Services.Google.Enqueue(ctx, s.UserID, "", application.GooglePullUser)
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
			IsAllDay       bool             `json:"isAllDay"`
			ManualPriority *domain.Priority `json:"manualPriority"`
		}
		if json.Unmarshal(raw, &input) != nil {
			return nil, domain.ErrInvalidTaskInput
		}
		return s.Services.Tasks.Create(ctx, s.UserID, domain.TaskDraft{Title: input.Title, GenreID: input.GenreID, Body: input.Body, TaskTypeID: input.TaskTypeID, ParentTaskID: input.ParentTaskID, StartAt: input.StartAt, EndAt: input.EndAt, IsAllDay: input.IsAllDay, ManualPriority: input.ManualPriority}, domain.TaskCreateAuthorization{})
	case "update_task":
		current, e := s.Services.Tasks.Get(ctx, s.UserID, str("taskId"))
		if e != nil {
			return nil, e
		}
		var patch struct {
			Title *string `json:"title"`
		}
		_ = json.Unmarshal(a["patch"], &patch)
		if patch.Title != nil {
			current.Title = *patch.Title
		}
		return s.Services.Tasks.Update(ctx, s.UserID, domain.TaskUpdate{Task: current}, integer("expectedVersion"))
	case "get_dashboard":
		return s.Services.Tasks.Search(ctx, s.UserID, domain.TaskSearch{})
	case "get_genre_dashboard":
		return s.Services.Tasks.Search(ctx, s.UserID, domain.TaskSearch{GenreIDs: []string{str("genreId")}})
	default:
		return nil, fmt.Errorf("unsupported tool %q", name)
	}
}

var _ = errors.Is
