package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pb "github.com/mralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
)

// --- Callback handler (button presses) ---

func (h *Handler) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleCallback")
	defer span.End()

	tgID := cb.From.ID
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID

	h.api.Request(tgbotapi.NewCallback(cb.ID, ""))

	data := cb.Data

	exactHandlers := map[string]func(){
		"main_menu":     func() { h.handleMainMenu(ctx, tgID, chatID, msgID) },
		"new_deal":      func() { h.handleNewDeal(ctx, tgID, chatID, msgID) },
		"my_deals":      func() { h.handleMyDeals(ctx, chatID, msgID, cb) },
		"deal_cov_add":  func() { h.handleDealCoverageAdd(ctx, tgID, chatID, msgID) },
		"deal_cov_back": func() { h.handleDealCoverageBack(ctx, tgID, chatID, msgID) },
		"back":          func() { h.handleBack(ctx, tgID, chatID, msgID) },
		"noop":          func() {}, // placeholder button — no action
	}

	if handler, ok := exactHandlers[data]; ok {
		handler()
		return
	}

	prefixHandlers := []struct {
		prefix  string
		handler func()
	}{
		{"deal:", func() { h.handleDeal(ctx, tgID, chatID, msgID, cb) }},
		{"participants:", func() { h.handleParticipants(ctx, tgID, chatID, msgID, cb) }},
		{"add_participant:", func() { h.handleAddParticipant(ctx, tgID, chatID, msgID, cb) }},
		{"add_purchase:", func() { h.handleAddPurchase(ctx, tgID, chatID, msgID, cb) }},
		{"purchases:", func() { h.handlePurchases(ctx, chatID, msgID, cb) }},
		{"calculate:", func() { h.handleCalculate(ctx, chatID, msgID, cb) }},
		{"deal_coverages:", func() { h.handleDealCoverages(ctx, tgID, chatID, msgID, cb) }},
		{"deal_cov_payer:", func() { h.handleDealCoveragePayer(ctx, tgID, chatID, msgID, cb) }},
		{"deal_cov_covered:", func() { h.handleDealCoverageCovered(ctx, tgID, chatID, msgID, cb) }},
		{"deal_cov_remove:", func() { h.handleDealCoverageRemove(ctx, tgID, chatID, msgID, cb) }},
		{"del_participant:", func() { h.handleDeleteParticipant(ctx, tgID, chatID, msgID, cb) }},
		{"del_purchase:", func() { h.handleDeletePurchase(ctx, tgID, chatID, msgID, cb) }},
		{"payer:", func() { h.handlePayerSelected(ctx, tgID, chatID, msgID, cb) }},
		{"split_mode:", func() { h.handleSplitModeSelected(ctx, tgID, chatID, msgID, cb) }},
		{"payments:", func() { h.handlePayments(ctx, tgID, chatID, msgID, cb) }},
		{"add_payment:", func() { h.handleAddPayment(ctx, tgID, chatID, msgID, cb) }},
		{"payment_from:", func() { h.handlePaymentFrom(ctx, tgID, chatID, msgID, cb) }},
		{"payment_to:", func() { h.handlePaymentTo(ctx, tgID, chatID, msgID, cb) }},
		{"del_payment:", func() { h.handleDeletePayment(ctx, tgID, chatID, msgID, cb) }},
	}

	for _, ph := range prefixHandlers {
		if strings.HasPrefix(data, ph.prefix) {
			ph.handler()
			return
		}
	}

	editText(ctx, h.api, chatID, msgID, "Неизвестная команда", nil)
}

func (h *Handler) handleMainMenu(ctx context.Context, tgID, chatID int64, msgID int) {
	ctx, span := tracer.Start(ctx, "handleMainMenu")
	defer span.End()

	h.navigateToMainMenu(ctx, tgID, chatID, msgID)
}

func (h *Handler) handleNewDeal(ctx context.Context, tgID, chatID int64, msgID int) {
	ctx, span := tracer.Start(ctx, "handleNewDeal")
	defer span.End()

	h.sm.Get(tgID).step = stepAwaitDealTitle
	kb := backKeyboard()
	editText(ctx, h.api, chatID, msgID, "Введите название сделки:", &kb)
}

func (h *Handler) handleMyDeals(ctx context.Context, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleMyDeals")
	defer span.End()

	user := h.resolveUser(ctx, cb.From)
	if user == nil {
		return
	}
	h.showDealsList(ctx, chatID, msgID, user.Id)
}

func (h *Handler) handleDeal(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDeal")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "deal:")
	h.navigateToDeal(ctx, tgID, chatID, msgID, dealID)
}

func (h *Handler) handleAddParticipant(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleAddParticipant")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "add_participant:")
	st := h.sm.Get(tgID)
	st.step = stepAwaitParticipantName
	st.dealID = dealID
	kb := backKeyboard()
	editText(ctx, h.api, chatID, msgID, "Добавьте участника одним из способов:\n\n• Введите имя\n• Отправьте @username\n• Перешлите сообщение от участника", &kb)
}

func (h *Handler) handleAddPurchase(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleAddPurchase")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "add_purchase:")
	st := h.sm.Get(tgID)
	st.step = stepAwaitPurchaseTitle
	st.dealID = dealID
	kb := backKeyboard()
	editText(ctx, h.api, chatID, msgID, "Введите название покупки:", &kb)
}

func (h *Handler) handleParticipants(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleParticipants")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "participants:")
	h.sm.Get(tgID).dealID = dealID
	h.showParticipants(ctx, chatID, msgID, dealID)
}

func (h *Handler) handlePurchases(ctx context.Context, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handlePurchases")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "purchases:")
	h.sm.Get(cb.From.ID).dealID = dealID
	h.showPurchases(ctx, chatID, msgID, dealID)
}

func (h *Handler) handleCalculate(ctx context.Context, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleCalculate")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "calculate:")
	user := h.resolveUser(ctx, cb.From)
	var userID string
	if user != nil {
		userID = user.Id
	}
	h.showCalculation(ctx, chatID, msgID, dealID, userID)
}

// Deal-level coverages management screen
// "deal_coverages:{dealID}" → max 15+36=51 chars ✓
func (h *Handler) handleDealCoverages(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDealCoverages")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "deal_coverages:")
	st := h.sm.Get(tgID)
	st.dealID = dealID
	h.showDealCoverageMenu(ctx, chatID, msgID, dealID)
}

// Start adding a coverage: load participants into state, show payer keyboard
// "deal_cov_add" → 12 chars ✓ (dealID already in state)
func (h *Handler) handleDealCoverageAdd(ctx context.Context, tgID, chatID int64, msgID int) {
	ctx, span := tracer.Start(ctx, "handleDealCoverageAdd")
	defer span.End()

	st := h.sm.Get(tgID)
	if st.dealID == "" {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	deal, err := h.client.GetDeal(ctx, st.dealID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке сделки.", nil)
		return
	}
	participants, err := fetchUsers(ctx, h.client, deal.ParticipantIds)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке участников.", nil)
		return
	}
	for _, p := range participants {
		st.participantNames[p.Id] = p.Name
	}
	st.step = stepDealCovSelectPayer
	h.showDealCovPayerKeyboard(ctx, chatID, msgID, participants)
}

// Coverage payer selected → "deal_cov_payer:{payerID}" → 15+36=51 chars ✓
func (h *Handler) handleDealCoveragePayer(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDealCoveragePayer")
	defer span.End()

	payerID := strings.TrimPrefix(cb.Data, "deal_cov_payer:")
	st := h.sm.Get(tgID)
	st.pendingCovPayerID = payerID
	st.step = stepDealCovSelectCovered
	h.showDealCovCoveredKeyboard(ctx, chatID, msgID, st)
}

// Covered person selected → "deal_cov_covered:{coveredID}" → 17+36=53 chars ✓
func (h *Handler) handleDealCoverageCovered(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDealCoverageCovered")
	defer span.End()

	coveredID := strings.TrimPrefix(cb.Data, "deal_cov_covered:")
	st := h.sm.Get(tgID)
	if st.dealID == "" {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if _, err := h.client.SetDealCoverage(ctx, st.dealID, st.pendingCovPayerID, coveredID); err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при сохранении покрытия.", nil)
		return
	}
	dealID := st.dealID
	st.step = stepIdle
	st.pendingCovPayerID = ""
	h.showDealCoverageMenu(ctx, chatID, msgID, dealID)
}

// Back to coverage menu (from payer selection)
func (h *Handler) handleDealCoverageBack(ctx context.Context, tgID, chatID int64, msgID int) {
	ctx, span := tracer.Start(ctx, "handleDealCoverageBack")
	defer span.End()

	st := h.sm.Get(tgID)
	if st.dealID == "" {
		h.navigateToMainMenu(ctx, tgID, chatID, msgID)
		return
	}
	st.step = stepIdle
	h.showDealCoverageMenu(ctx, chatID, msgID, st.dealID)
}

// Remove a coverage → "deal_cov_remove:{coveredID}" → 16+36=52 chars ✓
func (h *Handler) handleDealCoverageRemove(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDealCoverageRemove")
	defer span.End()

	coveredID := strings.TrimPrefix(cb.Data, "deal_cov_remove:")
	st := h.sm.Get(tgID)
	if st.dealID == "" {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if _, err := h.client.RemoveDealCoverage(ctx, st.dealID, coveredID); err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при удалении покрытия.", nil)
		return
	}
	h.showDealCoverageMenu(ctx, chatID, msgID, st.dealID)
}

// Payer selected → show split mode keyboard
func (h *Handler) handlePayerSelected(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handlePayerSelected")
	defer span.End()

	payerID := strings.TrimPrefix(cb.Data, "payer:")
	st := h.sm.Get(tgID)
	if st.step != stepAwaitPurchasePayer {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	st.purchasePayerID = payerID
	st.step = stepAwaitPurchaseSplitMode
	h.showSplitModeKeyboard(ctx, chatID, msgID)
}

// Split mode selected
func (h *Handler) handleSplitModeSelected(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleSplitModeSelected")
	defer span.End()

	mode := strings.TrimPrefix(cb.Data, "split_mode:")
	st := h.sm.Get(tgID)
	if st.step != stepAwaitPurchaseSplitMode {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}

	switch mode {
	case "all":
		_, err := h.client.AddPurchase(ctx, st.dealID, st.purchaseTitle, st.purchaseAmt, st.purchasePayerID, "all", nil, 0, nil)
		if err != nil {
			editText(ctx, h.api, chatID, msgID, "Ошибка при добавлении покупки.", nil)
			return
		}
		dealID := st.dealID
		title := st.purchaseTitle
		h.sm.Reset(tgID)
		editText(ctx, h.api, chatID, msgID, fmt.Sprintf("✅ Покупка «%s» добавлена!", title), nil)
		h.showPurchases(ctx, chatID, 0, dealID)

	case "all_share":
		st.step = stepAwaitPurchasePayerShare
		kb := backKeyboard()
		editText(ctx, h.api, chatID, msgID, "Введите вашу долю в рублях:", &kb)

	case "amounts":
		// Start collecting amounts from each participant
		deal, err := h.client.GetDeal(ctx, st.dealID)
		if err != nil {
			editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке сделки.", nil)
			return
		}
		participants, err := fetchUsers(ctx, h.client, deal.ParticipantIds)
		if err != nil || len(participants) == 0 {
			editText(ctx, h.api, chatID, msgID, "Нет участников в сделке.", nil)
			return
		}
		for _, p := range participants {
			st.participantNames[p.Id] = p.Name
			st.pendingAmountParticipants = append(st.pendingAmountParticipants, p.Id)
		}
		st.purchaseAmounts = make(map[string]int64)
		st.pendingAmountIdx = 0
		st.step = stepAwaitAmountsEntry
		firstID := st.pendingAmountParticipants[0]
		firstName := st.participantNames[firstID]
		kb := backKeyboard()
		editText(ctx, h.api, chatID, msgID, fmt.Sprintf("Сумма для %s (₽):", firstName), &kb)
	}
}

// Payments screen
func (h *Handler) handlePayments(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handlePayments")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "payments:")
	h.sm.Get(tgID).dealID = dealID
	h.showPayments(ctx, chatID, msgID, dealID)
}

// Start adding a payment: show "from" selector
func (h *Handler) handleAddPayment(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleAddPayment")
	defer span.End()

	dealID := strings.TrimPrefix(cb.Data, "add_payment:")
	st := h.sm.Get(tgID)
	st.dealID = dealID

	deal, err := h.client.GetDeal(ctx, dealID)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке сделки.", nil)
		return
	}
	participants, err := fetchUsers(ctx, h.client, deal.ParticipantIds)
	if err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при загрузке участников.", nil)
		return
	}
	for _, p := range participants {
		st.participantNames[p.Id] = p.Name
	}
	st.step = stepAwaitPaymentFrom
	h.showPaymentFromKeyboard(ctx, chatID, msgID, participants)
}

// Payment "from" selected
func (h *Handler) handlePaymentFrom(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handlePaymentFrom")
	defer span.End()

	fromID := strings.TrimPrefix(cb.Data, "payment_from:")
	st := h.sm.Get(tgID)
	st.pendingPaymentFromID = fromID
	st.step = stepAwaitPaymentTo

	// Build participants list from cached names, excluding fromID
	var participants []*pb.User
	for id, name := range st.participantNames {
		participants = append(participants, &pb.User{Id: id, Name: name})
	}
	h.showPaymentToKeyboard(ctx, chatID, msgID, participants, fromID)
}

// Payment "to" selected
func (h *Handler) handlePaymentTo(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handlePaymentTo")
	defer span.End()

	toID := strings.TrimPrefix(cb.Data, "payment_to:")
	st := h.sm.Get(tgID)
	st.purchasePayerID = toID // reuse field to store "to" user
	st.step = stepAwaitPaymentAmount
	kb := backKeyboard()
	editText(ctx, h.api, chatID, msgID, "Введите сумму платежа (₽):", &kb)
}

// Delete payment
func (h *Handler) handleDeletePayment(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDeletePayment")
	defer span.End()

	paymentID := strings.TrimPrefix(cb.Data, "del_payment:")
	st := h.sm.Get(tgID)
	if st.dealID == "" {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if err := h.client.RemovePayment(ctx, paymentID); err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при удалении платежа.", nil)
		return
	}
	h.showPayments(ctx, chatID, msgID, st.dealID)
}

func (h *Handler) handleDeleteParticipant(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDeleteParticipant")
	defer span.End()

	userID := strings.TrimPrefix(cb.Data, "del_participant:")
	st := h.sm.Get(tgID)
	if st.dealID == "" {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if _, err := h.client.RemoveDealParticipant(ctx, st.dealID, userID); err != nil {
		msg := grpcUserMessage(err, "Ошибка при удалении участника.")
		editText(ctx, h.api, chatID, msgID, msg, nil)
		return
	}
	h.showParticipants(ctx, chatID, msgID, st.dealID)
}

func (h *Handler) handleDeletePurchase(ctx context.Context, tgID, chatID int64, msgID int, cb *tgbotapi.CallbackQuery) {
	ctx, span := tracer.Start(ctx, "handleDeletePurchase")
	defer span.End()

	purchaseID := strings.TrimPrefix(cb.Data, "del_purchase:")
	st := h.sm.Get(tgID)
	if st.dealID == "" {
		editText(ctx, h.api, chatID, msgID, "Сессия устарела. Начните заново.", nil)
		return
	}
	if _, err := h.client.RemovePurchase(ctx, st.dealID, purchaseID); err != nil {
		editText(ctx, h.api, chatID, msgID, "Ошибка при удалении покупки.", nil)
		return
	}
	h.showPurchases(ctx, chatID, msgID, st.dealID)
}

func (h *Handler) handleBack(ctx context.Context, tgID, chatID int64, msgID int) {
	ctx, span := tracer.Start(ctx, "handleBack")
	defer span.End()

	st := h.sm.Get(tgID)
	dealID := st.dealID
	switch st.step {
	case stepAwaitDealTitle:
		h.navigateToMainMenu(ctx, tgID, chatID, msgID)
	case stepAwaitParticipantName, stepAwaitPurchaseTitle, stepAwaitPurchaseAmount,
		stepAwaitPurchasePayer, stepAwaitPurchaseSplitMode, stepAwaitPurchasePayerShare,
		stepAwaitAmountsEntry, stepAwaitPaymentFrom, stepAwaitPaymentTo, stepAwaitPaymentAmount:
		h.navigateToDeal(ctx, tgID, chatID, msgID, dealID)
	default:
		h.navigateToMainMenu(ctx, tgID, chatID, msgID)
	}
}
