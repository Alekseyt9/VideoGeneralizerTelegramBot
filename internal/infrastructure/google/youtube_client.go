package google

import (
	"context"
	"fmt"
	"io"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// YouTubeTranscriptClient fetches subtitles by hitting Google YouTube Data API.
type YouTubeTranscriptClient struct {
	service *youtube.Service
}

// NewYouTubeTranscriptClient wires the YouTube Data API client using a simple API key.
func NewYouTubeTranscriptClient(ctx context.Context, apiKey string) (*YouTubeTranscriptClient, error) {
	svc, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("create youtube service: %w", err)
	}
	return &YouTubeTranscriptClient{service: svc}, nil
}

// FetchTranscript returns raw transcript text for the given video ID by downloading first caption track.
func (c *YouTubeTranscriptClient) FetchTranscript(ctx context.Context, videoID string) (string, error) {
	listCall := c.service.Captions.List([]string{"snippet"}, videoID).Context(ctx)
	captions, err := listCall.Do()
	if err != nil {
		return "", fmt.Errorf("captions list: %w", err)
	}

	if len(captions.Items) == 0 {
		return "", fmt.Errorf("no captions available for video %s", videoID)
	}

	captionID := captions.Items[0].Id
	for _, item := range captions.Items {
		if item.Snippet == nil {
			continue
		}
		if !item.Snippet.IsDraft && item.Snippet.Language == "ru" {
			captionID = item.Id
			break
		}
		if !item.Snippet.IsDraft && item.Snippet.TrackKind == "ASR" {
			captionID = item.Id
		}
	}

	resp, err := c.service.Captions.Download(captionID).Tfmt("srt").Context(ctx).Download()
	if err != nil {
		return "", fmt.Errorf("captions download: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read captions: %w", err)
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
