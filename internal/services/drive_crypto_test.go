package services

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTokenCipherRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	cipher, err := newTokenCipher(key)
	if err != nil {
		t.Fatalf("newTokenCipher() error = %v", err)
	}

	encrypted, err := cipher.Encrypt("refresh-token-value")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if encrypted == "refresh-token-value" {
		t.Fatal("Encrypt() returned plaintext")
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != "refresh-token-value" {
		t.Fatalf("Decrypt() = %q", decrypted)
	}
}

func TestGuestIdentityHashIsStablePerGuest(t *testing.T) {
	first := guestIdentityHash("guest-a", "Rafly")
	if first != guestIdentityHash("guest-a", "Rafly") {
		t.Fatal("guest hash is not stable")
	}
	if first == guestIdentityHash("guest-b", "Rafly") {
		t.Fatal("different guests with the same name must not merge")
	}
	if first == guestIdentityHash("guest-a", "John Dane") {
		t.Fatal("the same device using a different name must get a separate guest identity")
	}
}

func TestNormalizeFolderID(t *testing.T) {
	cases := map[string]string{
		"folder-id": "folder-id",
		"https://drive.google.com/drive/folders/folder-id?usp=sharing": "folder-id",
		"https://drive.google.com/open?id=folder-id":                   "folder-id",
		"  folder-id/  ": "folder-id",
	}
	for input, want := range cases {
		if got := normalizeFolderID(input); got != want {
			t.Fatalf("normalizeFolderID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExportMediaNameIncludesGuestAndCaption(t *testing.T) {
	item := models.MediaItem{Username: "Ada / Lovelace", Captions: "best night: ever?", Kind: models.MediaKindPhoto}
	item.ID = primitive.NewObjectID()
	name := exportMediaName(item)
	if !strings.HasPrefix(name, "Ada - Lovelace - best night- ever- [") || !strings.HasSuffix(name, ".jpg") {
		t.Fatalf("exportMediaName() = %q", name)
	}
}

func TestExportHelpers(t *testing.T) {
	if got := safeGuestName("  Ada / Lovelace  "); got != "Ada  Lovelace" {
		t.Fatalf("safeGuestName() = %q", got)
	}
	if got := safeGuestName(""); got != "Guest" {
		t.Fatalf("safeGuestName(empty) = %q", got)
	}
}
