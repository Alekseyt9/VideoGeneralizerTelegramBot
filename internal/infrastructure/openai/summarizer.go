package openai

import (
	"context"
	"errors"

	goopenai "github.com/sashabaranov/go-openai"
)

// Summarizer wraps OpenAI chat completion endpoint.
type Summarizer struct {
	client *goopenai.Client
	model  string
}

// NewSummarizer configures OpenAI client with provided API key and model name.
func NewSummarizer(apiKey, model string) *Summarizer {
	return &Summarizer{
		client: goopenai.NewClient(apiKey),
		model:  model,
	}
}

// Summarize sends prompt to OpenAI chat completion API and returns generated text.
func (s *Summarizer) Summarize(ctx context.Context, body string) (string, error) {
	resp, err := s.client.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model:       s.model,
		Temperature: 0.3,
		Messages: []goopenai.ChatCompletionMessage{
			{Role: goopenai.ChatMessageRoleSystem, Content: "You are a concise assistant. Always deliver summaries in Russian and highlight the essential points."},
			{Role: goopenai.ChatMessageRoleUser, Content: body},
		},
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("empty completion response")
	}

	return resp.Choices[0].Message.Content, nil
}
