package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI    string
	MongoDBName string

	JWTSecret        string
	JWTRefreshSecret string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		MongoURI:         strings.TrimSpace(os.Getenv("MONGO_URI")),
		MongoDBName:      strings.TrimSpace(os.Getenv("DB_NAME")),
		JWTSecret:        strings.TrimSpace(os.Getenv("JWT_SECRET")),
		JWTRefreshSecret: strings.TrimSpace(os.Getenv("JWT_REFRESH_SECRET")),
	}

	if cfg.MongoURI == "" {
		return Config{}, fmt.Errorf("Missing mongo URI")
	}

	if cfg.MongoDBName == "" {
		return Config{}, fmt.Errorf("Missing mongo db name")
	}

	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("Missing jwt secret")
	}

	if cfg.JWTRefreshSecret == "" {
		return Config{}, fmt.Errorf("Missing jwt refresh secret")
	}

	return cfg, nil
}
