package bot

import (
	"context"
	"fmt"

	pb "github.com/mralexandrov/debt-bot/frontend/telegram/gen/debt/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DebtClient is the interface the bot handler uses to communicate with the backend.
// The concrete *Client satisfies this interface.
type DebtClient interface {
	CreateUser(ctx context.Context, name string) (*pb.User, error)
	ResolveOrCreateUser(ctx context.Context, platform, externalID, name, username string) (*pb.User, bool, error)
	GetUser(ctx context.Context, userID string) (*pb.User, error)
	CreateDeal(ctx context.Context, title, createdBy string) (*pb.Deal, error)
	GetDeal(ctx context.Context, dealID string) (*pb.Deal, error)
	ListUserDeals(ctx context.Context, userID string) ([]*pb.Deal, error)
	AddDealParticipant(ctx context.Context, dealID, userID string) (*pb.Deal, error)
	SetDealCoverage(ctx context.Context, dealID, payerID, coveredID string) (*pb.Deal, error)
	RemoveDealCoverage(ctx context.Context, dealID, coveredID string) (*pb.Deal, error)
	AddPurchase(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, participantIDs []string, payerShare int64, participantAmounts map[string]int64) (*pb.Purchase, error)
	ListDealPurchases(ctx context.Context, dealID string) ([]*pb.Purchase, error)
	RemoveDealParticipant(ctx context.Context, dealID, userID string) (*pb.Deal, error)
	RemovePurchase(ctx context.Context, dealID, purchaseID string) (*pb.Deal, error)
	CalculateDebts(ctx context.Context, dealID string) (*pb.CalculateDebtsResponse, error)
	AddPayment(ctx context.Context, dealID, fromUserID, toUserID string, amount int64) (*pb.Payment, error)
	ListDealPayments(ctx context.Context, dealID string) ([]*pb.Payment, error)
	RemovePayment(ctx context.Context, paymentID string) error
}

type Client struct {
	conn pb.DebtServiceClient
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to backend: %w", err)
	}
	return &Client{conn: pb.NewDebtServiceClient(conn)}, nil
}

func (c *Client) CreateUser(ctx context.Context, name string) (*pb.User, error) {
	resp, err := c.conn.CreateUser(ctx, &pb.CreateUserRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}

func (c *Client) ResolveOrCreateUser(ctx context.Context, platform, externalID, name, username string) (*pb.User, bool, error) {
	resp, err := c.conn.ResolveOrCreateUser(ctx, &pb.ResolveOrCreateUserRequest{
		Platform:   platform,
		ExternalId: externalID,
		Name:       name,
		Username:   username,
	})
	if err != nil {
		return nil, false, err
	}
	return resp.User, resp.Created, nil
}

func (c *Client) GetUser(ctx context.Context, userID string) (*pb.User, error) {
	resp, err := c.conn.GetUser(ctx, &pb.GetUserRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}

func (c *Client) CreateDeal(ctx context.Context, title, createdBy string) (*pb.Deal, error) {
	resp, err := c.conn.CreateDeal(ctx, &pb.CreateDealRequest{
		Title:     title,
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) GetDeal(ctx context.Context, dealID string) (*pb.Deal, error) {
	resp, err := c.conn.GetDeal(ctx, &pb.GetDealRequest{DealId: dealID})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) ListUserDeals(ctx context.Context, userID string) ([]*pb.Deal, error) {
	resp, err := c.conn.ListUserDeals(ctx, &pb.ListUserDealsRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return resp.Deals, nil
}

func (c *Client) AddDealParticipant(ctx context.Context, dealID, userID string) (*pb.Deal, error) {
	resp, err := c.conn.AddDealParticipant(ctx, &pb.AddDealParticipantRequest{
		DealId: dealID,
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) SetDealCoverage(ctx context.Context, dealID, payerID, coveredID string) (*pb.Deal, error) {
	resp, err := c.conn.SetDealCoverage(ctx, &pb.SetDealCoverageRequest{
		DealId:    dealID,
		PayerId:   payerID,
		CoveredId: coveredID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) RemoveDealCoverage(ctx context.Context, dealID, coveredID string) (*pb.Deal, error) {
	resp, err := c.conn.RemoveDealCoverage(ctx, &pb.RemoveDealCoverageRequest{
		DealId:    dealID,
		CoveredId: coveredID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) AddPurchase(ctx context.Context, dealID, title string, amount int64, paidBy, splitMode string, participantIDs []string, payerShare int64, participantAmounts map[string]int64) (*pb.Purchase, error) {
	resp, err := c.conn.AddPurchase(ctx, &pb.AddPurchaseRequest{
		DealId:             dealID,
		Title:              title,
		Amount:             amount,
		PaidBy:             paidBy,
		SplitMode:          splitMode,
		ParticipantIds:     participantIDs,
		PayerShare:         payerShare,
		ParticipantAmounts: participantAmounts,
	})
	if err != nil {
		return nil, err
	}
	return resp.Purchase, nil
}

func (c *Client) ListDealPurchases(ctx context.Context, dealID string) ([]*pb.Purchase, error) {
	resp, err := c.conn.ListDealPurchases(ctx, &pb.ListDealPurchasesRequest{DealId: dealID})
	if err != nil {
		return nil, err
	}
	return resp.Purchases, nil
}

func (c *Client) RemoveDealParticipant(ctx context.Context, dealID, userID string) (*pb.Deal, error) {
	resp, err := c.conn.RemoveDealParticipant(ctx, &pb.RemoveDealParticipantRequest{
		DealId: dealID,
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) RemovePurchase(ctx context.Context, dealID, purchaseID string) (*pb.Deal, error) {
	resp, err := c.conn.RemovePurchase(ctx, &pb.RemovePurchaseRequest{
		DealId:     dealID,
		PurchaseId: purchaseID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Deal, nil
}

func (c *Client) CalculateDebts(ctx context.Context, dealID string) (*pb.CalculateDebtsResponse, error) {
	return c.conn.CalculateDebts(ctx, &pb.CalculateDebtsRequest{DealId: dealID})
}

func (c *Client) AddPayment(ctx context.Context, dealID, fromUserID, toUserID string, amount int64) (*pb.Payment, error) {
	resp, err := c.conn.AddPayment(ctx, &pb.AddPaymentRequest{
		DealId:     dealID,
		FromUserId: fromUserID,
		ToUserId:   toUserID,
		Amount:     amount,
	})
	if err != nil {
		return nil, err
	}
	return resp.Payment, nil
}

func (c *Client) ListDealPayments(ctx context.Context, dealID string) ([]*pb.Payment, error) {
	resp, err := c.conn.ListDealPayments(ctx, &pb.ListDealPaymentsRequest{DealId: dealID})
	if err != nil {
		return nil, err
	}
	return resp.Payments, nil
}

func (c *Client) RemovePayment(ctx context.Context, paymentID string) error {
	_, err := c.conn.RemovePayment(ctx, &pb.RemovePaymentRequest{PaymentId: paymentID})
	return err
}
