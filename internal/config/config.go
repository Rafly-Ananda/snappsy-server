package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type GeneralConfig struct {
	AppEnv         string
	GinPort        string
	AllowedOrigins []string
}

type MongoConfig struct {
	Hosts                      string
	DbName                     string
	DbUsername                 string
	DbPassword                 string
	DbOpts                     string
	ImageCollection            string
	LegacyImageCollection      string
	UserCollection             string
	EventCollection            string
	AssetCollection            string
	TemplateCollection         string
	TemplateVersionCollection  string
	UploadIntentCollection     string
	MediaJobCollection         string
	DriveCollection            string
	CustomerCollection         string
	CustomerSessionCollection  string
	EventPlanCollection        string
	PlatformSettingsCollection string
}

type ObjectStorageConfig struct {
	Endpoint        string
	PublicEndpoint  string
	AccessKey       string
	SecretKey       string
	Bucket          string
	UseSSL          bool
	PresignedExpiry time.Duration
}

type AdminConfig struct {
	Username      string
	PasswordHash  string
	SessionTTL    time.Duration
	SecureCookies bool
}

type DemoConfig struct {
	Enabled   bool
	Photo     bool
	Strip     bool
	Boomerang bool
	Countdown int
}

type CustomerConfig struct {
	SessionTTL    time.Duration
	SecureCookies bool
}

type BoomerangConfig struct {
	FFmpegPath       string
	FFprobePath      string
	Concurrency      int
	MaxSourceBytes   int64
	MaxSourceSeconds int
	SegmentSeconds   int
	QueueLimit       int
}

type DriveConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	PickerAPIKey string
	PickerAppID  string
	FrontendURL  string
	TokenKey     string
	SetupTTL     time.Duration
	Concurrency  int
	QueueLimit   int
}

func (d DriveConfig) Configured() bool {
	return d.ClientID != "" && d.ClientSecret != "" && d.RedirectURL != "" && d.TokenKey != ""
}

func (d DriveConfig) PickerConfigured() bool {
	return d.PickerAPIKey != "" && d.PickerAppID != ""
}

type Config struct {
	StorageCfg   ObjectStorageConfig
	MongoCfg     MongoConfig
	GeneralCfg   GeneralConfig
	AdminCfg     AdminConfig
	CustomerCfg  CustomerConfig
	BoomerangCfg BoomerangConfig
	DriveCfg     DriveConfig
	DemoCfg      DemoConfig
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	cfg := &Config{
		StorageCfg:   getObjectStorage(),
		MongoCfg:     getMongo(),
		GeneralCfg:   getGeneral(),
		AdminCfg:     getAdmin(getEnv("APP_ENV", "development")),
		CustomerCfg:  getCustomer(getEnv("APP_ENV", "development")),
		BoomerangCfg: getBoomerang(),
		DriveCfg:     getDrive(),
		DemoCfg:      getDemo(),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if err := validatePort(c.GeneralCfg.GinPort); err != nil {
		return err
	}
	if c.AdminCfg.PasswordHash != "" {
		if _, err := bcrypt.Cost([]byte(c.AdminCfg.PasswordHash)); err != nil {
			return fmt.Errorf("ADMIN_PASSWORD_HASH must be a valid bcrypt hash")
		}
	}
	if c.StorageCfg.PresignedExpiry <= 0 {
		return fmt.Errorf("SEAWEEDFS_PRESIGNED_EXPIRY_MINUTES must be greater than zero")
	}
	if c.BoomerangCfg.MaxSourceSeconds < c.BoomerangCfg.SegmentSeconds {
		return fmt.Errorf("BOOMERANG_MAX_SOURCE_SECONDS must be at least BOOMERANG_SEGMENT_SECONDS")
	}
	if c.BoomerangCfg.SegmentSeconds < 1 || c.BoomerangCfg.SegmentSeconds > 10 {
		return fmt.Errorf("BOOMERANG_SEGMENT_SECONDS must be between 1 and 10")
	}
	if c.BoomerangCfg.Concurrency < 1 {
		return fmt.Errorf("BOOMERANG_WORKER_CONCURRENCY must be at least 1")
	}
	if c.DriveCfg.Concurrency < 1 {
		return fmt.Errorf("DRIVE_EXPORT_CONCURRENCY must be at least 1")
	}
	if c.DriveCfg.Configured() {
		key, err := base64.StdEncoding.DecodeString(c.DriveCfg.TokenKey)
		if err != nil || len(key) != 32 {
			return fmt.Errorf("DRIVE_TOKEN_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
		}
	}

	origins, err := normalizeOrigins(c.GeneralCfg.AllowedOrigins)
	if err != nil {
		return err
	}
	c.GeneralCfg.AllowedOrigins = origins

	publicEndpoint, err := normalizeOptionalURL(c.StorageCfg.PublicEndpoint, c.StorageCfg.UseSSL)
	if err != nil {
		return fmt.Errorf("invalid SEAWEEDFS_PUBLIC_URL: %w", err)
	}
	c.StorageCfg.PublicEndpoint = publicEndpoint

	if c.GeneralCfg.AppEnv != "production" {
		if c.AdminCfg.Username == "" {
			return fmt.Errorf("ADMIN_USERNAME is required")
		}
		if c.AdminCfg.PasswordHash == "" && strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")) == "" {
			return fmt.Errorf("ADMIN_PASSWORD or ADMIN_PASSWORD_HASH is required")
		}
		if c.MongoCfg.Hosts == "" {
			return fmt.Errorf("MONGODB_HOST is required")
		}
		if c.MongoCfg.DbName == "" {
			return fmt.Errorf("MONGO_INITDB_DATABASE is required")
		}
		if c.StorageCfg.Endpoint == "" {
			return fmt.Errorf("SEAWEEDFS_ENDPOINT is required")
		}
		if c.StorageCfg.Bucket == "" {
			return fmt.Errorf("SEAWEEDFS_BUCKET is required")
		}
		return nil
	}

	required := []string{
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
		"PUBLIC_APP_URL",
		"GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET",
		"GOOGLE_OAUTH_REDIRECT_URL",
		"GOOGLE_PICKER_API_KEY",
		"GOOGLE_PICKER_APP_ID",
		"DRIVE_TOKEN_ENCRYPTION_KEY",
	}
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return fmt.Errorf("missing required production environment variable %s", key)
		}
	}

	if c.MongoCfg.Hosts == "" {
		return fmt.Errorf("MONGODB_HOST is required")
	}
	if c.MongoCfg.DbName == "" {
		return fmt.Errorf("MONGO_INITDB_DATABASE is required")
	}
	if c.StorageCfg.Endpoint == "" {
		return fmt.Errorf("SEAWEEDFS_ENDPOINT is required")
	}
	if c.StorageCfg.Bucket == "" {
		return fmt.Errorf("SEAWEEDFS_BUCKET is required")
	}
	if c.AdminCfg.PasswordHash == "" {
		return fmt.Errorf("ADMIN_PASSWORD_HASH is required in production")
	}
	return nil
}

func getMongo() MongoConfig {
	return MongoConfig{
		Hosts:                      getEnv("MONGODB_HOST", "localhost:27017"),
		DbName:                     getEnv("MONGO_INITDB_DATABASE", "snappsy"),
		DbUsername:                 os.Getenv("MONGODB_USERNAME"),
		DbPassword:                 os.Getenv("MONGODB_PASSWORD"),
		DbOpts:                     os.Getenv("MONGO_OPTIONS"),
		ImageCollection:            getEnv("MONGO_MEDIA_ITEMS_COLLECTION", "media_items"),
		LegacyImageCollection:      getEnv("MONGO_IMAGES_COLLECTION", "images"),
		EventCollection:            getEnv("MONGO_EVENTS_COLLECTION", "events"),
		UserCollection:             getEnv("MONGO_USER_COLLECTION", "users"),
		AssetCollection:            getEnv("MONGO_ASSETS_COLLECTION", "assets"),
		TemplateCollection:         getEnv("MONGO_TEMPLATES_COLLECTION", "templates"),
		TemplateVersionCollection:  getEnv("MONGO_TEMPLATE_VERSIONS_COLLECTION", "template_versions"),
		UploadIntentCollection:     getEnv("MONGO_UPLOAD_INTENTS_COLLECTION", "upload_intents"),
		MediaJobCollection:         getEnv("MONGO_MEDIA_JOBS_COLLECTION", "media_jobs"),
		DriveCollection:            getEnv("MONGO_DRIVE_COLLECTION", "drive"),
		CustomerCollection:         getEnv("MONGO_CUSTOMERS_COLLECTION", "customers"),
		CustomerSessionCollection:  getEnv("MONGO_CUSTOMER_SESSIONS_COLLECTION", "customer_sessions"),
		EventPlanCollection:        getEnv("MONGO_EVENT_PLANS_COLLECTION", "event_plans"),
		PlatformSettingsCollection: getEnv("MONGO_PLATFORM_SETTINGS_COLLECTION", "platform_settings"),
	}
}

func getObjectStorage() ObjectStorageConfig {
	return ObjectStorageConfig{
		Endpoint:        os.Getenv("SEAWEEDFS_ENDPOINT"),
		PublicEndpoint:  os.Getenv("SEAWEEDFS_PUBLIC_URL"),
		AccessKey:       os.Getenv("SEAWEEDFS_ACCESS_KEY"),
		SecretKey:       os.Getenv("SEAWEEDFS_SECRET_KEY"),
		Bucket:          getEnv("SEAWEEDFS_BUCKET", "images"),
		UseSSL:          getEnvBool("SEAWEEDFS_USE_SSL", false),
		PresignedExpiry: time.Duration(getEnvInt("SEAWEEDFS_PRESIGNED_EXPIRY_MINUTES", 30)) * time.Minute,
	}
}

func getDemo() DemoConfig {
	return DemoConfig{
		Enabled:   getEnvBool("DEMO_ENABLED", true),
		Photo:     getEnvBool("DEMO_PHOTO_ENABLED", true),
		Strip:     getEnvBool("DEMO_STRIP_ENABLED", true),
		Boomerang: getEnvBool("DEMO_BOOMERANG_ENABLED", true),
		Countdown: getEnvInt("DEMO_COUNTDOWN_SECONDS", 3),
	}
}

func getBoomerang() BoomerangConfig {
	return BoomerangConfig{
		FFmpegPath:       getEnv("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:      getEnv("FFPROBE_PATH", "ffprobe"),
		Concurrency:      getEnvInt("BOOMERANG_WORKER_CONCURRENCY", 1),
		MaxSourceBytes:   int64(getEnvInt("BOOMERANG_MAX_SOURCE_BYTES", 50*1024*1024)),
		MaxSourceSeconds: getEnvInt("BOOMERANG_MAX_SOURCE_SECONDS", 30),
		SegmentSeconds:   getEnvInt("BOOMERANG_SEGMENT_SECONDS", 3),
		QueueLimit:       getEnvInt("BOOMERANG_QUEUE_LIMIT", 100),
	}
}

func getDrive() DriveConfig {
	frontend := getEnv("PUBLIC_APP_URL", "http://localhost:3000")
	return DriveConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  getEnv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:8080/api/v1/drive/oauth/callback"),
		PickerAPIKey: os.Getenv("GOOGLE_PICKER_API_KEY"),
		PickerAppID:  os.Getenv("GOOGLE_PICKER_APP_ID"),
		FrontendURL:  frontend,
		TokenKey:     os.Getenv("DRIVE_TOKEN_ENCRYPTION_KEY"),
		SetupTTL:     time.Duration(getEnvInt("DRIVE_SETUP_TOKEN_TTL_MINUTES", 30)) * time.Minute,
		Concurrency:  getEnvInt("DRIVE_EXPORT_CONCURRENCY", 1),
		QueueLimit:   getEnvInt("DRIVE_QUEUE_LIMIT", 1000),
	}
}

func getAdmin(appEnv string) AdminConfig {
	passwordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if passwordHash == "" && appEnv != "production" {
		if password := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")); password != "" {
			hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err == nil {
				passwordHash = string(hashed)
			}
		}
	}
	return AdminConfig{
		Username:      getEnv("ADMIN_USERNAME", "snappsy-admin"),
		PasswordHash:  passwordHash,
		SessionTTL:    time.Duration(getEnvInt("ADMIN_SESSION_TTL_MINUTES", 720)) * time.Minute,
		SecureCookies: appEnv == "production",
	}
}

func getCustomer(appEnv string) CustomerConfig {
	return CustomerConfig{
		SessionTTL:    time.Duration(getEnvInt("CUSTOMER_SESSION_TTL_MINUTES", 720)) * time.Minute,
		SecureCookies: appEnv == "production",
	}
}

func getGeneral() GeneralConfig {
	return GeneralConfig{
		AppEnv:         getEnv("APP_ENV", "development"),
		GinPort:        getEnv("APP_PORT", "8080"),
		AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),
	}
}

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return fallback
}

func validatePort(port string) error {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("APP_PORT must be a valid TCP port")
	}
	return nil
}

func normalizeOrigins(origins []string) ([]string, error) {
	out := make([]string, 0, len(origins))
	for _, origin := range origins {
		normalized, err := normalizeRequiredURL(strings.TrimSpace(origin), true)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed origin %q: %w", origin, err)
		}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one allowed origin is required")
	}
	return out, nil
}

func normalizeOptionalURL(raw string, secure bool) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return normalizeRequiredURL(raw, secure)
}

func normalizeRequiredURL(raw string, secure bool) (string, error) {
	value := strings.TrimSpace(raw)
	if !strings.Contains(value, "://") {
		value = "//" + value
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("path is not allowed")
	}
	if u.Scheme == "" {
		if secure {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	return u.Scheme + "://" + u.Host, nil
}
