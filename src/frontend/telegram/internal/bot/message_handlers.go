package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.opentelemetry.io/otel/attribute"
)

// --- Message handler (text input) ---

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	ctx, span := tracer.Start(ctx, "handleMessage")
	defer span.End()

	tgID := msg.From.ID

	if msg.IsCommand() && msg.Command() == "start" {
		h.sm.Reset(tgID)
		user := h.resolveUser(ctx, msg.From)
		greeting := "Привет!"
		if user != nil {
			greeting = "Привет, " + user.Name + "!"
		}
		h.showMainMenu(ctx, msg.Chat.ID, 0, greeting+"\n\nЯ помогу рассчитать долги после совместных трат.")
		return
	}

	st := h.sm.Get(tgID)
	text := strings.TrimSpace(msg.Text)

	if st.step != stepIdle {
		span.SetAttributes(attribute.String("fsm.step", st.step))
	}

	switch st.step {
	case stepAwaitDealTitle:
		if text == "" {
			send(ctx, h.api, msg.Chat.ID, "Название не может быть пустым. Попробуйте ещё раз:", nil)
			return
		}
		user := h.resolveUser(ctx, msg.From)
		if user == nil {
			return
		}
		deal, err := h.client.CreateDeal(ctx, text, user.Id)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при создании сделки.", nil)
			return
		}
		h.sm.Reset(tgID)
		send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("✅ Сделка «%s» создана!", deal.Title), nil)
		h.showDealMenu(ctx, msg.Chat.ID, 0, deal.Id)

	case stepAwaitParticipantName:
		dealID := st.dealID
		newUser, notice, err := h.resolveParticipant(ctx, msg)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении участника.", nil)
			return
		}
		if newUser == nil {
			return
		}
		if _, err := h.client.AddDealParticipant(ctx, dealID, newUser.Id); err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении в сделку.", nil)
			return
		}
		h.sm.Reset(tgID)
		send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("✅ %s добавлен.%s", newUser.Name, notice), nil)
		h.showParticipants(ctx, msg.Chat.ID, 0, dealID)

	case stepAwaitPurchaseTitle:
		if text == "" {
			send(ctx, h.api, msg.Chat.ID, "Название не может быть пустым. Попробуйте ещё раз:", nil)
			return
		}
		st.purchaseTitle = text
		st.step = stepAwaitPurchaseAmount
		kb := backKeyboard()
		send(ctx, h.api, msg.Chat.ID, "Введите сумму в рублях (например: 150 или 99.50 или 99,50):", &kb)

	case stepAwaitPurchaseAmount:
		amt, err := parseAmount(text)
		if err != nil || amt <= 0 {
			send(ctx, h.api, msg.Chat.ID, "Неверный формат. Введите сумму (например: 150 или 99.50):", nil)
			return
		}
		st.purchaseAmt = amt

		deal, err := h.client.GetDeal(ctx, st.dealID)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при загрузке сделки.", nil)
			return
		}
		participants, err := fetchUsers(ctx, h.client, deal.ParticipantIds)
		if err != nil || len(participants) == 0 {
			send(ctx, h.api, msg.Chat.ID, "Нет участников в сделке.", nil)
			return
		}
		for _, p := range participants {
			st.participantNames[p.Id] = p.Name
		}
		st.step = stepAwaitPurchasePayer
		h.showPayerKeyboard(ctx, msg.Chat.ID, 0, participants)

	case stepAwaitPurchasePayer:
		send(ctx, h.api, msg.Chat.ID, "Выберите плательщика из кнопок выше.", nil)

	case stepAwaitPurchaseSplitMode:
		send(ctx, h.api, msg.Chat.ID, "Выберите способ разделения из кнопок выше.", nil)

	case stepAwaitPurchasePayerShare:
		amt, err := parseAmount(text)
		if err != nil || amt <= 0 {
			send(ctx, h.api, msg.Chat.ID, "Неверный формат. Введите сумму (например: 150 или 99.50):", nil)
			return
		}
		st.purchasePayerShare = amt
		_, err = h.client.AddPurchase(ctx, st.dealID, st.purchaseTitle, st.purchaseAmt, st.purchasePayerID, "all", nil, amt, nil)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении покупки.", nil)
			return
		}
		dealID := st.dealID
		title := st.purchaseTitle
		h.sm.Reset(tgID)
		send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("✅ Покупка «%s» добавлена!", title), nil)
		h.showPurchases(ctx, msg.Chat.ID, 0, dealID)

	case stepAwaitAmountsEntry:
		amt, err := parseAmount(text)
		if err != nil || amt < 0 {
			send(ctx, h.api, msg.Chat.ID, "Неверный формат. Введите сумму (например: 150 или 99.50):", nil)
			return
		}
		participantID := st.pendingAmountParticipants[st.pendingAmountIdx]
		st.purchaseAmounts[participantID] = amt
		st.pendingAmountIdx++

		if st.pendingAmountIdx < len(st.pendingAmountParticipants) {
			nextID := st.pendingAmountParticipants[st.pendingAmountIdx]
			nextName := st.participantNames[nextID]
			kb := backKeyboard()
			send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("Сумма для %s (₽):", nextName), &kb)
			return
		}

		// All amounts collected — create purchase
		_, err = h.client.AddPurchase(ctx, st.dealID, st.purchaseTitle, st.purchaseAmt, st.purchasePayerID, "amounts", nil, 0, st.purchaseAmounts)
		if err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении покупки.", nil)
			return
		}
		dealID := st.dealID
		title := st.purchaseTitle
		h.sm.Reset(tgID)
		send(ctx, h.api, msg.Chat.ID, fmt.Sprintf("✅ Покупка «%s» добавлена!", title), nil)
		h.showPurchases(ctx, msg.Chat.ID, 0, dealID)

	case stepAwaitPaymentAmount:
		amt, err := parseAmount(text)
		if err != nil || amt <= 0 {
			send(ctx, h.api, msg.Chat.ID, "Неверный формат. Введите сумму (например: 150 или 99.50):", nil)
			return
		}
		dealID := st.dealID
		fromID := st.pendingPaymentFromID
		toID := st.purchasePayerID
		if _, err := h.client.AddPayment(ctx, dealID, fromID, toID, amt); err != nil {
			send(ctx, h.api, msg.Chat.ID, "Ошибка при добавлении платежа.", nil)
			return
		}
		h.sm.Reset(tgID)
		h.sm.Get(tgID).dealID = dealID
		send(ctx, h.api, msg.Chat.ID, "✅ Платёж записан!", nil)
		h.showPayments(ctx, msg.Chat.ID, 0, dealID)

	case stepDealCovSelectPayer, stepDealCovSelectCovered:
		send(ctx, h.api, msg.Chat.ID, "Используйте кнопки для навигации.", nil)

	case stepAwaitPaymentFrom, stepAwaitPaymentTo:
		send(ctx, h.api, msg.Chat.ID, "Используйте кнопки для выбора участника.", nil)
	}
}
