package video

import (
	"errors"
	"net/url"
	"strings"
)

var (
	// ErrInvalidURL indicates that provided URL is empty or malformed.
	ErrInvalidURL = errors.New("invalid video url")
	// ErrUnsupportedHost indicates that URL does not belong to YouTube domains.
	ErrUnsupportedHost = errors.New("unsupported video host")
)

// ExtractVideoID parses YouTube URLs and returns canonical video identifier.
func ExtractVideoID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidURL
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidURL
	}

	host := strings.ToLower(parsed.Host)
	switch host {
	case "www.youtube.com", "youtube.com", "m.youtube.com", "music.youtube.com":
		if parsed.Path == "/watch" || parsed.Path == "/" {
			query := parsed.Query().Get("v")
			if query == "" {
				return "", ErrInvalidURL
			}
			return query, nil
		}
		if strings.HasPrefix(parsed.Path, "/shorts/") {
			return strings.TrimPrefix(parsed.Path, "/shorts/"), nil
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 1 && parts[0] != "" {
			return parts[len(parts)-1], nil
		}
	case "youtu.be":
		id := strings.Trim(parsed.Path, "/")
		if id == "" {
			return "", ErrInvalidURL
		}
		return id, nil
	default:
		return "", ErrUnsupportedHost
	}

	return "", ErrInvalidURL
}
