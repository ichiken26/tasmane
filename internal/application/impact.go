package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidImpactToken = errors.New("invalid impact token")

// 影響トークンの束縛クレーム
type ImpactClaims struct {
	UserID     string `json:"u"`
	Operation  string `json:"o"`
	ResourceID string `json:"r"`
	Version    int64  `json:"v"`
	ExpiresAt  int64  `json:"e"`
}

// HMAC署名付き影響トークンの発行・検証
type ImpactTokens struct {
	Secret []byte
	Now    func() time.Time
	TTL    time.Duration
}

// 束縛クレーム付き影響トークンの発行
func (tokens ImpactTokens) Issue(claims ImpactClaims) (string, error) {
	// 秘密鍵とクレームの妥当性検証
	if len(tokens.Secret) < 32 || claims.UserID == "" || claims.Operation == "" || claims.ResourceID == "" || claims.Version < 1 {
		return "", ErrInvalidImpactToken
	}
	// 現在時刻関数の解決
	now := time.Now
	if tokens.Now != nil {
		now = tokens.Now
	}
	// TTLの決定
	ttl := 5 * time.Minute
	if tokens.TTL > 0 {
		ttl = tokens.TTL
	}
	// 有効期限の付与
	claims.ExpiresAt = now().Add(ttl).Unix()
	// クレームのJSON化
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", ErrInvalidImpactToken
	}
	// ペイロードの署名付きトークン組み立て
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := tokens.sign(encoded)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// 署名と束縛項目の影響トークン検証
func (tokens ImpactTokens) Verify(value string, expected ImpactClaims) error {
	// 秘密鍵とトークン形式の検証
	parts := splitToken(value)
	if len(tokens.Secret) < 32 || parts == nil {
		return ErrInvalidImpactToken
	}
	// HMAC署名の照合
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, tokens.sign(parts[0])) {
		return ErrInvalidImpactToken
	}
	// ペイロードのデコード
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrInvalidImpactToken
	}
	// クレームのJSON復元
	var claims ImpactClaims
	if json.Unmarshal(payload, &claims) != nil {
		return ErrInvalidImpactToken
	}
	// 現在時刻関数の解決
	now := time.Now
	if tokens.Now != nil {
		now = tokens.Now
	}
	// 期限と束縛項目の照合
	if claims.ExpiresAt < now().Unix() || claims.UserID != expected.UserID ||
		claims.Operation != expected.Operation || claims.ResourceID != expected.ResourceID || claims.Version != expected.Version {
		return ErrInvalidImpactToken
	}
	return nil
}

// HMAC-SHA256署名の計算
func (tokens ImpactTokens) sign(payload string) []byte {
	mac := hmac.New(sha256.New, tokens.Secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// ドット区切りのトークン分割
func splitToken(value string) []string {
	for index := 0; index < len(value); index++ {
		if value[index] == '.' && index > 0 && index < len(value)-1 {
			return []string{value[:index], value[index+1:]}
		}
	}
	return nil
}
