package ports

import "context"

// TranscriptProvider fetches transcript of a video by its ID.
type TranscriptProvider interface {
	FetchTranscript(ctx context.Context, videoID string) (string, error)
}

// Summarizer generates a text summary based on provided prompt body.
type Summarizer interface {
	Summarize(ctx context.Context, body string) (string, error)
}

// Logger hides concrete logging implementation behind a simple contract.
type Logger interface {
	Info(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
}
