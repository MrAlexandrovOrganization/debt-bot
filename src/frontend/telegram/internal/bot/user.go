package bot

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	pb "github.com/mralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
	"github.com/mymmrac/telego"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("debt-bot-frontend")

// --- User helpers (Handler methods) ---

func (h *Handler) resolveUser(ctx context.Context, from *telego.User) *pb.User {
	ctx, span := tracer.Start(ctx, "resolveUser")
	defer span.End()

	span.SetAttributes(attribute.Int64("tg.user_id", from.ID))

	name := strings.TrimSpace(from.FirstName + " " + from.LastName)
	if name == "" {
		name = from.Username
	}
	user, _, err := h.client.ResolveOrCreateUser(ctx, platform, strconv.FormatInt(from.ID, 10), name, from.Username)
	if err != nil {
		slog.ErrorContext(ctx, "resolveOrCreateUser failed", "tg_user_id", from.ID, "error", err)
		return nil
	}
	slog.InfoContext(ctx, "user resolved", "tg_user_id", from.ID, "user_id", user.Id)
	return user
}

// resolveParticipant determines how to add a participant based on the message:
//   - Forwarded message (MessageOriginUser) → link by Telegram ID
//   - Privacy-protected forward (MessageOriginHiddenUser) → plain name
//   - @username text                        → link by username (merge on first bot use)
//   - Plain text                            → name only
//
// Returns the created/found user, an optional notice string, and an error.
func (h *Handler) resolveParticipant(ctx context.Context, msg *telego.Message) (*pb.User, string, error) {
	ctx, span := tracer.Start(ctx, "resolveParticipant")
	defer span.End()

	// Case 1: forwarded message with public sender info
	if origin, ok := msg.ForwardOrigin.(*telego.MessageOriginUser); ok {
		from := origin.SenderUser
		span.SetAttributes(attribute.String("resolve.method", "forward_public"))
		name := strings.TrimSpace(from.FirstName + " " + from.LastName)
		if name == "" {
			name = from.Username
		}
		user, _, err := h.client.ResolveOrCreateUser(ctx, platform, strconv.FormatInt(from.ID, 10), name, from.Username)
		if err != nil {
			slog.ErrorContext(ctx, "resolveParticipant: forward public failed", "error", err)
			return nil, "", err
		}
		slog.InfoContext(ctx, "participant resolved via forward", "user_id", user.Id)
		return user, " Telegram-аккаунт привязан.", nil
	}

	// Case 2: forwarded message but sender hid their identity
	if origin, ok := msg.ForwardOrigin.(*telego.MessageOriginHiddenUser); ok {
		span.SetAttributes(attribute.String("resolve.method", "forward_private"))
		user, err := h.client.CreateUser(ctx, origin.SenderUserName)
		if err != nil {
			slog.ErrorContext(ctx, "resolveParticipant: forward private failed", "error", err)
			return nil, "", err
		}
		slog.InfoContext(ctx, "participant created from hidden forward", "user_id", user.Id)
		return user, " (аккаунт скрыт настройками приватности)", nil
	}

	text := strings.TrimSpace(msg.Text)

	// Case 3: @username text
	if after, ok := strings.CutPrefix(text, "@"); ok {
		span.SetAttributes(attribute.String("resolve.method", "username"))
		username := after
		if username == "" {
			send(ctx, h.api, msg.Chat.ID, "Укажите username после @.", nil)
			return nil, "", nil
		}
		user, _, err := h.client.ResolveOrCreateUser(ctx, "telegram_username", username, "@"+username, "")
		if err != nil {
			slog.ErrorContext(ctx, "resolveParticipant: username lookup failed", "username", username, "error", err)
			return nil, "", err
		}
		slog.InfoContext(ctx, "participant resolved via username", "username", username, "user_id", user.Id)
		return user, " Когда откроет бота — аккаунты свяжутся автоматически.", nil
	}

	// Case 4: plain name
	if text == "" {
		send(ctx, h.api, msg.Chat.ID, "Введите имя, @username или перешлите сообщение.", nil)
		return nil, "", nil
	}
	span.SetAttributes(attribute.String("resolve.method", "plain_name"))
	user, err := h.client.CreateUser(ctx, text)
	if err != nil {
		slog.ErrorContext(ctx, "resolveParticipant: create by name failed", "name", text, "error", err)
		return nil, "", err
	}
	slog.InfoContext(ctx, "participant created by name", "name", text, "user_id", user.Id)
	return user, "", nil
}

// --- User utility functions ---
// These are package-level functions (not Handler methods) because they only
// depend on the DebtClient, not on any other Handler state.

// fetchUsers fetches multiple users by ID in order.
func fetchUsers(ctx context.Context, client DebtClient, ids []string) ([]*pb.User, error) {
	ctx, span := tracer.Start(ctx, "fetchUsers")
	defer span.End()

	span.SetAttributes(attribute.Int("user.count", len(ids)))

	users := make([]*pb.User, 0, len(ids))
	for _, id := range ids {
		u, err := client.GetUser(ctx, id)
		if err != nil {
			slog.ErrorContext(ctx, "fetchUsers: get user failed", "user_id", id, "error", err)
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// resolveUserName returns a user's display name, using cache to avoid redundant lookups.
func resolveUserName(ctx context.Context, client DebtClient, userID string, cache map[string]string) string {
	ctx, span := tracer.Start(ctx, "resolveUserName")
	defer span.End()

	if name, ok := cache[userID]; ok {
		return name
	}
	u, err := client.GetUser(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "resolveUserName: get user failed", "user_id", userID, "error", err)
		return "?"
	}
	cache[userID] = u.Name
	return u.Name
}
