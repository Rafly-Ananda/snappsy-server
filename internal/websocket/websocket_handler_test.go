package websocket

import "testing"

func TestIsAllowedOrigin(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://show.example.test"}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "allowed localhost", origin: "http://localhost:3000", want: true},
		{name: "allowed production", origin: "https://show.example.test", want: true},
		{name: "wrong scheme", origin: "https://localhost:3000", want: false},
		{name: "wrong host", origin: "https://evil.example.test", want: false},
		{name: "empty origin", origin: "", want: false},
		{name: "null origin", origin: "null", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAllowedOrigin(tt.origin, allowed); got != tt.want {
				t.Fatalf("isAllowedOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}
