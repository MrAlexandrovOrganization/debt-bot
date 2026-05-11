package service

import (
	"context"
	"sort"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
	"github.com/mralexandrov/debt-bot/backend/internal/repository"
	"go.opentelemetry.io/otel/attribute"
)

type DebtService struct {
	deals     repository.DealRepository
	purchases repository.PurchaseRepository
	payments  repository.PaymentRepository
}

func NewDebtService(deals repository.DealRepository, purchases repository.PurchaseRepository, payments repository.PaymentRepository) *DebtService {
	return &DebtService{deals: deals, purchases: purchases, payments: payments}
}

func (s *DebtService) Calculate(ctx context.Context, dealID string) (*domain.CalculationResult, error) {
	ctx, span := tracer.Start(ctx, "DebtService.Calculate")
	defer span.End()
	span.SetAttributes(attribute.String("deal.id", dealID))

	participants, err := s.deals.GetParticipants(ctx, dealID)
	if err != nil {
		return nil, err
	}

	purchases, err := s.purchases.ListByDealID(ctx, dealID)
	if err != nil {
		return nil, err
	}

	coverages, err := s.deals.GetCoverages(ctx, dealID)
	if err != nil {
		return nil, err
	}

	payments, err := s.payments.ListByDealID(ctx, dealID)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("participant.count", len(participants)),
		attribute.Int("purchase.count", len(purchases)),
		attribute.Int("coverage.count", len(coverages)),
		attribute.Int("payment.count", len(payments)),
	)

	balances, summaries := calculateBalances(purchases, participants, coverages, payments)
	debts := minimizeTransactions(balances)

	span.SetAttributes(attribute.Int("debt.count", len(debts)))

	return &domain.CalculationResult{
		Debts:     debts,
		Balances:  balances,
		Summaries: summaries,
	}, nil
}

// calculateBalances computes net balance for each participant across all purchases.
// Positive balance = owed money (creditor), negative = owes money (debtor).
func calculateBalances(purchases []*domain.Purchase, dealParticipants []string, coverages []domain.Coverage, payments []*domain.Payment) (map[string]int64, map[string]*domain.PaymentSummary) {
	balances := make(map[string]int64)
	summaries := make(map[string]*domain.PaymentSummary)
	for _, id := range dealParticipants {
		balances[id] = 0
		summaries[id] = &domain.PaymentSummary{}
	}

	// Build coverage map: covered → who covers their share
	covers := make(map[string]string, len(coverages))
	for _, cov := range coverages {
		covers[cov.CoveredID] = cov.PayerID
	}

	for _, p := range purchases {
		paidBySum := summaries[p.PaidBy]
		if paidBySum != nil {
			paidBySum.TotalPaid += p.Amount
		}

		switch p.SplitMode {
		case domain.SplitModeAmounts:
			// Each participant has an explicit amount
			for uid, amt := range p.ParticipantAmounts {
				if covererID, ok := covers[uid]; ok {
					balances[covererID] -= amt
					if s, ok := summaries[covererID]; ok {
						s.OwnShare += amt
					}
				} else {
					balances[uid] -= amt
					if s, ok := summaries[uid]; ok {
						s.OwnShare += amt
					}
				}
			}
			balances[p.PaidBy] += p.Amount

		default:
			// "all" or "custom" split
			var splitAmong []string
			if p.SplitMode == domain.SplitModeCustom {
				splitAmong = p.ParticipantIDs
			} else {
				splitAmong = dealParticipants
			}

			n := int64(len(splitAmong))
			if n == 0 {
				continue
			}

			// Charge each participant's share
			for _, uid := range splitAmong {
				var share int64
				if uid == p.PaidBy && p.PayerShare > 0 {
					share = p.PayerShare
				} else {
					if p.PayerShare > 0 {
						// remaining split equally among non-payer participants
						nonPayers := n - 1
						if nonPayers <= 0 {
							share = 0
						} else {
							share = (p.Amount - p.PayerShare) / nonPayers
						}
					} else {
						share = p.Amount / n
					}
				}

				if covererID, ok := covers[uid]; ok {
					balances[covererID] -= share
					if s, ok := summaries[covererID]; ok {
						s.OwnShare += share
					}
				} else {
					balances[uid] -= share
					if s, ok := summaries[uid]; ok {
						s.OwnShare += share
					}
				}
			}
			balances[p.PaidBy] += p.Amount
		}
	}

	// Compute ExpectedFromOthers for each participant (balance before payments)
	for _, s := range summaries {
		if s.TotalPaid > s.OwnShare {
			s.ExpectedFromOthers = s.TotalPaid - s.OwnShare
		}
	}

	// Apply payments to balances and track received payments
	for _, pay := range payments {
		balances[pay.FromUserID] += pay.Amount
		balances[pay.ToUserID] -= pay.Amount
		if s, ok := summaries[pay.ToUserID]; ok {
			s.PaymentsReceived += pay.Amount
		}
	}

	// Compute StillAwaiting
	for _, s := range summaries {
		s.StillAwaiting = s.ExpectedFromOthers - s.PaymentsReceived
		if s.StillAwaiting < 0 {
			s.StillAwaiting = 0
		}
	}

	return balances, summaries
}

type balanceEntry struct {
	userID  string
	balance int64
}

// minimizeTransactions uses a greedy algorithm to minimize the number of transactions.
func minimizeTransactions(balances map[string]int64) []domain.DebtItem {
	var entries []balanceEntry
	for uid, bal := range balances {
		if bal != 0 {
			entries = append(entries, balanceEntry{uid, bal})
		}
	}

	// Sort descending: creditors first, then debtors
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].balance > entries[j].balance
	})

	var debts []domain.DebtItem
	i, j := 0, len(entries)-1

	for i < j {
		creditor := &entries[i]
		debtor := &entries[j]

		if creditor.balance <= 0 || debtor.balance >= 0 {
			break
		}

		amount := min(creditor.balance, -debtor.balance)

		debts = append(debts, domain.DebtItem{
			FromUserID: debtor.userID,
			ToUserID:   creditor.userID,
			Amount:     amount,
		})

		creditor.balance -= amount
		debtor.balance += amount

		if creditor.balance == 0 {
			i++
		}
		if debtor.balance == 0 {
			j--
		}
	}

	return debts
}
