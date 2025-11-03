package ytdlp

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseListSubsOutput(t *testing.T) {
	body := `
[youtube] info
Available subtitles for abc123:
Language     Name               Formats
en           English            vtt, srt
ru           Russian            vtt, srt

Available automatic captions for abc123:
Language     Name               Formats
en           English            vtt, srt
ru           Russian            vtt
en           English duplicate  vtt
`

	tracks := parseListSubsOutput(body)

	expected := []subtitleTrack{
		{Lang: "en", Auto: false},
		{Lang: "ru", Auto: false},
		{Lang: "en", Auto: true},
		{Lang: "ru", Auto: true},
	}

	if !reflect.DeepEqual(tracks, expected) {
		t.Fatalf("unexpected tracks: %#v", tracks)
	}
}

func TestPrioritizeSubtitleTracks(t *testing.T) {
	tracks := []subtitleTrack{
		{Lang: "fr", Auto: false},
		{Lang: "en-GB", Auto: false},
		{Lang: "ru", Auto: false},
		{Lang: "en", Auto: true},
		{Lang: "ru", Auto: true},
		{Lang: "es", Auto: true},
	}

	ordered := prioritizeSubtitleTracks(tracks)

	expected := []subtitleTrack{
		{Lang: "en-GB", Auto: false},
		{Lang: "ru", Auto: false},
		{Lang: "en", Auto: true},
		{Lang: "ru", Auto: true},
		{Lang: "fr", Auto: false},
		{Lang: "es", Auto: true},
	}

	if !reflect.DeepEqual(ordered, expected) {
		t.Fatalf("unexpected order: %#v", ordered)
	}
}

func TestMatchesLanguage(t *testing.T) {
	cases := []struct {
		code    string
		target  string
		matches bool
	}{
		{"en", "en", true},
		{"en-US", "en", true},
		{"ru-RU", "ru", true},
		{"ru", "en", false},
		{"fr", "en", false},
	}

	for _, tc := range cases {
		t.Run(tc.code+"_"+tc.target, func(t *testing.T) {
			if matchesLanguage(tc.code, tc.target) != tc.matches {
				t.Fatalf("expected matchesLanguage(%q, %q) = %t", tc.code, tc.target, tc.matches)
			}
		})
	}
}

func TestFindTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	videoID := "video"

	writeSRT(t, filepath.Join(dir, "video.en-orig.srt"), "Alt English")
	pathDirect := writeSRT(t, filepath.Join(dir, "video.en.srt"), "Primary English")

	path, err := findTranscriptFile(dir, videoID, "en")
	if err != nil {
		t.Fatalf("findTranscriptFile returned error: %v", err)
	}
	if path != pathDirect {
		t.Fatalf("expected %s, got %s", pathDirect, path)
	}

	if err := os.Remove(pathDirect); err != nil {
		t.Fatalf("remove direct file: %v", err)
	}

	path, err = findTranscriptFile(dir, videoID, "en")
	if err != nil {
		t.Fatalf("findTranscriptFile fallback error: %v", err)
	}

	expectedFallback := filepath.Join(dir, "video.en-orig.srt")
	if path != expectedFallback {
		t.Fatalf("expected fallback %s, got %s", expectedFallback, path)
	}

	if _, err := findTranscriptFile(dir, videoID, "ru"); !errors.Is(err, errNoSubtitles) {
		t.Fatalf("expected errNoSubtitles for missing language, got %v", err)
	}
}

func TestReadAnyTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	videoID := "clip"

	writeSRT(t, filepath.Join(dir, "clip.ru.srt"), "Привет")
	enPath := writeSRT(t, filepath.Join(dir, "clip.en.srt"), "Hello")

	text, err := readAnyTranscriptFile(dir, videoID)
	if err != nil {
		t.Fatalf("readAnyTranscriptFile returned error: %v", err)
	}
	if text != "Hello" {
		t.Fatalf("expected English transcript, got %q", text)
	}

	if err := os.Remove(enPath); err != nil {
		t.Fatalf("remove en file: %v", err)
	}

	text, err = readAnyTranscriptFile(dir, videoID)
	if err != nil {
		t.Fatalf("readAnyTranscriptFile fallback returned error: %v", err)
	}
	if text != "Привет" {
		t.Fatalf("expected Russian transcript, got %q", text)
	}

	if _, err := readAnyTranscriptFile(dir, "other"); !errors.Is(err, errNoSubtitles) {
		t.Fatalf("expected errNoSubtitles for missing files, got %v", err)
	}
}

func writeSRT(t *testing.T, path, text string) string {
	t.Helper()

	content := "1\n00:00:00,000 --> 00:00:01,000\n" + text + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write srt %s: %v", path, err)
	}
	return path
}
