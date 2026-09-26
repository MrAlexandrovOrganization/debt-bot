package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pb "github.com/mralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
	"github.com/mymmrac/telego"
)

const testToken = "123456789:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func testBot(t *testing.T, handler http.HandlerFunc) *telego.Bot {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	api, err := telego.NewBot(testToken, telego.WithAPIServer(server.URL),
		telego.WithHTTPClient(server.Client()), telego.WithDiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	return api
}

func TestSendAndEdit(t *testing.T) {
	var methods []string
	api := testBot(t, func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/bot"+testToken+"/")
		methods = append(methods, method)
		var params struct {
			ChatID      int64                        `json:"chat_id"`
			MessageID   int                          `json:"message_id"`
			Text        string                       `json:"text"`
			ParseMode   string                       `json:"parse_mode"`
			ReplyMarkup *telego.InlineKeyboardMarkup `json:"reply_markup"`
		}
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Error(err)
		}
		if params.ChatID != 42 || params.Text != "Имя &lt;&amp;&gt;" || params.ParseMode != "HTML" {
			t.Errorf("unexpected message: %+v", params)
		}
		if method == "editMessageText" && params.MessageID != 7 {
			t.Errorf("message ID = %d", params.MessageID)
		}
		if params.ReplyMarkup == nil || params.ReplyMarkup.InlineKeyboard[0][0].CallbackData != "back" {
			t.Errorf("unexpected keyboard: %+v", params.ReplyMarkup)
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":7,"date":1,"chat":{"id":42,"type":"private"}}}`)
	})
	kb := backKeyboard()
	sendOrEdit(t.Context(), api, 42, 0, "Имя <&>", &kb)
	sendOrEdit(t.Context(), api, 42, 7, "Имя <&>", &kb)
	if strings.Join(methods, ",") != "sendMessage,editMessageText" {
		t.Fatalf("methods = %v", methods)
	}
}

func TestCallbackNavigationAndUnavailableMessages(t *testing.T) {
	var methods []string
	api := testBot(t, func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/bot"+testToken+"/")
		methods = append(methods, method)
		if method == "answerCallbackQuery" {
			var params telego.AnswerCallbackQueryParams
			if err := json.NewDecoder(r.Body).Decode(&params); err != nil || params.CallbackQueryID != "query" {
				t.Errorf("callback answer: %+v, %v", params, err)
			}
			fmt.Fprint(w, `{"ok":true,"result":true}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":7,"date":1,"chat":{"id":42,"type":"private"}}}`)
	})
	h := NewHandler(api, nil)
	for _, message := range []string{
		`{"message_id":7,"date":1,"chat":{"id":42,"type":"private"}}`,
		`{"message_id":7,"date":0,"chat":{"id":42,"type":"private"}}`,
		`null`,
	} {
		h.sm.Reset(10)
		var cb telego.CallbackQuery
		if err := json.Unmarshal([]byte(`{"id":"query","from":{"id":10},"data":"new_deal","message":`+message+`}`), &cb); err != nil {
			t.Fatal(err)
		}
		h.dispatchCallback(t.Context(), &cb)
		want := stepIdle
		if m, ok := cb.Message.(*telego.Message); ok && m != nil {
			want = stepAwaitDealTitle
		}
		if got := h.sm.Get(10).step; got != want {
			t.Errorf("state = %q, want %q", got, want)
		}
	}
	if strings.Join(methods, ",") != "answerCallbackQuery,editMessageText,answerCallbackQuery,answerCallbackQuery" {
		t.Fatalf("methods = %v", methods)
	}
}

type participantClient struct {
	DebtClient
	platform, externalID, name, username string
}

func (c *participantClient) ResolveOrCreateUser(_ context.Context, platform, externalID, name, username string) (*pb.User, bool, error) {
	c.platform, c.externalID, c.name, c.username = platform, externalID, name, username
	return &pb.User{Id: "user", Name: name}, true, nil
}

func (c *participantClient) CreateUser(_ context.Context, name string) (*pb.User, error) {
	c.name = name
	return &pb.User{Id: "user", Name: name}, nil
}

func TestResolveParticipantForwardOrigin(t *testing.T) {
	for _, tc := range []struct{ name, origin, wantName, wantID, wantUsername string }{
		{"public", `{"type":"user","date":1,"sender_user":{"id":123,"first_name":"Анна","last_name":"Тест","username":"anna"}}`, "Анна Тест", "123", "anna"},
		{"username fallback", `{"type":"user","date":1,"sender_user":{"id":123,"first_name":"","username":"anna"}}`, "anna", "123", "anna"},
		{"hidden", `{"type":"hidden_user","date":1,"sender_user_name":"Скрытое имя"}`, "Скрытое имя", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var msg telego.Message
			if err := json.Unmarshal([]byte(`{"forward_origin":`+tc.origin+`}`), &msg); err != nil {
				t.Fatal(err)
			}
			client := &participantClient{}
			h := NewHandler(nil, client)
			user, notice, err := h.resolveParticipant(t.Context(), &msg)
			if err != nil || user == nil || notice == "" {
				t.Fatalf("resolve: %v, %q, %v", user, notice, err)
			}
			if client.name != tc.wantName || client.externalID != tc.wantID || client.username != tc.wantUsername {
				t.Errorf("resolved participant: %+v", client)
			}
			if tc.wantID != "" && client.platform != "telegram" {
				t.Errorf("platform = %q", client.platform)
			}
		})
	}
}

func TestMessageCommand(t *testing.T) {
	for _, text := range []string{"/start", "/start@debt_test_bot payload"} {
		msg := &telego.Message{Text: text, Entities: []telego.MessageEntity{{Type: "bot_command", Offset: 0, Length: len(strings.Fields(text)[0])}}}
		if got := messageCommand(msg); got != "start" {
			t.Errorf("command = %q", got)
		}
		msg.Entities = nil
		if got := messageCommand(msg); got != "" {
			t.Errorf("plain text interpreted as command: %q", got)
		}
	}
	NewHandler(nil, nil).dispatchMessage(t.Context(), &telego.Message{})
}

func TestRunCancellation(t *testing.T) {
	started := make(chan struct{})
	api := testBot(t, func(w http.ResponseWriter, r *http.Request) {
		var params telego.GetUpdatesParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Error(err)
		}
		if params.Timeout != 60 || strings.Join(params.AllowedUpdates, ",") != "message,callback_query" {
			t.Errorf("polling params: %+v", params)
		}
		close(started)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- NewHandler(api, nil).Run(ctx) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("polling did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("polling did not stop")
	}
}

func TestTelegramErrorsDoNotLogSecrets(t *testing.T) {
	var output bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	err := fmt.Errorf("Post https://api.telegram.org/bot%s/sendMessage: private text", testToken)
	recordTelegramError(t.Context(), "sendMessage", err)
	TelegramLogger{}.Errorf("%s", err)
	if strings.Contains(output.String(), testToken) || strings.Contains(output.String(), "private text") {
		t.Fatal("sensitive error content leaked")
	}
	if !strings.Contains(output.String(), "sendMessage") {
		t.Fatal("missing method diagnostic")
	}
}
