package google

import (
	"errors"
	"testing"
)

// 暗号化往復と改ざん検知
func TestTokenCipherRoundTripAndTamper(t *testing.T) {
	cipher, err := NewTokenCipher([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	// 平文の暗号化
	encrypted, err := cipher.Encrypt("refresh-secret")
	if err != nil || encrypted == "refresh-secret" {
		t.Fatal(encrypted, err)
	}
	// 暗号文の復号
	plain, err := cipher.Decrypt(encrypted)
	if err != nil || plain != "refresh-secret" {
		t.Fatal(plain, err)
	}
	// 改ざん暗号文の拒否
	if _, err := cipher.Decrypt(encrypted + "x"); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatal(err)
	}
}
