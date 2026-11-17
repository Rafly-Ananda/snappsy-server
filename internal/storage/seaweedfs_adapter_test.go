package storage

import "testing"

func TestParsePublicEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		defaultSecure bool
		wantRaw       string
		wantHost      string
		wantSecure    bool
		wantErr       bool
	}{
		{
			name:          "empty",
			raw:           "",
			defaultSecure: false,
		},
		{
			name:       "https URL",
			raw:        "https://files.example.test",
			wantRaw:    "https://files.example.test",
			wantHost:   "files.example.test",
			wantSecure: true,
		},
		{
			name:     "http URL with port",
			raw:      "http://localhost:9000",
			wantRaw:  "http://localhost:9000",
			wantHost: "localhost:9000",
		},
		{
			name:          "schemeless defaults to secure",
			raw:           "files.example.test:8443",
			defaultSecure: true,
			wantRaw:       "https://files.example.test:8443",
			wantHost:      "files.example.test:8443",
			wantSecure:    true,
		},
		{
			name:    "rejects path",
			raw:     "https://files.example.test/uploads",
			wantErr: true,
		},
		{
			name:    "rejects unsupported scheme",
			raw:     "ftp://files.example.test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePublicEndpoint(tt.raw, tt.defaultSecure)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parsePublicEndpoint() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePublicEndpoint() error = %v", err)
			}
			if got.raw != tt.wantRaw || got.host != tt.wantHost || got.secure != tt.wantSecure {
				t.Fatalf("parsePublicEndpoint() = %+v, want raw=%q host=%q secure=%v", got, tt.wantRaw, tt.wantHost, tt.wantSecure)
			}
		})
	}
}

func TestNewSeaweedFSUsesPublicEndpointForPresigning(t *testing.T) {
	adapter, err := NewSeaweedFS("seaweedfs:9000", "https://files.example.test", "access", "secret", "images", false)
	if err != nil {
		t.Fatalf("NewSeaweedFS() error = %v", err)
	}
	if adapter.endpoint != "http://seaweedfs:9000" {
		t.Fatalf("endpoint = %q", adapter.endpoint)
	}
	if adapter.presignEndpoint != "https://files.example.test" {
		t.Fatalf("presignEndpoint = %q", adapter.presignEndpoint)
	}
}
