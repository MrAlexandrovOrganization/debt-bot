package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mralexandrov/debt-bot/frontend/telegram/internal/bot"
	observability "github.com/mralexandrov/go-observability"
	"github.com/mymmrac/telego"
)

func main() {
	token := mustEnv("TELEGRAM_BOT_TOKEN")
	backendAddr := envOr("BACKEND_ADDR", "backend:50051")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("frontend")
	slog.SetDefault(logger)

	shutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    "frontend",
		ServiceVersion: "0.1.0",
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		slog.ErrorContext(ctx, "setup observability", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdown(shutdownCtx)
	}()

	client, err := bot.NewClient(backendAddr)
	if err != nil {
		slog.ErrorContext(ctx, "create backend client", "error", err)
		os.Exit(1)
	}

	api, err := telego.NewBot(token,
		telego.WithHTTPClient(&http.Client{Timeout: 70 * time.Second}),
		telego.WithLogger(bot.TelegramLogger{}),
	)
	if err != nil {
		slog.ErrorContext(ctx, "create telegram bot failed")
		os.Exit(1)
	}

	me, err := api.GetMe(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "telegram authorization failed")
		os.Exit(1)
	}
	slog.InfoContext(ctx, "authorized on account", "username", me.Username)

	handler := bot.NewHandler(api, client)
	if err := handler.Run(ctx); err != nil {
		slog.ErrorContext(ctx, "run bot", "error", err)
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
