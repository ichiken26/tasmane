package domain

import (
	"errors"
	"testing"
)

// 階層割当の境界と循環検証
func TestValidateHierarchyAssignment(t *testing.T) {
	nodes := []GraphTask{
		{ID: "root", UserID: "u1", GenreID: "g1"},
		{ID: "child", UserID: "u1", GenreID: "g1", ParentID: "root"},
		{ID: "grandchild", UserID: "u1", GenreID: "g1", ParentID: "child"},
		{ID: "other-user", UserID: "u2", GenreID: "g1"},
		{ID: "other-genre", UserID: "u1", GenreID: "g2"},
	}
	cases := []struct {
		name, task, parent string
		want               error
	}{
		{"valid move", "grandchild", "root", nil},
		{"detach", "child", "", nil},
		{"self", "root", "root", ErrHierarchyCycle},
		{"descendant", "root", "grandchild", ErrHierarchyCycle},
		{"user boundary", "root", "other-user", ErrGraphOwnership},
		{"genre boundary", "root", "other-genre", ErrGraphGenre},
		{"missing parent", "root", "missing", ErrGraphTaskNotFound},
		{"empty task", "", "root", ErrInvalidGraphInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateHierarchyAssignment(nodes, tc.task, tc.parent); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

// 不正タスク集合のグラフ検証拒否
func TestGraphValidationRejectsMalformedTaskSets(t *testing.T) {
	duplicate := []GraphTask{{ID: "a", UserID: "u", GenreID: "g"}, {ID: "a", UserID: "u", GenreID: "g"}}
	if err := ValidateHierarchyAssignment(duplicate, "a", ""); !errors.Is(err, ErrInvalidGraphInput) {
		t.Fatal(err)
	}
	missingOwner := []GraphTask{{ID: "a", GenreID: "g"}}
	if err := ValidateHierarchyAssignment(missingOwner, "a", ""); !errors.Is(err, ErrInvalidGraphInput) {
		t.Fatal(err)
	}
}

// 依存追加の境界と循環検証
func TestValidateDependencyAddition(t *testing.T) {
	tasks := []GraphTask{
		{ID: "a", UserID: "u1", GenreID: "g1"},
		{ID: "b", UserID: "u1", GenreID: "g1"},
		{ID: "c", UserID: "u1", GenreID: "g1"},
		{ID: "u2", UserID: "u2", GenreID: "g1"},
		{ID: "g2", UserID: "u1", GenreID: "g2"},
	}
	edges := []Dependency{{TaskID: "a", DependsOnTaskID: "b"}, {TaskID: "b", DependsOnTaskID: "c"}}
	cases := []struct {
		name, task, dependency string
		want                   error
	}{
		{"valid", "a", "c", nil},
		{"self", "a", "a", ErrDependencyCycle},
		{"indirect cycle", "c", "a", ErrDependencyCycle},
		{"user boundary", "a", "u2", ErrGraphOwnership},
		{"genre boundary", "a", "g2", ErrGraphGenre},
		{"missing", "a", "missing", ErrGraphTaskNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDependencyAddition(tasks, edges, tc.task, tc.dependency); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

// 不正既存辺の依存検証拒否
func TestDependencyValidationRejectsMalformedExistingEdges(t *testing.T) {
	tasks := []GraphTask{
		{ID: "a", UserID: "u1", GenreID: "g1"},
		{ID: "b", UserID: "u1", GenreID: "g1"},
		{ID: "c", UserID: "u1", GenreID: "g1"},
		{ID: "foreign", UserID: "u2", GenreID: "g1"},
	}
	cases := []struct {
		name  string
		edges []Dependency
		want  error
	}{
		{"empty endpoint", []Dependency{{TaskID: "", DependsOnTaskID: "b"}}, ErrInvalidGraphInput},
		{"missing endpoint", []Dependency{{TaskID: "a", DependsOnTaskID: "missing"}}, ErrGraphTaskNotFound},
		{"cross-user existing edge", []Dependency{{TaskID: "a", DependsOnTaskID: "foreign"}}, ErrGraphOwnership},
		{"existing remote self cycle", []Dependency{{TaskID: "c", DependsOnTaskID: "c"}}, ErrDependencyCycle},
		{"existing remote cycle", []Dependency{{TaskID: "b", DependsOnTaskID: "c"}, {TaskID: "c", DependsOnTaskID: "b"}}, ErrDependencyCycle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDependencyAddition(tasks, tc.edges, "a", "b"); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
