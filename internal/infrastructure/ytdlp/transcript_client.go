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

type subtitleTrack struct {
	Lang string
	Auto bool
}

// NewTranscriptClient creates a new yt-dlp backed transcript provider.
func NewTranscriptClient(executable string) *TranscriptClient {
	return &TranscriptClient{executable: executable}
}

// FetchTranscript returns transcript text for the given video ID by converting subtitles to plain text.
func (c *TranscriptClient) FetchTranscript(ctx context.Context, videoID string) (string, error) {
	tracks, err := c.listSubtitleTracks(ctx, videoID)
	if err != nil {
		if errors.Is(err, errNoSubtitles) {
			if existing, readErr := readAnyTranscriptFile(".", videoID); readErr == nil && existing != "" {
				return existing, nil
			}
		}
		return "", err
	}

	candidates := prioritizeSubtitleTracks(tracks)
	if len(candidates) == 0 {
		if existing, readErr := readAnyTranscriptFile(".", videoID); readErr == nil && existing != "" {
			return existing, nil
		}
		return "", errNoSubtitles
	}

	transcript, err := c.fetchWithTracks(ctx, videoID, candidates)
	if err != nil {
		if errors.Is(err, errNoSubtitles) {
			if existing, readErr := readAnyTranscriptFile(".", videoID); readErr == nil && existing != "" {
				return existing, nil
			}
		}
		return "", err
	}

	return transcript, nil
}

func (c *TranscriptClient) fetchWithTracks(ctx context.Context, videoID string, tracks []subtitleTrack) (string, error) {
	const maxAttempts = 3
	backoff := 5 * time.Second

	var lastErr error
	for _, track := range tracks {
		// Try to reuse subtitles downloaded earlier for the selected track.
		if existing, err := readTranscriptForTrack(".", videoID, track); err == nil && existing != "" {
			return existing, nil
		}

		currentBackoff := backoff
		for attempt := 0; attempt < maxAttempts; attempt++ {
			transcript, err := c.downloadTranscript(ctx, videoID, track)
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
				case <-time.After(currentBackoff):
				}
				currentBackoff *= 2
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

func (c *TranscriptClient) listSubtitleTracks(ctx context.Context, videoID string) ([]subtitleTrack, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	args := []string{
		"--list-subs",
		"--ignore-config",
		videoURL,
	}

	cmd := exec.CommandContext(ctx, c.executable, args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	trimmed := strings.TrimSpace(outputStr)
	if err != nil {
		lower := strings.ToLower(trimmed)
		switch {
		case strings.Contains(lower, "has no subtitles"), strings.Contains(lower, "no subtitles"):
			return nil, errNoSubtitles
		case strings.Contains(lower, "too many requests"):
			return nil, fmt.Errorf("rate limited by youtube: %s", trimmed)
		default:
			return nil, fmt.Errorf("yt-dlp --list-subs failed: %w, output: %s", err, trimmed)
		}
	}

	tracks := parseListSubsOutput(outputStr)
	if len(tracks) == 0 {
		return nil, errNoSubtitles
	}

	return tracks, nil
}

type subtitleSection int

const (
	subtitleSectionNone subtitleSection = iota
	subtitleSectionManual
	subtitleSectionAuto
)

func parseListSubsOutput(body string) []subtitleTrack {
	lines := strings.Split(body, "\n")
	var tracks []subtitleTrack
	section := subtitleSectionNone
	headerSeen := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case strings.Contains(line, "Available subtitles for"):
			section = subtitleSectionManual
			headerSeen = false
			continue
		case strings.Contains(line, "Available automatic captions for"):
			section = subtitleSectionAuto
			headerSeen = false
			continue
		}

		if strings.HasPrefix(line, "[") {
			continue
		}

		if section == subtitleSectionNone {
			continue
		}

		if !headerSeen {
			if strings.HasPrefix(strings.ToLower(line), "language") {
				headerSeen = true
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		lang := strings.TrimSuffix(fields[0], ":")
		tracks = append(tracks, subtitleTrack{Lang: lang, Auto: section == subtitleSectionAuto})
	}

	return deduplicateTracks(tracks)
}

func deduplicateTracks(tracks []subtitleTrack) []subtitleTrack {
	seen := make(map[string]struct{})
	result := make([]subtitleTrack, 0, len(tracks))
	for _, track := range tracks {
		key := trackKey(track)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, track)
	}
	return result
}

func trackKey(track subtitleTrack) string {
	return fmt.Sprintf("%t:%s", track.Auto, strings.ToLower(track.Lang))
}

func prioritizeSubtitleTracks(tracks []subtitleTrack) []subtitleTrack {
	var manual []subtitleTrack
	var auto []subtitleTrack
	for _, track := range tracks {
		if track.Auto {
			auto = append(auto, track)
		} else {
			manual = append(manual, track)
		}
	}

	result := make([]subtitleTrack, 0, len(tracks))
	seen := make(map[string]struct{})

	addMatches := func(list []subtitleTrack, lang string) {
		for _, track := range list {
			if !matchesLanguage(track.Lang, lang) {
				continue
			}
			key := trackKey(track)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, track)
		}
	}

	addRemaining := func(list []subtitleTrack) {
		for _, track := range list {
			key := trackKey(track)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, track)
		}
	}

	addMatches(manual, "en")
	addMatches(manual, "ru")
	addMatches(auto, "en")
	addMatches(auto, "ru")
	addRemaining(manual)
	addRemaining(auto)

	return result
}

func matchesLanguage(code, target string) bool {
	code = strings.ToLower(code)
	target = strings.ToLower(target)

	if code == target {
		return true
	}

	return strings.HasPrefix(code, target+"-")
}

func (c *TranscriptClient) downloadTranscript(ctx context.Context, videoID string, track subtitleTrack) (string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	args := []string{
		"--skip-download",
		"--sub-format", "srt",
		"--sub-langs", track.Lang,
		"--output", "%(id)s.%(ext)s",
		"--ignore-config",
	}
	if track.Auto {
		args = append(args, "--write-auto-sub")
	} else {
		args = append(args, "--write-sub")
	}
	args = append(args, videoURL)

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

	transcript, err := readTranscriptForTrack(".", videoID, track)
	if err != nil {
		return "", err
	}

	return transcript, nil
}

func isRateLimitError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "too many requests")
}

func readTranscriptForTrack(dir, videoID string, track subtitleTrack) (string, error) {
	path, err := findTranscriptFile(dir, videoID, track.Lang)
	if err != nil {
		return "", err
	}

	return readTranscriptFromPath(path)
}

func readTranscriptFromPath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read subtitles: %w", err)
	}

	transcript := parseSRT(string(data))
	if transcript == "" {
		return "", fmt.Errorf("empty transcript returned")
	}

	return transcript, nil
}

func findTranscriptFile(dir, videoID, lang string) (string, error) {
	base := fmt.Sprintf("%s.%s", videoID, lang)
	direct := filepath.Join(dir, base+".srt")
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("locate subtitles: %w", err)
	}

	pattern := filepath.Join(dir, base+"*.srt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("locate subtitles: %w", err)
	}

	if len(matches) == 0 {
		return "", errNoSubtitles
	}

	sort.Strings(matches)
	return matches[0], nil
}

func readAnyTranscriptFile(dir, videoID string) (string, error) {
	pattern := filepath.Join(dir, fmt.Sprintf("%s.*.srt", videoID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("locate subtitles: %w", err)
	}

	if len(matches) == 0 {
		return "", errNoSubtitles
	}

	sort.Strings(matches)

	priorityLanguages := []string{"en", "ru"}
	for _, lang := range priorityLanguages {
		for _, candidate := range matches {
			code := extractLanguageFromFilename(candidate, videoID)
			if matchesLanguage(code, lang) {
				return readTranscriptFromPath(candidate)
			}
		}
	}

	return readTranscriptFromPath(matches[0])
}

func extractLanguageFromFilename(path, videoID string) string {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, videoID+".") || !strings.HasSuffix(base, ".srt") {
		return ""
	}
	trimmed := strings.TrimPrefix(base, videoID+".")
	return strings.TrimSuffix(trimmed, ".srt")
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
