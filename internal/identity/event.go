package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

func NormalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		switch {
		case valid:
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "event"
	}
	return slug
}

func UniqueSlug(name string, idSuffix string, exists map[string]bool) string {
	base := NormalizeSlug(name)
	candidate := base
	if idSuffix != "" {
		candidate = base + "-" + idSuffix
	}
	if !exists[candidate] {
		return candidate
	}
	for i := 2; ; i++ {
		candidate = base + "-" + idSuffix + "-" + itoa(i)
		if !exists[candidate] {
			return candidate
		}
	}
}

func NewDisplayToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}
