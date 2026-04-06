package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/sample-go/item-service/config"
	lb "github.com/sample-go/item-service/config/liquibase"
	"github.com/sample-go/item-service/config/session"
	itemv1 "github.com/sample-go/item-service/gen/pb/item/v1"
	fdsPostgres "github.com/sample-go/item-service/internal/fds/adapter/postgres"
	fdsService "github.com/sample-go/item-service/internal/fds/core/service"
	grpcadapter "github.com/sample-go/item-service/internal/items/adapter/grpc"
	"github.com/sample-go/item-service/internal/items/adapter/postgres"
	"github.com/sample-go/item-service/internal/items/core/service"
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

	fdsRepo := fdsPostgres.NewPlatformFDSIdentifierMappingRepository(readerDB, writerDB, logger)
	fdsSvc := fdsService.NewPlatformFDSIdentifierMapService(fdsRepo, logger)
	_ = fdsSvc // available for injection into handlers that need FDS identity resolution

	// gRPC server with session and recovery interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			session.UnaryInterceptor(logger, cfg.FDSIssuer),
			recoveryInterceptor(logger),
		),
	)

	itemv1.RegisterItemServiceServer(grpcServer, itemServer)
	reflection.Register(grpcServer)

	grpcAddr := fmt.Sprintf(":%s", cfg.GRPCPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("failed to listen", "address", grpcAddr, "error", err)
		os.Exit(1)
	}

	// Start gRPC server in background
	go func() {
		logger.Info("gRPC server listening", "address", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	// HTTP REST gateway (grpc-gateway reverse proxy)
	gwMux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(session.GatewayHeaderMatcher),
	)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := itemv1.RegisterItemServiceHandlerFromEndpoint(ctx, gwMux, grpcAddr, opts); err != nil {
		logger.Error("failed to register gateway", "error", err)
		os.Exit(1)
	}

	httpAddr := fmt.Sprintf(":%s", cfg.HTTPPort)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: gwMux,
	}

	// Start HTTP server in background
	go func() {
		logger.Info("HTTP gateway listening", "address", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP gateway failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutting down servers...")

	grpcServer.GracefulStop()
	if err := httpServer.Shutdown(context.Background()); err != nil {
		logger.Error("HTTP gateway shutdown error", "error", err)
	}
	cancel()
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
