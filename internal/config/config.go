package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI    string
	MongoDBName string

	JWTSecret               string
	JWTRefreshSecret        string
	FirebaseCredentialsFile string

	// SMTP — optional. If Host is empty the email service logs codes to stdout
	// instead of sending real emails (useful in development).
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		MongoURI:                strings.TrimSpace(os.Getenv("MONGO_URI")),
		MongoDBName:             strings.TrimSpace(os.Getenv("DB_NAME")),
		JWTSecret:               strings.TrimSpace(os.Getenv("JWT_SECRET")),
		JWTRefreshSecret:        strings.TrimSpace(os.Getenv("JWT_REFRESH_SECRET")),
		FirebaseCredentialsFile: strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_FILE")),
		SMTPHost:                strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPUsername:            strings.TrimSpace(os.Getenv("SMTP_USER")),
		SMTPPassword:            strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		SMTPFrom:                strings.TrimSpace(os.Getenv("SMTP_FROM")),
	}

	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 587
	}
	cfg.SMTPPort = port

	if cfg.SMTPFrom == "" {
		cfg.SMTPFrom = "ShopKeeper <noreply@shopkeeper.cm>"
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
	if cfg.FirebaseCredentialsFile == "" {
		return Config{}, fmt.Errorf("Missing firebase credentials file")
	}

	return cfg, nil
}
