package httpapi

import (
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// OpenAPI に定義された全 operation がルータに実装され、ルータの全ルートが OpenAPI に記載されていること
func TestRouterMatchesOpenAPIOperations(t *testing.T) {
	body, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pathLine := regexp.MustCompile(`^  (/v1/[^:]+):\s*$`)
	methodLine := regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
	documented := map[string]bool{}
	current := ""
	for _, line := range strings.Split(string(body), "\n") {
		if match := pathLine.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}
		if strings.HasPrefix(line, "components:") {
			current = ""
		}
		if match := methodLine.FindStringSubmatch(line); match != nil && current != "" {
			documented[strings.ToUpper(match[1])+" "+current] = true
		}
	}
	implemented := map[string]bool{}
	router := NewRouter(Handler{}).(chi.Routes)
	if err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		implemented[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	missing, undocumented := []string{}, []string{}
	for operation := range documented {
		if !implemented[operation] {
			missing = append(missing, operation)
		}
	}
	for operation := range implemented {
		if !documented[operation] {
			undocumented = append(undocumented, operation)
		}
	}
	sort.Strings(missing)
	sort.Strings(undocumented)
	if len(missing) > 0 || len(undocumented) > 0 {
		t.Fatalf("OpenAPI/router mismatch\nmissing in router: %v\nmissing in OpenAPI: %v", missing, undocumented)
	}
}
