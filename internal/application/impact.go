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

type ImpactClaims struct {
	UserID     string `json:"u"`
	Operation  string `json:"o"`
	ResourceID string `json:"r"`
	Version    int64  `json:"v"`
	ExpiresAt  int64  `json:"e"`
}

type ImpactTokens struct {
	Secret []byte
	Now    func() time.Time
	TTL    time.Duration
}

func (tokens ImpactTokens) Issue(claims ImpactClaims) (string, error) {
	if len(tokens.Secret) < 32 || claims.UserID == "" || claims.Operation == "" || claims.ResourceID == "" || claims.Version < 1 {
		return "", ErrInvalidImpactToken
	}
	now := time.Now
	if tokens.Now != nil {
		now = tokens.Now
	}
	ttl := 5 * time.Minute
	if tokens.TTL > 0 {
		ttl = tokens.TTL
	}
	claims.ExpiresAt = now().Add(ttl).Unix()
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", ErrInvalidImpactToken
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := tokens.sign(encoded)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (tokens ImpactTokens) Verify(value string, expected ImpactClaims) error {
	parts := splitToken(value)
	if len(tokens.Secret) < 32 || parts == nil {
		return ErrInvalidImpactToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, tokens.sign(parts[0])) {
		return ErrInvalidImpactToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrInvalidImpactToken
	}
	var claims ImpactClaims
	if json.Unmarshal(payload, &claims) != nil {
		return ErrInvalidImpactToken
	}
	now := time.Now
	if tokens.Now != nil {
		now = tokens.Now
	}
	if claims.ExpiresAt < now().Unix() || claims.UserID != expected.UserID ||
		claims.Operation != expected.Operation || claims.ResourceID != expected.ResourceID || claims.Version != expected.Version {
		return ErrInvalidImpactToken
	}
	return nil
}

func (tokens ImpactTokens) sign(payload string) []byte {
	mac := hmac.New(sha256.New, tokens.Secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func splitToken(value string) []string {
	for index := 0; index < len(value); index++ {
		if value[index] == '.' && index > 0 && index < len(value)-1 {
			return []string{value[:index], value[index+1:]}
		}
	}
	return nil
}
