package models

import "testing"

func TestEventAllowsCapture(t *testing.T) {
	event := Event{CaptureModes: EventCaptureModes{Photo: true, Strip: false, Boomerang: true}}
	cases := []struct {
		kind MediaKind
		want bool
	}{
		{MediaKindPhoto, true},
		{MediaKindPhotoStrip, false},
		{MediaKindBoomerang, true},
		{MediaKind("unknown"), false},
	}
	for _, tc := range cases {
		if got := event.AllowsCapture(tc.kind); got != tc.want {
			t.Fatalf("AllowsCapture(%q) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}
