package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

// ヘルスチェック用ハンドラ
type Health struct {
	DB *sql.DB
}

// 生存確認と準備完了確認のルート登録
func (h Health) Register(mux *http.ServeMux) {
	// プロセス生存確認
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		// 生存確認レスポンスの書き出し
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// DB到達性の確認
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if h.DB == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.DB.PingContext(ctx); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		// 準備完了レスポンスの書き出し
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
}
