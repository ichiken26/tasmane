package mcp

import (
	"context"
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
