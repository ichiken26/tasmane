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
	if err != nil || got.(domain.Task).ID != "t" {
		t.Fatal(got, err)
	}
}

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

func mcpInt(value int) *int { return &value }
