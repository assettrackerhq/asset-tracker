package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabaseURL: dbURL,
		JWTSecret:   jwtSecret,
		Port:        port,
	}, nil
}
