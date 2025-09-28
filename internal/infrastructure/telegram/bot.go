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
	api          *tgbotapi.BotAPI
	summaryUC    *usecase.SummarizeVideo
	log          ports.Logger
	queue        chan job
	taskInterval time.Duration
}

type job struct {
	chatID     int64
	url        string
	enqueuedAt time.Time
}

// NewBot builds telegram bot instance.
func NewBot(token string, summaryUC *usecase.SummarizeVideo, log ports.Logger, taskInterval time.Duration) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Bot{api: api, summaryUC: summaryUC, log: log, queue: make(chan job, 200), taskInterval: taskInterval}, nil
}

// Run starts telegram updates loop until context cancellation.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	// Start a single worker to process jobs sequentially
	go b.worker(ctx)

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
			b.enqueue(update.Message)
		}
	}
}

// handleMessage is a legacy direct handler (unused in queue mode) kept for reference.
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
		b.reply(ctx, msg.Chat.ID, fmt.Sprintf("Ошибка: %v", err))
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

func (b *Bot) enqueue(m *tgbotapi.Message) {
	if m.Chat == nil {
		return
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		return
	}
	if !strings.Contains(text, "youtu") {
		b.reply(context.Background(), m.Chat.ID, "Пришлите ссылку на YouTube.")
		return
	}
	// Inform user and queue the job
	pos := len(b.queue) + 1
	b.reply(context.Background(), m.Chat.ID, fmt.Sprintf("Ссылка добавлена в очередь. Позиция: %d", pos))
	select {
	case b.queue <- job{chatID: m.Chat.ID, url: text, enqueuedAt: time.Now()}:
	default:
		b.reply(context.Background(), m.Chat.ID, "Очередь переполнена, попробуйте позже.")
	}
}

func (b *Bot) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-b.queue:
			// Process with timeout similar to previous logic
			jobCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			b.reply(jobCtx, j.chatID, "Начинаю обработку... Это может занять время.")
			summary, err := b.summaryUC.Execute(jobCtx, j.url)
			if err != nil {
				b.log.Error(jobCtx, "failed to summarize video", "error", err)
				b.reply(jobCtx, j.chatID, fmt.Sprintf("Ошибка: %v", err))
			} else {
				response := tgbotapi.NewMessage(j.chatID, summary)
				response.DisableWebPagePreview = true
				if _, err = b.api.Send(response); err != nil {
					b.log.Error(jobCtx, "failed to send summary", "error", err)
				}
			}
			cancel()
			// Respect interval between jobs to avoid rate limits
			select {
			case <-ctx.Done():
				return
			case <-time.After(b.taskInterval):
			}
		}
	}
}
