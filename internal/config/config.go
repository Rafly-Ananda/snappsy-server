package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type GeneralConfig struct {
	GinPort string
}

type MongoConfig struct {
	Hosts           string
	DbName          string
	DbUsername      string
	DbPassword      string
	DbOpts          string
	ImageCollection string
	UserCollection  string
	EventCollection string
}

type MinioConfig struct {
	MinIOEndpoint        string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOBucket          string
	MinioPresignedExpiry time.Duration
}

type Config struct {
	MinioCfg   MinioConfig
	MongoCfg   MongoConfig
	GeneralCfg GeneralConfig
}

func Load() *Config {
	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

	cfg := &Config{
		MinioCfg:   GetMinio(),
		MongoCfg:   GetMongo(),
		GeneralCfg: GetGeneral(),
	}
	return cfg
}

func GetMongo() MongoConfig {
	return MongoConfig{
		Hosts:           getEnv("MONGODB_HOST", "localhost:2173"),
		DbName:          getEnv("MONGO_INITDB_DATABASE", "snappsy"),
		DbUsername:      getEnv("MONGODB_USERNAME", "mongoadmin"),
		DbPassword:      getEnv("MONGODB_PASSWORD", "mongoadmin"),
		DbOpts:          getEnv("MONGO_OPTIONS", ""),
		ImageCollection: getEnv("MONGO_IMAGES_COLLECTION", "images"),
		EventCollection: getEnv("MONGO_EVENTS_COLLECTION", "events"),
		UserCollection:  getEnv("MONGO_USER_COLLECTION", "users"),
	}
}

func GetMinio() MinioConfig {
	return MinioConfig{
		MinIOEndpoint:        getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:       getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:       getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:          getEnv("MINIO_BUCKET", "images"),
		MinioPresignedExpiry: time.Duration(getEnvInt("MINIO_EXPIRY_IN_MINUTES", 30)) * time.Minute,
	}
}

func GetGeneral() GeneralConfig {
	return GeneralConfig{
		GinPort: getEnv("APP_PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	log.Printf("using fallback for %s", key)
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
		log.Printf("invalid int for %s: %s, using fallback", key, val)
	}
	return fallback
}
