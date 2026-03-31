package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	itemv1 "github.com/sample-go/item-service/gen/pb/item/v1"
	lb "github.com/sample-go/item-service/internal/adapter/driven/liquibase"
	"github.com/sample-go/item-service/internal/adapter/driven/postgres"
	grpcadapter "github.com/sample-go/item-service/internal/adapter/driving/grpc"
	"github.com/sample-go/item-service/internal/config"
	"github.com/sample-go/item-service/internal/core/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// App Configurations
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to database")

	// Run migrations using pgxpool (Liquibase runner needs raw pool)
	if err := lb.Run(pool, cfg.LiquibaseChangelog, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Initialize GORM writer DB (primary)
	writerDB, err := gorm.Open(gormpostgres.Open(cfg.DatabaseWriterURL), &gorm.Config{})
	if err != nil {
		logger.Error("failed to initialize GORM writer", "error", err)
		os.Exit(1)
	}
	logger.Info("GORM writer initialized", "url", cfg.DatabaseWriterURL)

	// Initialize GORM reader DB (replica)
	readerDB, err := gorm.Open(gormpostgres.Open(cfg.DatabaseReaderURL), &gorm.Config{})
	if err != nil {
		logger.Error("failed to initialize GORM reader", "error", err)
		os.Exit(1)
	}
	logger.Info("GORM reader initialized", "url", cfg.DatabaseReaderURL)

	// Hexagonal wiring
	itemRepo := postgres.NewItemRepository(readerDB, writerDB, logger)
	itemSvc := service.New(itemRepo, logger)
	itemServer := grpcadapter.NewItemServer(itemSvc, logger)

	// gRPC server with recovery interceptor
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(recoveryInterceptor(logger)),
	)
	itemv1.RegisterItemServiceServer(grpcServer, itemServer)
	reflection.Register(grpcServer)

	addr := fmt.Sprintf(":%s", cfg.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "address", addr, "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down gRPC server...")
		grpcServer.GracefulStop()
		cancel()
	}()

	logger.Info("gRPC server listening", "address", addr)
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("gRPC server failed", "error", err)
		os.Exit(1)
	}
}

// recoveryInterceptor returns a gRPC unary interceptor that recovers from panics.
func recoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic recovered in gRPC handler",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
