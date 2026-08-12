package google

import (
	"errors"
	"testing"
)

func TestTokenCipherRoundTripAndTamper(t *testing.T) {
	cipher, err := NewTokenCipher([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("refresh-secret")
	if err != nil || encrypted == "refresh-secret" {
		t.Fatal(encrypted, err)
	}
	plain, err := cipher.Decrypt(encrypted)
	if err != nil || plain != "refresh-secret" {
		t.Fatal(plain, err)
	}
	if _, err := cipher.Decrypt(encrypted + "x"); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatal(err)
	}
}
