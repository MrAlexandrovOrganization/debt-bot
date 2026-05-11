package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pb "github.com/mralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
	"go.opentelemetry.io/otel/attribute"
)

// StateManager manages per-user FSM state.
// The default implementation is in-memory; swap for a persistent backend as needed.
type StateManager interface {
	Get(userID int64) *userState
	Reset(userID int64)
}

type inMemoryStateManager struct {
	mu     sync.Mutex
	states map[int64]*userState
}

func newInMemoryStateManager() *inMemoryStateManager {
	return &inMemoryStateManager{states: make(map[int64]*userState)}
}

func (m *inMemoryStateManager) Get(userID int64) *userState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.states[userID]; !ok {
		m.states[userID] = newUserState()
	}
	return m.states[userID]
}

func (m *inMemoryStateManager) Reset(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[userID] = newUserState()
}

func newUserState() *userState {
	return &userState{
		participantNames: make(map[string]string),
		purchaseAmounts:  make(map[string]int64),
	}
}

const platform = "telegram"

// FSM steps
const (
	stepIdle                    = ""
	stepAwaitDealTitle          = "await_deal_title"
	stepAwaitParticipantName    = "await_participant_name"
	stepAwaitPurchaseTitle      = "await_purchase_title"
	stepAwaitPurchaseAmount     = "await_purchase_amount"
	stepAwaitPurchasePayer      = "await_purchase_payer"
	stepAwaitPurchaseSplitMode  = "await_purchase_split_mode"
	stepAwaitPurchasePayerShare = "await_purchase_payer_share"
	stepAwaitAmountsEntry       = "await_amounts_entry"
	stepDealCovSelectPayer      = "deal_cov_select_payer"
	stepDealCovSelectCovered    = "deal_cov_select_covered"
	stepAwaitPaymentFrom        = "await_payment_from"
	stepAwaitPaymentTo          = "await_payment_to"
	stepAwaitPaymentAmount      = "await_payment_amount"
)

type userState struct {
	step                      string
	dealID                    string
	purchaseTitle             string
	purchaseAmt               int64
	purchasePayerID           string
	purchaseSplitMode         string
	purchasePayerShare        int64
	purchaseAmounts           map[string]int64 // participant_id → amount
	pendingAmountParticipants []string         // ordered list for amounts loop
	pendingAmountIdx          int              // current participant index
	pendingCovPayerID         string
	pendingPaymentFromID      string
	// cache: participant id → name
	participantNames map[string]string
}

type Handler struct {
	api    *tgbotapi.BotAPI
	client DebtClient
	sm     StateManager
}

func NewHandler(api *tgbotapi.BotAPI, client DebtClient) *Handler {
	return &Handler{
		api:    api,
		client: client,
		sm:     newInMemoryStateManager(),
	}
}

func (h *Handler) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.api.GetUpdatesChan(u)
	for update := range updates {
		if update.CallbackQuery != nil {
			h.dispatchCallback(update.CallbackQuery)
		} else if update.Message != nil {
			h.dispatchMessage(update.Message)
		}
	}
	return nil
}

func (h *Handler) dispatchMessage(msg *tgbotapi.Message) {
	ctx, span := tracer.Start(context.Background(), "tg.message")
	defer span.End()
	span.SetAttributes(
		attribute.Int64("tg.user_id", msg.From.ID),
		attribute.Int64("tg.chat_id", msg.Chat.ID),
	)
	if msg.IsCommand() {
		span.SetAttributes(attribute.String("tg.command", msg.Command()))
	}
	h.handleMessage(ctx, msg)
}

func (h *Handler) dispatchCallback(cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(context.Background(), "tg.callback")
	defer span.End()
	span.SetAttributes(
		attribute.Int64("tg.user_id", cb.From.ID),
		attribute.Int64("tg.chat_id", cb.Message.Chat.ID),
		attribute.String("tg.callback_data", cb.Data),
	)
	h.handleCallback(ctx, cb)
}

// --- Navigation helpers ---
// These helpers combine FSM state reset with screen rendering, so every
// "navigate to X" transition is expressed in one place regardless of where
// it is triggered (back button, forward flow, deep link, etc.).

func (h *Handler) navigateToMainMenu(ctx context.Context, tgID, chatID int64, msgID int) {
	h.sm.Reset(tgID)
	h.showMainMenu(ctx, chatID, msgID, "Главное меню:")
}

func (h *Handler) navigateToDeal(ctx context.Context, tgID, chatID int64, msgID int, dealID string) {
	h.sm.Reset(tgID)
	h.showDealMenu(ctx, chatID, msgID, dealID)
}

// --- UI screens ---

func (h *Handler) showMainMenu(ctx context.Context, chatID int64, msgID int, text string) {
	ctx, span := tracer.Start(ctx, "showMainMenu")
	defer span.End()

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Мои сделки", "my_deals"),
		),
	)
	sendOrEdit(ctx, h.api, chatID, msgID, text, &kb)
}

func (h *Handler) showDealsList(ctx context.Context, chatID int64, msgID int, userID string) {
	ctx, span := tracer.Start(ctx, "showDealsList")
	defer span.End()

	deals, err := h.client.ListUserDeals(ctx, userID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке сделок.", nil)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	text := "Ваши сделки:"
	if len(deals) == 0 {
		text = "У вас пока нет сделок."
	}
	for _, d := range deals {
		label := fmt.Sprintf("📦 %s (%d чел.)", d.Title, len(d.ParticipantIds))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "deal:"+d.Id),
		))
	}
	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("➕ Создать сделку", "new_deal")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← Назад", "main_menu")),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, text, &kb)
}

func (h *Handler) showDealMenu(ctx context.Context, chatID int64, msgID int, dealID string) {
	ctx, span := tracer.Start(ctx, "showDealMenu")
	defer span.End()

	deal, err := h.client.GetDeal(ctx, dealID)
	if err != nil {
		send(ctx, h.api, chatID, "Ошибка при загрузке сделки.", nil)
		return
	}
	covCount := len(deal.Coverages)
	covLabel := "👥 Покрытие"
	if covCount > 0 {
		covLabel = fmt.Sprintf("👥 Покрытие (%d)", covCount)
	}
	text := fmt.Sprintf("📦 %s\nУчастников: %d", deal.Title, len(deal.ParticipantIds))
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Участники", "participants:"+dealID),
			tgbotapi.NewInlineKeyboardButtonData("🛍 Покупки", "purchases:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Рассчитать", "calculate:"+dealID),
			tgbotapi.NewInlineKeyboardButtonData(covLabel, "deal_coverages:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💸 Платежи", "payments:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← К сделкам", "my_deals"),
		),
	)
	sendOrEdit(ctx, h.api, chatID, msgID, text, &kb)
}

func (h *Handler) showDealCoverageMenu(ctx context.Context, chatID int64, msgID int, dealID string) {
	ctx, span := tracer.Start(ctx, "showDealCoverageMenu")
	defer span.End()

	deal, err := h.client.GetDeal(ctx, dealID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке сделки.", nil)
		return
	}

	names := make(map[string]string)
	var sb strings.Builder
	sb.WriteString("👥 Покрытие расходов\n")
	sb.WriteString("(кто платит за кого во всех покупках сделки)\n")

	var rows [][]tgbotapi.InlineKeyboardButton

	if len(deal.Coverages) == 0 {
		sb.WriteString("\nПокрытий нет.")
	} else {
		sb.WriteString("\nТекущие покрытия:\n")
		for _, cov := range deal.Coverages {
			payer := resolveUserName(ctx, h.client, cov.PayerId, names)
			covered := resolveUserName(ctx, h.client, cov.CoveredId, names)
			fmt.Fprintf(&sb, "• %s платит за %s\n", payer, covered)
			removeLabel := fmt.Sprintf("❌ %s→%s", payer, covered)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				// "deal_cov_remove:{coveredID}" → 16+36=52 chars ✓
				tgbotapi.NewInlineKeyboardButtonData(removeLabel, "deal_cov_remove:"+cov.CoveredId),
			))
		}
	}

	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить покрытие", "deal_cov_add"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID),
		),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, sb.String(), &kb)
}

func (h *Handler) showDealCovPayerKeyboard(ctx context.Context, chatID int64, msgID int, participants []*pb.User) {
	ctx, span := tracer.Start(ctx, "showDealCovPayerKeyboard")
	defer span.End()

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range participants {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			// "deal_cov_payer:{payerID}" → 15+36=51 chars ✓
			tgbotapi.NewInlineKeyboardButtonData(p.Name, "deal_cov_payer:"+p.Id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal_cov_back"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, "Кто платит за другого?", &kb)
}

func (h *Handler) showDealCovCoveredKeyboard(ctx context.Context, chatID int64, msgID int, st *userState) {
	ctx, span := tracer.Start(ctx, "showDealCovCoveredKeyboard")
	defer span.End()

	payerName := st.participantNames[st.pendingCovPayerID]
	var rows [][]tgbotapi.InlineKeyboardButton
	for id, name := range st.participantNames {
		if id == st.pendingCovPayerID {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			// "deal_cov_covered:{coveredID}" → 17+36=53 chars ✓
			tgbotapi.NewInlineKeyboardButtonData(name, "deal_cov_covered:"+id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal_cov_add"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, fmt.Sprintf("За кого платит %s?", payerName), &kb)
}

func (h *Handler) showParticipants(ctx context.Context, chatID int64, msgID int, dealID string) {
	ctx, span := tracer.Start(ctx, "showParticipants")
	defer span.End()

	deal, err := h.client.GetDeal(ctx, dealID)
	if err != nil {
		sendOrEdit(ctx, h.api, chatID, msgID, "Ошибка при загрузке участников.", nil)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	text := "👤 Участники сделки:\n\nУчастников пока нет."
	if len(deal.ParticipantIds) > 0 {
		users, err := fetchUsers(ctx, h.client, deal.ParticipantIds)
		if err != nil {
			sendOrEdit(ctx, h.api, chatID, msgID, "Ошибка при загрузке имён участников.", nil)
			return
		}
		text = "👤 Участники сделки:"
		for _, u := range users {
			// "del_participant:{userID}" → 17+36=53 chars ✓
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(u.Name, "noop"),
				tgbotapi.NewInlineKeyboardButtonData("❌", "del_participant:"+u.Id),
			))
		}
	}

	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить участника", "add_participant:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID),
		),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, text, &kb)
}

func (h *Handler) showPurchases(ctx context.Context, chatID int64, msgID int, dealID string) {
	ctx, span := tracer.Start(ctx, "showPurchases")
	defer span.End()

	purchases, err := h.client.ListDealPurchases(ctx, dealID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке покупок.", nil)
		return
	}

	bottomKb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить покупку", "add_purchase:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID),
		),
	)

	if len(purchases) == 0 {
		sendOrEdit(ctx, h.api, chatID, msgID, "Покупок пока нет.", &bottomKb)
		return
	}

	names := make(map[string]string)
	var sb strings.Builder
	sb.WriteString("🛍 Покупки:\n\n")
	var total int64
	var purchaseRows [][]tgbotapi.InlineKeyboardButton
	for _, p := range purchases {
		payerName := resolveUserName(ctx, h.client, p.PaidBy, names)
		fmt.Fprintf(&sb, "• %s — %s ₽ (платил %s)\n", p.Title, formatAmount(p.Amount), payerName)
		total += p.Amount
		// "del_purchase:{purchaseID}" → 13+36=49 chars ✓
		purchaseRows = append(purchaseRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ "+p.Title, "del_purchase:"+p.Id),
		))
	}
	fmt.Fprintf(&sb, "\nИтого: %s ₽", formatAmount(total))

	// Merge: delete buttons per purchase + bottom action buttons
	allRows := append(purchaseRows, bottomKb.InlineKeyboard...)
	fullKb := tgbotapi.NewInlineKeyboardMarkup(allRows...)
	sendOrEdit(ctx, h.api, chatID, msgID, sb.String(), &fullKb)
}

func (h *Handler) showCalculation(ctx context.Context, chatID int64, msgID int, dealID string, currentUserID string) {
	ctx, span := tracer.Start(ctx, "showCalculation")
	defer span.End()

	result, err := h.client.CalculateDebts(ctx, dealID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при расчёте.", nil)
		return
	}

	back := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID)),
	)

	var sb strings.Builder

	if len(result.Debts) == 0 {
		sb.WriteString("✅ Все в расчёте, долгов нет!")
	} else {
		names := make(map[string]string)
		sb.WriteString("Взаиморасчёты:\n\n")
		for _, d := range result.Debts {
			from := resolveUserName(ctx, h.client, d.FromUserId, names)
			to := resolveUserName(ctx, h.client, d.ToUserId, names)
			fmt.Fprintf(&sb, "• %s → %s: %s ₽\n", from, to, formatAmount(d.Amount))
		}
	}

	// Show personal summary for the current user
	if currentUserID != "" && result.Summaries != nil {
		if s, ok := result.Summaries[currentUserID]; ok && s.TotalPaid > 0 {
			sb.WriteString("\n💰 Ваш итог:\n")
			fmt.Fprintf(&sb, "  Заплатили: %s ₽\n", formatAmount(s.TotalPaid))
			fmt.Fprintf(&sb, "  Ваша доля: %s ₽\n", formatAmount(s.OwnShare))
			fmt.Fprintf(&sb, "  Ожидаете от других: %s ₽\n", formatAmount(s.ExpectedFromOthers))
			fmt.Fprintf(&sb, "  Уже получено: %s ₽\n", formatAmount(s.PaymentsReceived))
			fmt.Fprintf(&sb, "  Ещё ждёте: %s ₽\n", formatAmount(s.StillAwaiting))
		}
	}

	sendOrEdit(ctx, h.api, chatID, msgID, sb.String(), &back)
}

func (h *Handler) showPayments(ctx context.Context, chatID int64, msgID int, dealID string) {
	ctx, span := tracer.Start(ctx, "showPayments")
	defer span.End()

	payments, err := h.client.ListDealPayments(ctx, dealID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке платежей.", nil)
		return
	}

	var sb strings.Builder
	sb.WriteString("💸 Платежи:\n\n")

	var rows [][]tgbotapi.InlineKeyboardButton
	names := make(map[string]string)

	if len(payments) == 0 {
		sb.WriteString("Платежей пока нет.")
	} else {
		for _, p := range payments {
			from := resolveUserName(ctx, h.client, p.FromUserId, names)
			to := resolveUserName(ctx, h.client, p.ToUserId, names)
			fmt.Fprintf(&sb, "• %s → %s: %s ₽\n", from, to, formatAmount(p.Amount))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ "+from+"→"+to, "del_payment:"+p.Id),
			))
		}
	}

	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить платёж", "add_payment:"+dealID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Назад", "deal:"+dealID),
		),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, sb.String(), &kb)
}

func (h *Handler) showPaymentFromKeyboard(ctx context.Context, chatID int64, msgID int, participants []*pb.User) {
	ctx, span := tracer.Start(ctx, "showPaymentFromKeyboard")
	defer span.End()

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range participants {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(p.Name, "payment_from:"+p.Id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Назад", "back"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, "Кто перевёл деньги?", &kb)
}

func (h *Handler) showPaymentToKeyboard(ctx context.Context, chatID int64, msgID int, participants []*pb.User, excludeID string) {
	ctx, span := tracer.Start(ctx, "showPaymentToKeyboard")
	defer span.End()

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range participants {
		if p.Id == excludeID {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(p.Name, "payment_to:"+p.Id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Назад", "back"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	text := "Кому перевёл?"
	if len(rows) == 1 {
		text = "Нет других участников для выбора получателя."
	}
	sendOrEdit(ctx, h.api, chatID, msgID, text, &kb)
}

func (h *Handler) showSplitModeKeyboard(ctx context.Context, chatID int64, msgID int) {
	ctx, span := tracer.Start(ctx, "showSplitModeKeyboard")
	defer span.End()

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Поровну", "split_mode:all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Поровну — указать мою долю", "split_mode:all_share"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("По суммам каждого", "split_mode:amounts"),
		),
	)
	sendOrEdit(ctx, h.api, chatID, msgID, "Как разделить расходы?", &kb)
}

func (h *Handler) showPayerKeyboard(ctx context.Context, chatID int64, msgID int, participants []*pb.User) {
	ctx, span := tracer.Start(ctx, "showPayerKeyboard")
	defer span.End()

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range participants {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(p.Name, "payer:"+p.Id),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendOrEdit(ctx, h.api, chatID, msgID, "Кто оплатил?", &kb)
}
