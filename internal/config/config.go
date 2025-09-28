package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"runtime"
)

// Config keeps runtime configuration loaded from environment variables.
type Config struct {
	TelegramToken string
	YtDLPPath     string
	OpenAIAPIKey  string
	OpenAIModel   string
	Environment   string
	// TaskInterval defines delay between processing queued links.
	TaskInterval int
}

// Load populates Config from environment variables and returns an error when required values are missing.
func Load() (*Config, error) {
	cfg := &Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		YtDLPPath:     strings.TrimSpace(os.Getenv("YT_DLP_PATH")),
		OpenAIAPIKey:  strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIModel:   strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		Environment:   strings.TrimSpace(os.Getenv("APP_ENV")),
		TaskInterval:  0,
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
		// Pick bundled yt-dlp from utils depending on OS
		var bin string
		if runtime.GOOS == "windows" {
			bin = filepath.Join("utils", "yt-dlp.exe")
		} else {
			bin = filepath.Join("utils", "yt-dlp_linux")
		}
		cfg.YtDLPPath = bin
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

	// Optional: interval between queued tasks in seconds; default to 15s
	if v := strings.TrimSpace(os.Getenv("TASK_INTERVAL_SECONDS")); v != "" {
		// Poor-man parsing to avoid adding deps
		var parsed int
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				parsed = 0
				break
			}
		}
		if parsed == 0 {
			// Fallback to fmt parsing if all digits
			if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil {
				cfg.TaskInterval = parsed
			}
		} else {
			cfg.TaskInterval = parsed
		}
	}
	if cfg.TaskInterval <= 0 {
		cfg.TaskInterval = 3
	}

	return cfg, nil
}
