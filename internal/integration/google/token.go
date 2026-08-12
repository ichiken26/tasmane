package google

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var ErrInvalidCiphertext = errors.New("invalid encrypted token")

type TokenCipher struct{ key []byte }

func NewTokenCipher(key []byte) (TokenCipher, error) {
	if len(key) != 32 {
		return TokenCipher{}, errors.New("token encryption key must be 32 bytes")
	}
	copyKey := append([]byte(nil), key...)
	return TokenCipher{key: copyKey}, nil
}
func (c TokenCipher) Encrypt(value string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}
func (c TokenCipher) Decrypt(value string) (string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", ErrInvalidCiphertext
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plain), nil
}
