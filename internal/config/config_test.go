package config

import (
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:3000")
	t.Setenv("ADMIN_USERNAME", "snappsy-admin")
	t.Setenv("ADMIN_PASSWORD", "snappsy-dev-password")
	t.Setenv("ADMIN_PASSWORD_HASH", "")
	t.Setenv("MONGODB_HOST", "mongo:27017")
	t.Setenv("MONGODB_USERNAME", "")
	t.Setenv("MONGODB_PASSWORD", "")
	t.Setenv("MONGO_INITDB_DATABASE", "snappsy")
	t.Setenv("SEAWEEDFS_ENDPOINT", "seaweedfs:9000")
	t.Setenv("SEAWEEDFS_PUBLIC_URL", "")
	t.Setenv("SEAWEEDFS_ACCESS_KEY", "")
	t.Setenv("SEAWEEDFS_SECRET_KEY", "")
	t.Setenv("SEAWEEDFS_BUCKET", "images")
}

func TestLoadDevelopmentConfig(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.test, http://localhost:3000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GeneralCfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q", cfg.GeneralCfg.AppEnv)
	}
	if got, want := strings.Join(cfg.GeneralCfg.AllowedOrigins, ","), "https://app.example.test,http://localhost:3000"; got != want {
		t.Fatalf("AllowedOrigins = %q, want %q", got, want)
	}
	if cfg.StorageCfg.Endpoint != "seaweedfs:9000" {
		t.Fatalf("StorageCfg.Endpoint = %q", cfg.StorageCfg.Endpoint)
	}
}

func TestLoadProductionRequiresExplicitConfig(t *testing.T) {
	requiredKeys := []string{
		"APP_PORT",
		"ALLOWED_ORIGINS",
		"ADMIN_USERNAME",
		"ADMIN_PASSWORD_HASH",
		"MONGODB_HOST",
		"MONGODB_USERNAME",
		"MONGODB_PASSWORD",
		"MONGO_INITDB_DATABASE",
		"SEAWEEDFS_ENDPOINT",
		"SEAWEEDFS_ACCESS_KEY",
		"SEAWEEDFS_SECRET_KEY",
		"SEAWEEDFS_BUCKET",
	}

	for _, missingKey := range requiredKeys {
		t.Run(missingKey, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("APP_ENV", "production")
			for _, key := range requiredKeys {
				value := "value"
				switch key {
				case "APP_PORT":
					value = "8080"
				case "ALLOWED_ORIGINS":
					value = "https://app.example.test"
				case "ADMIN_PASSWORD_HASH":
					value = "$2a$12$C0rdsA5RRRg1lIFSOG4rBOySLCXNWDi8N9ozY1xsHaesm5OzfSPxC"
				case "MONGO_INITDB_DATABASE":
					value = "snappsy"
				case "SEAWEEDFS_BUCKET":
					value = "images"
				}
				t.Setenv(key, value)
			}
			t.Setenv(missingKey, "")

			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "missing required production environment variable "+missingKey) {
				t.Fatalf("Load() error = %v, want missing %s", err, missingKey)
			}
		})
	}
}

func TestLoadRejectsInvalidAdminPasswordHash(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ADMIN_PASSWORD_HASH", "not-a-bcrypt-hash")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "ADMIN_PASSWORD_HASH") {
		t.Fatalf("Load() error = %v, want ADMIN_PASSWORD_HASH validation error", err)
	}
}

func TestLoadRejectsInvalidPublicURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("SEAWEEDFS_PUBLIC_URL", "https://files.example.test/uploads")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "path is not allowed") {
		t.Fatalf("Load() error = %v, want path validation error", err)
	}
}
