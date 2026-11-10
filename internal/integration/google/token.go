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

// リフレッシュトークンのAES-GCM暗号器
type TokenCipher struct{ key []byte }

// 32バイト鍵からの暗号器生成
func NewTokenCipher(key []byte) (TokenCipher, error) {
	if len(key) != 32 {
		return TokenCipher{}, errors.New("token encryption key must be 32 bytes")
	}
	copyKey := append([]byte(nil), key...)
	return TokenCipher{key: copyKey}, nil
}

// 平文トークンの暗号化
func (c TokenCipher) Encrypt(value string) (string, error) {
	// AES-GCMの初期化
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	// 乱数nonceの生成
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// 暗号文のBase64符号化
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// 暗号文トークンの復号
func (c TokenCipher) Decrypt(value string) (string, error) {
	// Base64暗号文の復元
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	// AES-GCMの初期化
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
	// nonce分離後の平文復元
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plain), nil
}
