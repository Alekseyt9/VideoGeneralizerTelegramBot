package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"videogeneralizertelegrambot/internal/application/ports"
	"videogeneralizertelegrambot/internal/application/usecase"
)

// Bot keeps telegram bot API client and orchestrates incoming updates handling.
type Bot struct {
	api       *tgbotapi.BotAPI
	summaryUC *usecase.SummarizeVideo
	log       ports.Logger
}

// NewBot builds telegram bot instance.
func NewBot(token string, summaryUC *usecase.SummarizeVideo, log ports.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Bot{api: api, summaryUC: summaryUC, log: log}, nil
}

// Run starts telegram updates loop until context cancellation.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return fmt.Errorf("telegram updates channel closed")
			}
			if update.Message == nil || update.Message.Text == "" {
				continue
			}
			go b.handleMessage(ctx, update.Message)
		}
	}
}

func (b *Bot) handleMessage(parent context.Context, msg *tgbotapi.Message) {
	if msg.Chat == nil {
		return
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	if !strings.Contains(text, "youtu") {
		b.reply(ctx, msg.Chat.ID, "Пришлите ссылку на YouTube видео.")
		return
	}

	b.reply(ctx, msg.Chat.ID, "Обрабатываю видео, это может занять несколько минут...")

	summary, err := b.summaryUC.Execute(ctx, text)
	if err != nil {
		b.log.Error(ctx, "failed to summarize video", "error", err)
		b.reply(ctx, msg.Chat.ID, "Не удалось обработать видео, попробуйте позже.")
		return
	}

	response := tgbotapi.NewMessage(msg.Chat.ID, summary)
	response.DisableWebPagePreview = true
	if _, err = b.api.Send(response); err != nil {
		b.log.Error(ctx, "failed to send summary", "error", err)
	}
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error(ctx, "failed to send message", "error", err)
	}
}
