package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// APIキー送信と応答テキスト抽出
func TestGenerateUsesAPIKeyAndExtractsText(t *testing.T) {
	// 固定応答のテストサーバ起動
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "secret" {
			t.Fatal("missing key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"focus"}]}}]}`))
	}))
	defer server.Close()
	// 生成APIの呼び出し
	got, err := (Client{APIKey: "secret", Model: "model", BaseURL: server.URL, HTTP: server.Client()}).Generate(context.Background(), "tasks")
	if err != nil || got != "focus" {
		t.Fatal(got, err)
	}
}
