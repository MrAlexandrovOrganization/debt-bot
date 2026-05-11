package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"go.opentelemetry.io/otel/attribute"
)

type PurchaseRepository struct {
	db *pgxpool.Pool
}

func NewPurchaseRepository(db *pgxpool.Pool) *PurchaseRepository {
	return &PurchaseRepository{db: db}
}

func (r *PurchaseRepository) Create(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, payerShare int64) (*domain.Purchase, error) {
	ctx, span := tracer.Start(ctx, "db.purchases.Create")
	defer span.End()
	span.SetAttributes(
		attribute.String("deal.id", dealID),
		attribute.Int64("purchase.amount", amount),
		attribute.String("purchase.split_mode", splitMode),
		attribute.String("purchase.paid_by", paidBy),
	)

	var p domain.Purchase
	var ps *int64
	if payerShare > 0 {
		ps = &payerShare
	}
	err := r.db.QueryRow(ctx,
		`INSERT INTO purchases (deal_id, title, amount, paid_by, split_mode, payer_share) VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, deal_id, title, amount, paid_by, split_mode, payer_share`,
		dealID, title, amount, paidBy, splitMode, ps,
	).Scan(&p.ID, &p.DealID, &p.Title, &p.Amount, &p.PaidBy, &p.SplitMode, &ps)
	if err != nil {
		return nil, fmt.Errorf("create purchase: %w", err)
	}
	if ps != nil {
		p.PayerShare = *ps
	}
	span.SetAttributes(attribute.String("purchase.id", p.ID))
	return &p, nil
}

func (r *PurchaseRepository) ListByDealID(ctx context.Context, dealID string) ([]*domain.Purchase, error) {
	ctx, span := tracer.Start(ctx, "db.purchases.ListByDealID")
	defer span.End()
	span.SetAttributes(attribute.String("deal.id", dealID))

	rows, err := r.db.Query(ctx,
		`SELECT id, deal_id, title, amount, paid_by, split_mode, payer_share FROM purchases WHERE deal_id = $1`,
		dealID,
	)
	if err != nil {
		return nil, fmt.Errorf("list purchases: %w", err)
	}
	defer rows.Close()

	var purchases []*domain.Purchase
	for rows.Next() {
		var p domain.Purchase
		var ps *int64
		if err := rows.Scan(&p.ID, &p.DealID, &p.Title, &p.Amount, &p.PaidBy, &p.SplitMode, &ps); err != nil {
			return nil, fmt.Errorf("scan purchase: %w", err)
		}
		if ps != nil {
			p.PayerShare = *ps
		}
		purchases = append(purchases, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load participants for custom split and amounts split purchases
	for _, p := range purchases {
		if p.SplitMode == domain.SplitModeCustom || p.SplitMode == domain.SplitModeAmounts {
			participants, amounts, err := r.getParticipantsWithAmounts(ctx, p.ID)
			if err != nil {
				return nil, err
			}
			p.ParticipantIDs = participants
			if p.SplitMode == domain.SplitModeAmounts {
				p.ParticipantAmounts = amounts
			}
		}
	}
	span.SetAttributes(attribute.Int("purchase.count", len(purchases)))
	return purchases, nil
}

func (r *PurchaseRepository) Delete(ctx context.Context, purchaseID string) error {
	ctx, span := tracer.Start(ctx, "db.purchases.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("purchase.id", purchaseID))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM purchase_participants WHERE purchase_id = $1`, purchaseID); err != nil {
		return fmt.Errorf("delete purchase participants: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM purchases WHERE id = $1`, purchaseID); err != nil {
		return fmt.Errorf("delete purchase: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *PurchaseRepository) AddParticipant(ctx context.Context, purchaseID, userID string, amount int64) error {
	ctx, span := tracer.Start(ctx, "db.purchase_participants.AddParticipant")
	defer span.End()
	span.SetAttributes(
		attribute.String("purchase.id", purchaseID),
		attribute.String("user.id", userID),
	)

	var amt *int64
	if amount > 0 {
		amt = &amount
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO purchase_participants (purchase_id, user_id, amount) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		purchaseID, userID, amt,
	)
	if err != nil {
		return fmt.Errorf("add purchase participant: %w", err)
	}
	return nil
}

func (r *PurchaseRepository) GetParticipants(ctx context.Context, purchaseID string) ([]string, error) {
	ctx, span := tracer.Start(ctx, "db.purchase_participants.GetParticipants")
	defer span.End()
	span.SetAttributes(attribute.String("purchase.id", purchaseID))

	rows, err := r.db.Query(ctx,
		`SELECT user_id FROM purchase_participants WHERE purchase_id = $1`,
		purchaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("get purchase participants: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *PurchaseRepository) getParticipantsWithAmounts(ctx context.Context, purchaseID string) ([]string, map[string]int64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT user_id, amount FROM purchase_participants WHERE purchase_id = $1`,
		purchaseID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get purchase participants: %w", err)
	}
	defer rows.Close()

	var ids []string
	amounts := make(map[string]int64)
	for rows.Next() {
		var id string
		var amt *int64
		if err := rows.Scan(&id, &amt); err != nil {
			return nil, nil, fmt.Errorf("scan participant: %w", err)
		}
		ids = append(ids, id)
		if amt != nil {
			amounts[id] = *amt
		}
	}
	return ids, amounts, rows.Err()
}
