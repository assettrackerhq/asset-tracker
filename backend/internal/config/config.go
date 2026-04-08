package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	DatabaseURL           string
	JWTSecret             string
	Port                  string
	ReplicatedSDKEndpoint string
	MetricsInterval       time.Duration
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

	sdkEndpoint := os.Getenv("REPLICATED_SDK_ENDPOINT")
	if sdkEndpoint == "" {
		sdkEndpoint = "http://asset-tracker-sdk:3000"
	}

	metricsInterval := 4 * time.Hour
	if v := os.Getenv("METRICS_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_INTERVAL: %w", err)
		}
		metricsInterval = d
	}

	return &Config{
		DatabaseURL:           dbURL,
		JWTSecret:             jwtSecret,
		Port:                  port,
		ReplicatedSDKEndpoint: sdkEndpoint,
		MetricsInterval:       metricsInterval,
	}, nil
}
