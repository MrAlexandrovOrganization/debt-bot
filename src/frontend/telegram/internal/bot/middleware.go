package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/mralexandrov/debt-bot/frontend/telegram/internal/tgfmt"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	tu "github.com/mymmrac/telego/telegoutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func send(ctx context.Context, api *telego.Bot, chatID int64, text string, kb *telego.InlineKeyboardMarkup) {
	ctx, span := tracer.Start(ctx, "send")
	defer span.End()

	msg := &telego.SendMessageParams{
		ChatID: tu.ID(chatID), Text: tgfmt.Escape(text).String(), ParseMode: telego.ModeHTML,
	}
	if kb != nil {
		msg.ReplyMarkup = kb
	}
	if _, err := api.SendMessage(ctx, msg); err != nil {
		recordTelegramError(ctx, "sendMessage", err)
	}
}

func editText(ctx context.Context, api *telego.Bot, chatID int64, msgID int, text string, kb *telego.InlineKeyboardMarkup) {
	ctx, span := tracer.Start(ctx, "editText")
	defer span.End()

	msg := &telego.EditMessageTextParams{
		ChatID: tu.ID(chatID), MessageID: msgID, Text: tgfmt.Escape(text).String(),
		ParseMode: telego.ModeHTML, ReplyMarkup: kb,
	}
	if _, err := api.EditMessageText(ctx, msg); err != nil {
		recordTelegramError(ctx, "editMessageText", err)
	}
}

func sendOrEdit(ctx context.Context, api *telego.Bot, chatID int64, msgID int, text string, kb *telego.InlineKeyboardMarkup) {
	ctx, span := tracer.Start(ctx, "sendOrEdit")
	defer span.End()

	if msgID != 0 {
		editText(ctx, api, chatID, msgID, text, kb)
	} else {
		send(ctx, api, chatID, text, kb)
	}
}

func backKeyboard() telego.InlineKeyboardMarkup {
	return *tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("← Назад").WithCallbackData("back"),
		),
	)
}

// Transport errors may contain the token-bearing request URL. Log only the
// method and numeric API code, never the raw error or response body.
func recordTelegramError(ctx context.Context, method string, err error) {
	var apiErr *telegoapi.Error
	code := 0
	if errors.As(err, &apiErr) {
		code = apiErr.ErrorCode
	}
	slog.ErrorContext(ctx, "telegram request failed", "method", method, "error_code", code)
}

// TelegramLogger keeps SDK transport diagnostics out of logs: they may include
// request URLs, tokens and message contents. Application calls log API codes above.
type TelegramLogger struct{}

func (TelegramLogger) Debugf(string, ...any) {}
func (TelegramLogger) Errorf(string, ...any) {
	slog.Error("telegram SDK request failed")
}

// grpcUserMessage extracts a human-readable message from a gRPC error.
// For FAILED_PRECONDITION errors the server message is safe to show directly.
// For other errors it falls back to the provided fallback string.
func grpcUserMessage(err error, fallback string) string {
	if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
		return "⚠️ " + st.Message()
	}
	return fallback
}
