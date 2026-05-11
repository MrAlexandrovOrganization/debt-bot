package repository

import (
	"context"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, name string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id, name string) (*domain.User, error)

	// Identity
	FindIdentity(ctx context.Context, platform, externalID string) (*domain.UserIdentity, error)
	CreateIdentity(ctx context.Context, userID, platform, externalID string) (*domain.UserIdentity, error)
}

type DealRepository interface {
	Create(ctx context.Context, title, createdBy string) (*domain.Deal, error)
	GetByID(ctx context.Context, id string) (*domain.Deal, error)
	ListByUserID(ctx context.Context, userID string) ([]*domain.Deal, error)
	AddParticipant(ctx context.Context, dealID, userID string) error
	RemoveParticipant(ctx context.Context, dealID, userID string) error
	GetParticipants(ctx context.Context, dealID string) ([]string, error)
	SetCoverage(ctx context.Context, dealID, payerID, coveredID string) error
	RemoveCoverage(ctx context.Context, dealID, coveredID string) error
	GetCoverages(ctx context.Context, dealID string) ([]domain.Coverage, error)
}

type PurchaseRepository interface {
	Create(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, payerShare int64) (*domain.Purchase, error)
	ListByDealID(ctx context.Context, dealID string) ([]*domain.Purchase, error)
	Delete(ctx context.Context, purchaseID string) error
	AddParticipant(ctx context.Context, purchaseID, userID string, amount int64) error
	GetParticipants(ctx context.Context, purchaseID string) ([]string, error)
}

type PaymentRepository interface {
	Create(ctx context.Context, dealID, fromUserID, toUserID string, amount int64) (*domain.Payment, error)
	ListByDealID(ctx context.Context, dealID string) ([]*domain.Payment, error)
	Delete(ctx context.Context, paymentID string) error
}
