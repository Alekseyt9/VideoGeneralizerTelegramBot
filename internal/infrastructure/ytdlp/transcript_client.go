package ytdlp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var errNoSubtitles = errors.New("no subtitles downloaded")

// TranscriptClient fetches subtitles by calling the local yt-dlp executable.
type TranscriptClient struct {
	executable string
}

// NewTranscriptClient creates a new yt-dlp backed transcript provider.
func NewTranscriptClient(executable string) *TranscriptClient {
	return &TranscriptClient{executable: executable}
}

// FetchTranscript returns transcript text for the given video ID by converting subtitles to plain text.
func (c *TranscriptClient) FetchTranscript(ctx context.Context, videoID string) (string, error) {
	if transcript, err := c.fetchWithLanguages(ctx, videoID, []string{"ru", "ru.*"}); err == nil {
		return transcript, nil
	} else if !errors.Is(err, errNoSubtitles) {
		return "", err
	}

	transcript, err := c.fetchWithLanguages(ctx, videoID, []string{"en", "en.*"})
	if err != nil {
		return "", err
	}

	return transcript, nil
}

func (c *TranscriptClient) fetchWithLanguages(ctx context.Context, videoID string, langs []string) (string, error) {
	const maxAttempts = 3
	backoff := 5 * time.Second

	var lastErr error
	for _, lang := range langs {
		// Try to reuse subtitles downloaded earlier.
		if existing, err := readTranscriptFile(".", videoID); err == nil && existing != "" {
			return existing, nil
		}

		for attempt := 0; attempt < maxAttempts; attempt++ {
			transcript, err := c.downloadTranscript(ctx, videoID, lang)
			if err == nil {
				return transcript, nil
			}

			lastErr = err

			if isRateLimitError(err) {
				if attempt == maxAttempts-1 {
					break
				}
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
				continue
			}

			if errors.Is(err, errNoSubtitles) {
				break
			}

			return "", err
		}
	}

	if lastErr == nil {
		lastErr = errNoSubtitles
	}

	return "", lastErr
}

func (c *TranscriptClient) downloadTranscript(ctx context.Context, videoID, lang string) (string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	args := []string{
		"--skip-download",
		"--write-sub",
		"--write-auto-sub",
		"--sub-format", "srt",
		"--sub-langs", lang,
		"--output", "%(id)s.%(ext)s",
		"--ignore-config",
		videoURL,
	}

	cmd := exec.CommandContext(ctx, c.executable, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		lower := strings.ToLower(trimmed)
		switch {
		case strings.Contains(lower, "no subtitles"), strings.Contains(lower, "subtitles for language"):
			return "", errNoSubtitles
		case strings.Contains(lower, "too many requests"):
			return "", fmt.Errorf("rate limited by youtube: %s", trimmed)
		default:
			return "", fmt.Errorf("yt-dlp failed: %w, output: %s", err, trimmed)
		}
	}

	transcript, err := readTranscriptFile(".", videoID)
	if err != nil {
		return "", err
	}

	return transcript, nil
}

func isRateLimitError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "too many requests")
}

func readTranscriptFile(dir, videoID string) (string, error) {
	pattern := filepath.Join(dir, fmt.Sprintf("%s.*.srt", videoID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("locate subtitles: %w", err)
	}

	if len(matches) == 0 {
		return "", errNoSubtitles
	}

	sort.Strings(matches)

	chosen := matches[0]
	for _, candidate := range matches {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, ".ru.") || strings.Contains(lower, ".ru-") {
			chosen = candidate
			break
		}
		if strings.Contains(lower, ".en.") || strings.Contains(lower, ".en-") {
			chosen = candidate
		}
	}

	data, err := os.ReadFile(chosen)
	if err != nil {
		return "", fmt.Errorf("read subtitles: %w", err)
	}

	transcript := parseSRT(string(data))
	if transcript == "" {
		return "", fmt.Errorf("empty transcript returned")
	}

	return transcript, nil
}

func parseSRT(body string) string {
	lines := strings.Split(body, "\n")
	var builder strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isTimecode(trimmed) || isSequenceNumber(trimmed) {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(trimmed)
	}
	return builder.String()
}

func isTimecode(line string) bool {
	return strings.Contains(line, "-->")
}

func isSequenceNumber(line string) bool {
	if line == "" {
		return false
	}
	for _, ch := range line {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
