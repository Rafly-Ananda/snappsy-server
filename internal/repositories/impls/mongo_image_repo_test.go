package impls

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCursorRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 11, 2, 12, 30, 0, 123456789, time.UTC)
	id := primitive.NewObjectID()

	parsedTime, parsedID, ok := parseCursor(makeCursor(createdAt, id))
	if !ok {
		t.Fatal("parseCursor() = false")
	}
	if !parsedTime.Equal(createdAt) || parsedID != id {
		t.Fatalf("parseCursor() = (%v, %v), want (%v, %v)", parsedTime, parsedID, createdAt, id)
	}
}

func TestParseCursorRejectsInvalidValues(t *testing.T) {
	for _, cursor := range []string{"", "not-base64", "aW52YWxpZA==", "2026-11-02T12:30:00Z|not-an-object-id"} {
		if _, _, ok := parseCursor(cursor); ok {
			t.Fatalf("parseCursor(%q) = true", cursor)
		}
	}
}
