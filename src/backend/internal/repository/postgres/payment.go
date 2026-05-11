package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"go.opentelemetry.io/otel/attribute"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, dealID, fromUserID, toUserID string, amount int64) (*domain.Payment, error) {
	ctx, span := tracer.Start(ctx, "db.payments.Create")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.String("payment.from_user_id", fromUserID),
		attribute.String("payment.to_user_id", toUserID),
		attribute.Int64("payment.amount", amount),
	)

	var p domain.Payment
	err := r.db.QueryRow(ctx,
		`INSERT INTO payments (deal_id, from_user_id, to_user_id, amount)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, deal_id, from_user_id, to_user_id, amount, created_at`,
		dealID, fromUserID, toUserID, amount,
	).Scan(&p.ID, &p.DealID, &p.FromUserID, &p.ToUserID, &p.Amount, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	span.SetAttributes(attribute.String("payment.id", p.ID))
	return &p, nil
}

func (r *PaymentRepository) ListByDealID(ctx context.Context, dealID string) ([]*domain.Payment, error) {
	ctx, span := tracer.Start(ctx, "db.payments.ListByDealID")
	defer span.End()
	span.SetAttributes(attribute.String("deal.id", dealID))

	rows, err := r.db.Query(ctx,
		`SELECT id, deal_id, from_user_id, to_user_id, amount, created_at
		 FROM payments WHERE deal_id = $1 ORDER BY created_at`,
		dealID,
	)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		var p domain.Payment
		if err := rows.Scan(&p.ID, &p.DealID, &p.FromUserID, &p.ToUserID, &p.Amount, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("payment.count", len(payments)))
	return payments, nil
}

func (r *PaymentRepository) Delete(ctx context.Context, paymentID string) error {
	ctx, span := tracer.Start(ctx, "db.payments.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("payment.id", paymentID))

	_, err := r.db.Exec(ctx, `DELETE FROM payments WHERE id = $1`, paymentID)
	if err != nil {
		return fmt.Errorf("delete payment: %w", err)
	}
	return nil
}
