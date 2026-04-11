package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL                   string
	JWTSecret                     string
	Port                          string
	ReplicatedSDKEndpoint         string
	MetricsInterval               time.Duration
	SMTPHost                      string
	SMTPPort                      string
	SMTPUsername                   string
	SMTPPassword                   string
	SMTPFrom                      string
	AnalyticsEnabled              bool
	SupportBundleImage            string
	SupportBundleServiceAccount   string
	SupportBundleImagePullSecrets []string
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

	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}

	sbImage := os.Getenv("SUPPORT_BUNDLE_IMAGE")
	if sbImage == "" {
		sbImage = "replicated/troubleshoot:latest"
	}

	sbServiceAccount := os.Getenv("SUPPORT_BUNDLE_SERVICE_ACCOUNT")
	if sbServiceAccount == "" {
		sbServiceAccount = "default"
	}

	analyticsEnabled := true
	if v := os.Getenv("ANALYTICS_ENABLED"); v != "" {
		analyticsEnabled = strings.EqualFold(v, "true") || v == "1"
	}

	var sbPullSecrets []string
	if v := os.Getenv("SUPPORT_BUNDLE_IMAGE_PULL_SECRETS"); v != "" {
		for _, s := range strings.Split(v, ",") {
			if t := strings.TrimSpace(s); t != "" {
				sbPullSecrets = append(sbPullSecrets, t)
			}
		}
	}

	return &Config{
		DatabaseURL:           dbURL,
		JWTSecret:             jwtSecret,
		Port:                  port,
		ReplicatedSDKEndpoint: sdkEndpoint,
		MetricsInterval:       metricsInterval,
		SMTPHost:              os.Getenv("SMTP_HOST"),
		SMTPPort:              smtpPort,
		SMTPUsername:           os.Getenv("SMTP_USERNAME"),
		SMTPPassword:           os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                      os.Getenv("SMTP_FROM"),
		AnalyticsEnabled:              analyticsEnabled,
		SupportBundleImage:            sbImage,
		SupportBundleServiceAccount:   sbServiceAccount,
		SupportBundleImagePullSecrets: sbPullSecrets,
	}, nil
}
