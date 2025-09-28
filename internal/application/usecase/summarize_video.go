package usecase

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "videogeneralizertelegrambot/internal/application/ports"
    "videogeneralizertelegrambot/internal/domain/video"
)

// SummarizeVideo orchestrates transcript retrieval and summary generation.
type SummarizeVideo struct {
    transcripts ports.TranscriptProvider
    summarizer  ports.Summarizer
    log         ports.Logger
}

// NewSummarizeVideo builds a new use case instance.
func NewSummarizeVideo(log ports.Logger, transcripts ports.TranscriptProvider, summarizer ports.Summarizer) *SummarizeVideo {
    return &SummarizeVideo{
        transcripts: transcripts,
        summarizer:  summarizer,
        log:         log,
    }
}

// Execute resolves transcript for provided URL and returns ChatGPT summary.
func (uc *SummarizeVideo) Execute(ctx context.Context, videoURL string) (string, error) {
    videoID, err := video.ExtractVideoID(videoURL)
    if err != nil {
        uc.log.Error(ctx, "failed to parse video url", "error", err)
        return "", err
    }

    // Always clean up subtitle files on function exit (success or error)
    defer uc.cleanupSubtitles(ctx, videoID)

    uc.log.Info(ctx, "fetching transcript", "video_id", videoID)
    transcript, err := uc.transcripts.FetchTranscript(ctx, videoID)
    if err != nil {
        uc.log.Error(ctx, "failed to fetch transcript", "error", err, "video_id", videoID)
        return "", fmt.Errorf("fetch transcript: %w", err)
    }

    prompt := fmt.Sprintf("Summarize the following video in Russian. Use Telegram Markdown (asterisks for bold, underscores for italics, backticks for inline code) and structure the summary into logical paragraphs or lists as needed.\n\n%s", transcript)
    uc.log.Info(ctx, "sending transcript to summarizer", "length", len(transcript))

    summary, err := uc.summarizer.Summarize(ctx, prompt)
    if err != nil {
        uc.log.Error(ctx, "failed to summarize video", "error", err, "video_id", videoID)
        return "", fmt.Errorf("summarize video: %w", err)
    }

    uc.log.Info(ctx, "summary generated", "video_id", videoID)
    return summary, nil
}

func (uc *SummarizeVideo) cleanupSubtitles(ctx context.Context, videoID string) {
    pattern := filepath.Join(".", fmt.Sprintf("%s.*.srt", videoID))
    matches, _ := filepath.Glob(pattern)
    for _, f := range matches {
        if err := os.Remove(f); err != nil {
            uc.log.Error(ctx, "failed to remove subtitle file", "file", f, "error", err)
        } else {
            uc.log.Info(ctx, "removed subtitle file", "file", f)
        }
    }
}
