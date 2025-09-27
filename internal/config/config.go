package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config keeps runtime configuration loaded from environment variables.
type Config struct {
	TelegramToken string
	YtDLPPath     string
	OpenAIAPIKey  string
	OpenAIModel   string
	Environment   string
}

// Load populates Config from environment variables and returns an error when required values are missing.
func Load() (*Config, error) {
	cfg := &Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		YtDLPPath:     strings.TrimSpace(os.Getenv("YT_DLP_PATH")),
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

	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}

	if cfg.YtDLPPath == "" {
		cfg.YtDLPPath = "yt-dlp.exe"
	}

	if !filepath.IsAbs(cfg.YtDLPPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve working directory: %w", err)
		}
		cfg.YtDLPPath = filepath.Join(cwd, cfg.YtDLPPath)
	}

	if _, err := os.Stat(cfg.YtDLPPath); err != nil {
		return nil, fmt.Errorf("yt-dlp executable not found: %w", err)
	}

	return cfg, nil
}
