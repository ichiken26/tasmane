package application

import (
	"errors"
	"testing"
	"time"
)

// ユーザー・操作・資源・バージョン・期限の束縛確認
func TestImpactTokenBindsUserOperationResourceVersionAndExpiry(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tokens := ImpactTokens{Secret: []byte("01234567890123456789012345678901"), Now: func() time.Time { return now }, TTL: time.Minute}
	claims := ImpactClaims{UserID: "u", Operation: "ADD_FIRST_CHILD", ResourceID: "parent", Version: 4}
	// 発行と同一クレーム検証の成功確認
	token, err := tokens.Issue(claims)
	if err != nil || tokens.Verify(token, claims) != nil {
		t.Fatalf("token=%q issue=%v", token, err)
	}
	// バージョン不一致の拒否確認
	changed := claims
	changed.Version++
	if !errors.Is(tokens.Verify(token, changed), ErrInvalidImpactToken) {
		t.Fatal("version mismatch accepted")
	}
	// 期限切れトークンの拒否確認
	tokens.Now = func() time.Time { return now.Add(2 * time.Minute) }
	if !errors.Is(tokens.Verify(token, claims), ErrInvalidImpactToken) {
		t.Fatal("expired token accepted")
	}
}
