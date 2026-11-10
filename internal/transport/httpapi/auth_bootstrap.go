package httpapi

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"tasmane/internal/identifier"
	googleintegration "tasmane/internal/integration/google"
)

// 認証ブートストラップのリクエスト本体
type authBootstrapRequest struct {
	GoogleSubject string   `json:"googleSubject"`
	Email         string   `json:"email"`
	DisplayName   string   `json:"displayName"`
	AvatarURL     string   `json:"avatarUrl"`
	RefreshToken  string   `json:"refreshToken"`
	Scopes        []string `json:"scopes"`
}

// 内部ユーザーID払い出しの応答
type authBootstrapResponse struct {
	UserID string `json:"userId"`
}

// ユーザープロビジョニングの永続化境界
type authBootstrapStore interface {
	Bootstrap(context.Context, authBootstrapRequest) (string, error)
}

// Postgres上のユーザーとGoogle連携の初期化
type postgresAuthBootstrap struct {
	database *sql.DB
	cipher   googleintegration.TokenCipher
	ids      *identifier.UUIDv7Generator
}

// Worker検証後の内部ユーザーID払い出し
func NewAuthBootstrapHandler(database *sql.DB, cipher googleintegration.TokenCipher, bindingSecret string) http.Handler {
	return newAuthBootstrapHandler(&postgresAuthBootstrap{
		database: database,
		cipher:   cipher,
		ids:      identifier.NewUUIDv7Generator(),
	}, bindingSecret)
}

// バインディング照合付きブートストラップハンドラ
func newAuthBootstrapHandler(store authBootstrapStore, bindingSecret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// バインディング秘密の照合
		provided := r.Header.Get("X-Tasmane-Internal-Binding")
		validBinding := bindingSecret != "" && len(provided) == len(bindingSecret) &&
			subtle.ConstantTimeCompare([]byte(provided), []byte(bindingSecret)) == 1
		if !validBinding {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// リクエストJSONのデコード
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input authBootstrapRequest
		if err := decoder.Decode(&input); err != nil || !validBootstrapIdentity(input) {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid identity")
			return
		}
		// ユーザープロビジョニングの呼び出し
		userID, err := store.Bootstrap(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user provisioning failed")
			return
		}
		// ユーザーIDレスポンスの書き出し
		writeJSON(w, http.StatusOK, authBootstrapResponse{UserID: userID})
	})
}

// Google subjectとメール形式の検証
func validBootstrapIdentity(input authBootstrapRequest) bool {
	input.GoogleSubject = strings.TrimSpace(input.GoogleSubject)
	input.Email = strings.TrimSpace(input.Email)
	if input.GoogleSubject == "" || len(input.GoogleSubject) > 255 || len(input.Email) > 320 || len(input.DisplayName) > 500 || len(input.AvatarURL) > 2048 || len(input.RefreshToken) > 16384 || len(input.Scopes) > 64 {
		return false
	}
	address, err := mail.ParseAddress(input.Email)
	return err == nil && strings.EqualFold(address.Address, input.Email)
}

// ユーザー・設定・Google連携の原子的初期化
func (p *postgresAuthBootstrap) Bootstrap(ctx context.Context, input authBootstrapRequest) (string, error) {
	if p == nil || p.database == nil || p.ids == nil {
		return "", errors.New("auth bootstrap store is unavailable")
	}
	userID, err := p.ids.New()
	if err != nil {
		return "", err
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// ユーザー行のupsert
	if err := tx.QueryRowContext(ctx, `INSERT INTO users(id,google_subject,email,display_name,avatar_url)
		VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''))
		ON CONFLICT(google_subject) DO UPDATE SET email=EXCLUDED.email,
			display_name=EXCLUDED.display_name,avatar_url=EXCLUDED.avatar_url,updated_at=now()
		RETURNING id`, userID, strings.TrimSpace(input.GoogleSubject), strings.TrimSpace(input.Email),
		strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.AvatarURL)).Scan(&userID); err != nil {
		return "", err
	}
	// サーバー設定行の確保
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_server_settings(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		return "", err
	}
	if input.RefreshToken != "" {
		// リフレッシュトークンの暗号化と連携保存
		encrypted, err := p.cipher.Encrypt(input.RefreshToken)
		if err != nil {
			return "", err
		}
		integrationID, err := p.ids.New()
		if err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO google_integrations(id,user_id,google_subject,encrypted_refresh_token,scopes)
			VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(user_id) DO UPDATE SET google_subject=EXCLUDED.google_subject,
				encrypted_refresh_token=EXCLUDED.encrypted_refresh_token,scopes=EXCLUDED.scopes,updated_at=now()`,
			integrationID, userID, strings.TrimSpace(input.GoogleSubject), encrypted, input.Scopes); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return userID, nil
}
