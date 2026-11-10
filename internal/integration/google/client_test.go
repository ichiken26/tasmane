package google

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"tasmane/internal/application"
)

// Bearer認証とGoogle HTTPステータス伝播
func TestRequestJSONUsesBearerAndReportsGoogleStatus(t *testing.T) {
	// 成功と429を返すテストサーバ起動
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
	// 成功パスのJSONデコード
	if err := client.requestJSON(context.Background(), "access", http.MethodGet, "/ok", nil, &list); err != nil || list.ID != "list" {
		t.Fatalf("list=%#v err=%v", list, err)
	}
	// 失敗パスのAPIエラー変換
	err := client.requestJSON(context.Background(), "access", http.MethodGet, "/fail", nil, nil)
	var apiError application.GoogleAPIError
	if !errors.As(err, &apiError) || apiError.Status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 API error, got %v", err)
	}
}

// 比較・取込用スナップショットではメモ欄を除いた本文だけを使う
func TestSnapshotOfStripsMemoSection(t *testing.T) {
	got := snapshotOf(googleTask{Title: "t", Notes: "本文\n\n── memo(tasmane) ──\n・議事録\nhttps://memo.kokage-studio.com/memo/a", Status: "needsAction"})
	if got["notes"] != "本文" {
		t.Fatalf("notes=%q", got["notes"])
	}
}
