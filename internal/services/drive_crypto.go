package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

var ErrDriveNotConfigured = errors.New("google drive is not configured")
var ErrDriveFolderInaccessible = errors.New("select the folder with Google Picker so Snappsy can access it")
var ErrDriveReconnect = errors.New("reconnect Google Drive to grant folder access")

func guestIdentityHash(guestID, username string) string {
	identity := strings.TrimSpace(guestID)
	if identity == "" {
		identity = "legacy:" + strings.ToLower(strings.TrimSpace(username))
	}
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		name = "guest"
	}
	return hashToken("guest:" + identity + ":" + name)
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type tokenCipher struct {
	aead cipher.AEAD
}

func newTokenCipher(base64Key string) (*tokenCipher, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil || len(key) != 32 {
		return nil, ErrDriveNotConfigured
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &tokenCipher{aead: aead}, nil
}

func (c *tokenCipher) Encrypt(value string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *tokenCipher) Decrypt(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	if len(raw) < c.aead.NonceSize() {
		return "", errors.New("invalid encrypted token")
	}
	nonce, payload := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
