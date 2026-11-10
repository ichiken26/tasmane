package mcp

import (
	"context"
	"errors"
	"strings"
	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
	"testing"
)

// 削除ツール非公開と get_task 共有ユースケース利用の検証
func TestToolsExposeNoDeleteAndGetUsesSharedTaskUseCase(t *testing.T) {
	tools, err := Tools()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name), "delete") {
			t.Fatalf("delete tool exposed: %s", tool.Name)
		}
		if tool.InputSchema["type"] != "object" || tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("invalid schema for %s: %#v", tool.Name, tool.InputSchema)
		}
		if !supportedTools[tool.Name] {
			t.Fatalf("manifest tool has no implementation: %s", tool.Name)
		}
	}
	store := repository.NewMemory(domain.Task{ID: "t", UserID: "u", GenreID: "g", Title: "Task", Status: domain.NotStarted, Version: 1})
	server := Server{UserID: "u", Services: Services{Tasks: application.Tasks{Repo: store}}}
	args := []byte(`{"taskId":"t"}`)
	got, err := server.call(context.Background(), "get_task", args)
	// OpenAPI Task スキーマ相当の算出値付き表現で返す
	if err != nil || got.(application.TaskView).ID != "t" {
		t.Fatal(got, err)
	}
}

// 影響確認必須時の非破壊遷移と不正トークン拒否の検証
func TestTransitionToolReturnsConfirmationRequiredWithoutMutating(t *testing.T) {
	store := repository.NewMemory(domain.Task{ID: "t", UserID: "u", Status: domain.Completed, Progress: mcpInt(100), Version: 1})
	server := Server{UserID: "u", Services: Services{
		Tasks:        application.Tasks{Repo: store},
		ImpactTokens: application.ImpactTokens{Secret: []byte("01234567890123456789012345678901")},
	}}
	got, err := server.call(context.Background(), "transition_task", []byte(`{"taskId":"t","toStatus":"IN_PROGRESS","expectedVersion":1}`))
	if err != nil {
		t.Fatal(err)
	}
	impact, ok := got.(map[string]any)
	if !ok || impact["status"] != "confirmation_required" || impact["impactToken"] == "" {
		t.Fatalf("unexpected impact: %#v", got)
	}
	if store.Tasks["t"].Status != domain.Completed {
		t.Fatal("MCP auto-approved rollback")
	}
	_, err = server.call(context.Background(), "transition_task", []byte(`{"taskId":"t","toStatus":"IN_PROGRESS","expectedVersion":1,"impactToken":"invalid"}`))
	if !errors.Is(err, application.ErrInvalidImpactToken) {
		t.Fatalf("invalid token: %v", err)
	}
}

// 整数ポインタのテストヘルパ
func mcpInt(value int) *int { return &value }

// update_task は影響確認が必要な付け替えで confirmation_required を返し、状態・進捗は変更不可
func TestUpdateToolRequiresConfirmationAndRejectsStatusPatch(t *testing.T) {
	store := repository.NewMemory(
		domain.Task{ID: "leaf", UserID: "u", GenreID: "g", Title: "葉", Status: domain.InProgress, Version: 1},
		domain.Task{ID: "t", UserID: "u", GenreID: "g", Title: "Task", Status: domain.NotStarted, IsAllDay: true, Version: 1},
	)
	server := Server{UserID: "u", Services: Services{
		Tasks:        application.Tasks{Repo: store},
		ImpactTokens: application.ImpactTokens{Secret: []byte("01234567890123456789012345678901")},
	}}
	got, err := server.call(context.Background(), "update_task", []byte(`{"taskId":"t","expectedVersion":1,"patch":{"parentTaskId":"leaf"}}`))
	if err != nil {
		t.Fatal(err)
	}
	impact, ok := got.(map[string]any)
	if !ok || impact["status"] != "confirmation_required" {
		t.Fatalf("unexpected: %#v", got)
	}
	if store.Tasks["t"].ParentTaskID != nil {
		t.Fatal("MCP auto-approved leaf conversion")
	}
	if _, err := server.call(context.Background(), "update_task", []byte(`{"taskId":"t","expectedVersion":1,"patch":{"status":"COMPLETED"}}`)); !errors.Is(err, domain.ErrInvalidTaskInput) {
		t.Fatalf("status patch must be rejected: %v", err)
	}
}
