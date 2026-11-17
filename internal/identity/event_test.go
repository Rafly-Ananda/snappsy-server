package identity

import "testing"

func TestNormalizeSlug(t *testing.T) {
	tests := map[string]string{
		"Rafly & Intan Wedding": "rafly-intan-wedding",
		"  ---  ":               "event",
		"Already-OK_slug 123":   "already-ok_slug-123",
	}
	for input, want := range tests {
		if got := NormalizeSlug(input); got != want {
			t.Fatalf("NormalizeSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestUniqueSlugAddsSuffix(t *testing.T) {
	exists := map[string]bool{"event-abc123": true}
	if got := UniqueSlug("Event", "abc123", exists); got != "event-abc123-2" {
		t.Fatalf("UniqueSlug() = %q", got)
	}
}

func TestDisplayTokenHashVerifiesRawToken(t *testing.T) {
	raw, hash, err := NewDisplayToken()
	if err != nil {
		t.Fatalf("NewDisplayToken() error = %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("NewDisplayToken() returned empty values")
	}
	if HashToken(raw) != hash {
		t.Fatal("HashToken(raw) does not match issued hash")
	}
	if HashToken(raw+"x") == hash {
		t.Fatal("HashToken() matched a different token")
	}
}
