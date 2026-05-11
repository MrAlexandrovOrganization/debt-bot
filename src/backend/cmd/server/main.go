package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/mralexandrov/debt-bot/backend/gen/debt/v1"
	"github.com/mralexandrov/debt-bot/backend/internal/grpchandler"
	"github.com/mralexandrov/debt-bot/backend/internal/repository/postgres"
	"github.com/mralexandrov/debt-bot/backend/internal/schema"
	"github.com/mralexandrov/debt-bot/backend/internal/service"
	observability "github.com/mralexandrov/go-observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dsn := mustEnv("DATABASE_URL")
	port := envOr("GRPC_PORT", "50051")

	ctx := context.Background()

	logger := observability.NewLogger("backend")
	slog.SetDefault(logger)

	shutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    "backend",
		ServiceVersion: "0.1.0",
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		slog.ErrorContext(ctx, "setup observability", "error", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		slog.ErrorContext(ctx, "connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "ping database", "error", err)
		os.Exit(1)
	}

	if err := schema.Apply(ctx, db); err != nil {
		slog.ErrorContext(ctx, "apply schema", "error", err)
		os.Exit(1)
	}

	userRepo := postgres.NewUserRepository(db)
	dealRepo := postgres.NewDealRepository(db)
	purchaseRepo := postgres.NewPurchaseRepository(db)
	paymentRepo := postgres.NewPaymentRepository(db)

	userSvc := service.NewUserService(userRepo)
	dealSvc := service.NewDealService(dealRepo, purchaseRepo)
	debtSvc := service.NewDebtService(dealRepo, purchaseRepo, paymentRepo)
	paymentSvc := service.NewPaymentService(paymentRepo)

	handler := grpchandler.New(userSvc, dealSvc, debtSvc, paymentSvc)

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterDebtServiceServer(srv, handler)
	reflection.Register(srv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.ErrorContext(ctx, "listen", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "gRPC server listening", "port", port)
	if err := srv.Serve(lis); err != nil {
		slog.ErrorContext(ctx, "serve", "error", err)
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
