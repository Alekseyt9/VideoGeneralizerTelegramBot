package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"videogeneralizertelegrambot/internal/application/usecase"
	"videogeneralizertelegrambot/internal/config"
	googleinfra "videogeneralizertelegrambot/internal/infrastructure/google"
	"videogeneralizertelegrambot/internal/infrastructure/logger"
	openaiinfra "videogeneralizertelegrambot/internal/infrastructure/openai"
	teleinfra "videogeneralizertelegrambot/internal/infrastructure/telegram"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logProvider := logger.New(cfg.Environment)

	ytClient, err := googleinfra.NewYouTubeTranscriptClient(ctx, cfg.GoogleAPIKey)
	if err != nil {
		log.Fatalf("init youtube client: %v", err)
	}

	summarizer := openaiinfra.NewSummarizer(cfg.OpenAIAPIKey, cfg.OpenAIModel)

	summarizeVideo := usecase.NewSummarizeVideo(logProvider, ytClient, summarizer)

	bot, err := teleinfra.NewBot(cfg.TelegramToken, summarizeVideo, logProvider)
	if err != nil {
		log.Fatalf("init telegram bot: %v", err)
	}

	if err := bot.Run(ctx); err != nil {
		logProvider.Error(ctx, "bot stopped with error", "error", err)
	}
}
