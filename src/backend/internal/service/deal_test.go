package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mralexandrov/debt-bot/backend/internal/domain"
)

// --- Fakes ---

type fakeDealRepo struct {
	deals        map[string]*domain.Deal
	participants map[string][]string // dealID → userIDs
	coverages    map[string][]domain.Coverage
}

func newFakeDealRepo() *fakeDealRepo {
	return &fakeDealRepo{
		deals:        make(map[string]*domain.Deal),
		participants: make(map[string][]string),
		coverages:    make(map[string][]domain.Coverage),
	}
}

func (r *fakeDealRepo) Create(ctx context.Context, title, createdBy string) (*domain.Deal, error) {
	d := &domain.Deal{ID: "deal-1", Title: title, CreatedBy: createdBy}
	r.deals[d.ID] = d
	return d, nil
}

func (r *fakeDealRepo) GetByID(ctx context.Context, id string) (*domain.Deal, error) {
	d, ok := r.deals[id]
	if !ok {
		return nil, errors.New("not found")
	}
	d.ParticipantIDs = r.participants[id]
	d.Coverages = r.coverages[id]
	return d, nil
}

func (r *fakeDealRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Deal, error) {
	return nil, nil
}

func (r *fakeDealRepo) AddParticipant(ctx context.Context, dealID, userID string) error {
	r.participants[dealID] = append(r.participants[dealID], userID)
	return nil
}

func (r *fakeDealRepo) RemoveParticipant(ctx context.Context, dealID, userID string) error {
	ids := r.participants[dealID]
	for i, id := range ids {
		if id == userID {
			r.participants[dealID] = append(ids[:i], ids[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *fakeDealRepo) GetParticipants(ctx context.Context, dealID string) ([]string, error) {
	return r.participants[dealID], nil
}

func (r *fakeDealRepo) SetCoverage(ctx context.Context, dealID, payerID, coveredID string) error {
	r.coverages[dealID] = append(r.coverages[dealID], domain.Coverage{PayerID: payerID, CoveredID: coveredID})
	return nil
}

func (r *fakeDealRepo) RemoveCoverage(ctx context.Context, dealID, coveredID string) error {
	covs := r.coverages[dealID]
	for i, c := range covs {
		if c.CoveredID == coveredID {
			r.coverages[dealID] = append(covs[:i], covs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *fakeDealRepo) GetCoverages(ctx context.Context, dealID string) ([]domain.Coverage, error) {
	return r.coverages[dealID], nil
}

type fakePurchaseRepo struct {
	purchases map[string][]*domain.Purchase // dealID → purchases
}

func newFakePurchaseRepo() *fakePurchaseRepo {
	return &fakePurchaseRepo{purchases: make(map[string][]*domain.Purchase)}
}

func (r *fakePurchaseRepo) Create(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, payerShare int64) (*domain.Purchase, error) {
	p := &domain.Purchase{ID: "p-" + title, DealID: dealID, Title: title, Amount: amount, PaidBy: paidBy, SplitMode: splitMode, PayerShare: payerShare}
	r.purchases[dealID] = append(r.purchases[dealID], p)
	return p, nil
}

func (r *fakePurchaseRepo) ListByDealID(ctx context.Context, dealID string) ([]*domain.Purchase, error) {
	return r.purchases[dealID], nil
}

func (r *fakePurchaseRepo) Delete(ctx context.Context, purchaseID string) error {
	for dealID, ps := range r.purchases {
		for i, p := range ps {
			if p.ID == purchaseID {
				r.purchases[dealID] = append(ps[:i], ps[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

func (r *fakePurchaseRepo) AddParticipant(ctx context.Context, purchaseID, userID string, amount int64) error {
	return nil
}

func (r *fakePurchaseRepo) GetParticipants(ctx context.Context, purchaseID string) ([]string, error) {
	return nil, nil
}

type fakePaymentRepo struct {
	payments []*domain.Payment
}

func (r *fakePaymentRepo) Create(ctx context.Context, dealID, fromUserID, toUserID string, amount int64) (*domain.Payment, error) {
	p := &domain.Payment{ID: "pay-1", DealID: dealID, FromUserID: fromUserID, ToUserID: toUserID, Amount: amount}
	r.payments = append(r.payments, p)
	return p, nil
}

func (r *fakePaymentRepo) ListByDealID(ctx context.Context, dealID string) ([]*domain.Payment, error) {
	return r.payments, nil
}

func (r *fakePaymentRepo) Delete(ctx context.Context, paymentID string) error {
	return nil
}

// --- DealService.Create tests ---

func TestDealServiceCreate_AddsCreatorAsParticipant(t *testing.T) {
	dealRepo := newFakeDealRepo()
	svc := NewDealService(dealRepo, newFakePurchaseRepo())

	deal, err := svc.Create(context.Background(), "Test Deal", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deal.ParticipantIDs) != 1 || deal.ParticipantIDs[0] != "user-1" {
		t.Errorf("expected creator to be a participant, got %v", deal.ParticipantIDs)
	}
}

// --- DealService.RemoveParticipant tests ---

func setupDealWithParticipants(t *testing.T) (*DealService, *fakeDealRepo, *fakePurchaseRepo) {
	t.Helper()
	dealRepo := newFakeDealRepo()
	purchaseRepo := newFakePurchaseRepo()
	svc := NewDealService(dealRepo, purchaseRepo)

	dealRepo.deals["deal-1"] = &domain.Deal{ID: "deal-1", Title: "Test", CreatedBy: "alice"}
	dealRepo.participants["deal-1"] = []string{"alice", "bob", "carol"}
	return svc, dealRepo, purchaseRepo
}

func TestRemoveParticipant_HappyPath(t *testing.T) {
	svc, dealRepo, _ := setupDealWithParticipants(t)

	_, err := svc.RemoveParticipant(context.Background(), "deal-1", "carol")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	remaining := dealRepo.participants["deal-1"]
	for _, id := range remaining {
		if id == "carol" {
			t.Error("carol should have been removed")
		}
	}
}

func TestRemoveParticipant_BlockedByPurchase(t *testing.T) {
	svc, _, purchaseRepo := setupDealWithParticipants(t)
	purchaseRepo.purchases["deal-1"] = []*domain.Purchase{
		{ID: "p-1", DealID: "deal-1", PaidBy: "bob", Amount: 1000, SplitMode: domain.SplitModeAll},
	}

	_, err := svc.RemoveParticipant(context.Background(), "deal-1", "bob")
	if !errors.Is(err, domain.ErrParticipantHasPurchases) {
		t.Errorf("expected ErrParticipantHasPurchases, got %v", err)
	}
}

func TestRemoveParticipant_BlockedByCoveragePayer(t *testing.T) {
	svc, dealRepo, _ := setupDealWithParticipants(t)
	// alice pays for carol
	dealRepo.coverages["deal-1"] = []domain.Coverage{{PayerID: "alice", CoveredID: "carol"}}

	_, err := svc.RemoveParticipant(context.Background(), "deal-1", "alice")
	if !errors.Is(err, domain.ErrParticipantIsCoveragePayer) {
		t.Errorf("expected ErrParticipantIsCoveragePayer, got %v", err)
	}
}

func TestRemoveParticipant_BlockedByCoveredPerson(t *testing.T) {
	svc, dealRepo, _ := setupDealWithParticipants(t)
	// alice pays for carol → carol is covered
	dealRepo.coverages["deal-1"] = []domain.Coverage{{PayerID: "alice", CoveredID: "carol"}}

	_, err := svc.RemoveParticipant(context.Background(), "deal-1", "carol")
	if !errors.Is(err, domain.ErrParticipantIsCovered) {
		t.Errorf("expected ErrParticipantIsCovered, got %v", err)
	}
}

func TestRemoveParticipant_AllowedWhenNoPurchasesOrCoverages(t *testing.T) {
	svc, _, _ := setupDealWithParticipants(t)
	// bob has no purchases, no coverages → deletion should succeed

	_, err := svc.RemoveParticipant(context.Background(), "deal-1", "bob")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// --- DealService.AddParticipant tests ---

func TestAddParticipant_AddsParticipant(t *testing.T) {
	dealRepo := newFakeDealRepo()
	svc := NewDealService(dealRepo, newFakePurchaseRepo())
	dealRepo.deals["deal-1"] = &domain.Deal{ID: "deal-1", Title: "T", CreatedBy: "alice"}

	deal, err := svc.AddParticipant(context.Background(), "deal-1", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deal.ParticipantIDs) != 1 || deal.ParticipantIDs[0] != "bob" {
		t.Errorf("expected [bob], got %v", deal.ParticipantIDs)
	}
}

// --- DealService.AddPurchase tests ---

func TestAddPurchase_SplitModeAll(t *testing.T) {
	svc := NewDealService(newFakeDealRepo(), newFakePurchaseRepo())

	p, err := svc.AddPurchase(context.Background(), "deal-1", "Dinner", 3000, "alice", domain.SplitModeAll, nil, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Title != "Dinner" || p.Amount != 3000 || p.PaidBy != "alice" {
		t.Errorf("unexpected purchase: %+v", p)
	}
	if len(p.ParticipantIDs) != 0 {
		t.Errorf("SplitModeAll should not set ParticipantIDs, got %v", p.ParticipantIDs)
	}
}

func TestAddPurchase_SplitModeCustom(t *testing.T) {
	svc := NewDealService(newFakeDealRepo(), newFakePurchaseRepo())

	participants := []string{"alice", "bob"}
	p, err := svc.AddPurchase(context.Background(), "deal-1", "Taxi", 600, "alice", domain.SplitModeCustom, participants, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.ParticipantIDs) != 2 {
		t.Errorf("expected 2 custom participants, got %v", p.ParticipantIDs)
	}
}

// --- DealService.SetCoverage / RemoveCoverage tests ---

func TestSetCoverage_StoresCoverage(t *testing.T) {
	dealRepo := newFakeDealRepo()
	svc := NewDealService(dealRepo, newFakePurchaseRepo())
	dealRepo.deals["deal-1"] = &domain.Deal{ID: "deal-1", Title: "T", CreatedBy: "alice"}

	_, err := svc.SetCoverage(context.Background(), "deal-1", "alice", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	covs := dealRepo.coverages["deal-1"]
	if len(covs) != 1 || covs[0].PayerID != "alice" || covs[0].CoveredID != "bob" {
		t.Errorf("unexpected coverages: %v", covs)
	}
}

func TestRemoveCoverage_DeletesCoverage(t *testing.T) {
	dealRepo := newFakeDealRepo()
	svc := NewDealService(dealRepo, newFakePurchaseRepo())
	dealRepo.deals["deal-1"] = &domain.Deal{ID: "deal-1", Title: "T", CreatedBy: "alice"}
	dealRepo.coverages["deal-1"] = []domain.Coverage{{PayerID: "alice", CoveredID: "bob"}}

	_, err := svc.RemoveCoverage(context.Background(), "deal-1", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dealRepo.coverages["deal-1"]) != 0 {
		t.Error("coverage should have been removed")
	}
}

// --- DealService.ListPurchases tests ---

func TestListPurchases_ReturnsPurchases(t *testing.T) {
	purchaseRepo := newFakePurchaseRepo()
	svc := NewDealService(newFakeDealRepo(), purchaseRepo)
	purchaseRepo.purchases["deal-1"] = []*domain.Purchase{
		{ID: "p-1", DealID: "deal-1", Title: "A", Amount: 100, PaidBy: "alice", SplitMode: domain.SplitModeAll},
		{ID: "p-2", DealID: "deal-1", Title: "B", Amount: 200, PaidBy: "bob", SplitMode: domain.SplitModeAll},
	}

	ps, err := svc.ListPurchases(context.Background(), "deal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ps) != 2 {
		t.Errorf("expected 2 purchases, got %d", len(ps))
	}
}

// --- DealService.GetByID / ListByUserID tests ---

func TestGetByID_ReturnsDeal(t *testing.T) {
	dealRepo := newFakeDealRepo()
	svc := NewDealService(dealRepo, newFakePurchaseRepo())
	dealRepo.deals["deal-1"] = &domain.Deal{ID: "deal-1", Title: "Hello", CreatedBy: "alice"}

	deal, err := svc.GetByID(context.Background(), "deal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deal.Title != "Hello" {
		t.Errorf("expected title 'Hello', got %q", deal.Title)
	}
}

func TestListByUserID_ReturnsDealList(t *testing.T) {
	dealRepo := newFakeDealRepo()
	svc := NewDealService(dealRepo, newFakePurchaseRepo())
	// fakeDealRepo.ListByUserID always returns nil, nil — just verify no error
	deals, err := svc.ListByUserID(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = deals
}

// --- DebtService.Calculate integration test ---

func TestDebtService_Calculate_SimpleCase(t *testing.T) {
	dealRepo := newFakeDealRepo()
	purchaseRepo := newFakePurchaseRepo()

	dealRepo.participants["deal-1"] = []string{"alice", "bob"}
	purchaseRepo.purchases["deal-1"] = []*domain.Purchase{
		{ID: "p-1", DealID: "deal-1", Title: "Lunch", Amount: 200, PaidBy: "alice", SplitMode: domain.SplitModeAll},
	}

	svc := NewDebtService(dealRepo, purchaseRepo, &fakePaymentRepo{})
	result, err := svc.Calculate(context.Background(), "deal-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// alice paid 200, split equally: each owes 100
	// alice net +100, bob net −100 → bob owes alice 100
	if len(result.Debts) != 1 {
		t.Fatalf("expected 1 debt, got %d: %v", len(result.Debts), result.Debts)
	}
	d := result.Debts[0]
	if d.FromUserID != "bob" || d.ToUserID != "alice" || d.Amount != 100 {
		t.Errorf("unexpected debt: %+v", d)
	}
}

// --- DealService.RemovePurchase tests ---

func TestRemovePurchase_RemovesPurchase(t *testing.T) {
	dealRepo := newFakeDealRepo()
	purchaseRepo := newFakePurchaseRepo()
	svc := NewDealService(dealRepo, purchaseRepo)

	dealRepo.deals["deal-1"] = &domain.Deal{ID: "deal-1", Title: "Test", CreatedBy: "alice"}
	purchaseRepo.purchases["deal-1"] = []*domain.Purchase{
		{ID: "p-1", DealID: "deal-1", Title: "Dinner", Amount: 1200, PaidBy: "alice", SplitMode: domain.SplitModeAll},
		{ID: "p-2", DealID: "deal-1", Title: "Taxi", Amount: 500, PaidBy: "bob", SplitMode: domain.SplitModeAll},
	}

	_, err := svc.RemovePurchase(context.Background(), "deal-1", "p-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	remaining := purchaseRepo.purchases["deal-1"]
	if len(remaining) != 1 || remaining[0].ID != "p-2" {
		t.Errorf("expected only p-2 to remain, got %v", remaining)
	}
}
