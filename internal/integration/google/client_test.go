package google

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"tasmane/internal/application"
)

func TestRequestJSONUsesBearerAndReportsGoogleStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/ok" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"list"}`))
			return
		}
		http.Error(w, "quota", http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := &Client{HTTP: server.Client(), APIBase: server.URL}
	var list taskList
	if err := client.requestJSON(context.Background(), "access", http.MethodGet, "/ok", nil, &list); err != nil || list.ID != "list" {
		t.Fatalf("list=%#v err=%v", list, err)
	}
	err := client.requestJSON(context.Background(), "access", http.MethodGet, "/fail", nil, nil)
	var apiError application.GoogleAPIError
	if !errors.As(err, &apiError) || apiError.Status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 API error, got %v", err)
	}
}
