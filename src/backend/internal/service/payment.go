package service

import (
	"context"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"github.com/mralexandrov/debt-bot/backend/internal/repository"
	"go.opentelemetry.io/otel/attribute"
)

type PaymentService struct {
	payments repository.PaymentRepository
}

func NewPaymentService(payments repository.PaymentRepository) *PaymentService {
	return &PaymentService{payments: payments}
}

func (s *PaymentService) Add(ctx context.Context, dealID, fromUserID, toUserID string, amount int64) (*domain.Payment, error) {
	ctx, span := tracer.Start(ctx, "PaymentService.Add")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.String("payment.from_user_id", fromUserID),
		attribute.String("payment.to_user_id", toUserID),
		attribute.Int64("payment.amount", amount),
	)
	return s.payments.Create(ctx, dealID, fromUserID, toUserID, amount)
}

func (s *PaymentService) ListByDealID(ctx context.Context, dealID string) ([]*domain.Payment, error) {
	ctx, span := tracer.Start(ctx, "PaymentService.ListByDealID")
	defer span.End()
	span.SetAttributes(attribute.String("deal.id", dealID))
	return s.payments.ListByDealID(ctx, dealID)
}

func (s *PaymentService) Delete(ctx context.Context, paymentID string) error {
	ctx, span := tracer.Start(ctx, "PaymentService.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("payment.id", paymentID))
	return s.payments.Delete(ctx, paymentID)
}
