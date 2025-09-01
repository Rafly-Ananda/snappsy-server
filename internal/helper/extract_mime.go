package helper

import (
	"errors"
	"fmt"
	"mime"
	"strings"
)

type Info struct {
	MIME   string // e.g. "image/png"
	Ext    string // e.g. ".png"
	Base64 bool   // whether ";base64" flag is present
}

func FromDataURL(s string) (Info, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "data:") {
		return Info{}, errors.New("not a data URL")
	}
	comma := strings.IndexByte(s, ',')
	if comma == -1 {
		return Info{}, errors.New("invalid data URL: missing comma")
	}

	header := s[len("data:"):comma] // e.g. "image/png;base64" or "image/svg+xml;charset=utf-8;base64"
	parts := strings.Split(header, ";")
	if len(parts) == 0 || parts[0] == "" {
		return Info{}, errors.New("invalid data URL: missing media type")
	}

	mimeType := strings.ToLower(parts[0])
	base64Flag := false
	for _, p := range parts[1:] {
		if strings.EqualFold(p, "base64") {
			base64Flag = true
			break
		}
	}

	ext := extensionFromMIME(mimeType)
	fmt.Printf("ext is %s", ext)
	fmt.Println()
	if ext == "" {
		return Info{}, errors.New("unknown/unsupported media type: " + mimeType)
	}

	return Info{MIME: mimeType, Ext: ext, Base64: base64Flag}, nil
}

func extensionFromMIME(mt string) string {
	
	fmt.Printf("logging mime type")
	fmt.Println(mt)

	x, _ := mime.ExtensionsByType(mt)
	fmt.Println(x)

	// TODO: stdLib will return .jfif if image is jpeg, need to override later

	// // stdlib first
	// if exts, _ := mime.ExtensionsByType(mt); len(exts) > 0 {
	// 	return exts[0] // includes leading dot
	// }
	// common fallbacks
	switch mt {
	case "image/jpg", "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "image/heic":
		return ".heic"
	default:
		return ""
	}
}
