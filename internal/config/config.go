package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	Port             string
	DatabaseURL      string
	FrontendURL      string
	SessionSecret    string
	CookieSecure     bool
	AttachmentBucket string
	AWSRegion        string
}

func Load() (Config, error) {
	databaseURL, err := loadDatabaseURL()
	if err != nil {
		return Config{}, err
	}

	cookieSecure, err := strconv.ParseBool(getEnv("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE must be true or false")
	}

	cfg := Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      databaseURL,
		FrontendURL:      getEnv("FRONTEND_URL", "http://localhost:5173"),
		SessionSecret:    os.Getenv("SESSION_SECRET"),
		CookieSecure:     cookieSecure,
		AttachmentBucket: getEnv("ATTACHMENT_BUCKET", "todoku-attachments-local"),
		AWSRegion:        getEnv("AWS_REGION", "ap-northeast-1"),
	}

	if len(cfg.SessionSecret) < 32 {
		return Config{}, fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func loadDatabaseURL() (string, error) {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL, nil
	}

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	if host == "" || user == "" || password == "" {
		return "", fmt.Errorf("DATABASE_URL or DB_HOST, DB_USER and DB_PASSWORD are required")
	}

	databaseName := getEnv("DB_NAME", "postgres")
	port := getEnv("DB_PORT", "5432")
	sslMode := getEnv("DB_SSLMODE", "require")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   databaseName,
	}
	query := u.Query()
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()

	return u.String(), nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
