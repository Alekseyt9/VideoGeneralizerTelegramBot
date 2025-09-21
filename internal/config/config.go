package config

import (
	"fmt"
	"os"
	"strings"
)

// Config keeps runtime configuration loaded from environment variables.
type Config struct {
	TelegramToken string
	GoogleAPIKey  string
	OpenAIAPIKey  string
	OpenAIModel   string
	Environment   string
}

// Load populates Config from environment variables and returns an error when required values are missing.
func Load() (*Config, error) {
	cfg := &Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		GoogleAPIKey:  strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")),
		OpenAIAPIKey:  strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIModel:   strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		Environment:   strings.TrimSpace(os.Getenv("APP_ENV")),
	}

	if cfg.OpenAIModel == "" {
		cfg.OpenAIModel = "gpt-4o-mini"
	}

	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	if cfg.GoogleAPIKey == "" {
		return nil, fmt.Errorf("google api key is required")
	}

	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}

	return cfg, nil
}
