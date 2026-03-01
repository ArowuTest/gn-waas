package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/backend/meter-ingestor/internal/handler"
	"github.com/ArowuTest/gn-waas/backend/meter-ingestor/internal/repository"
	"github.com/ArowuTest/gn-waas/backend/meter-ingestor/internal/service"
	pb "github.com/ArowuTest/gn-waas/backend/meter-ingestor/proto/meter"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	env := getEnv("APP_ENV", "development")
	var logger *zap.Logger
	if env == "production" {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	logger.Info("GN-WAAS Meter Ingestor starting",
		zap.String("env", env),
		zap.String("grpc_port", getEnv("GRPC_PORT", "9090")),
		zap.String("http_port", getEnv("PORT", "8086")),
	)

	// ── Database ──────────────────────────────────────────────────────────────
	dsn := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s pool_max_conns=10",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "gnwaas"),
		getEnv("DB_USER", "gnwaas_app"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_SSLMODE", "disable"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	db, err := pgxpool.New(ctx, dsn)
	cancel()
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		logger.Fatal("Database ping failed", zap.Error(err))
	}
	logger.Info("Connected to GN-WAAS database")

	// ── NATS (optional) ───────────────────────────────────────────────────────
	var nc *natsgo.Conn
	natsURL := getEnv("NATS_URL", "")
	if natsURL != "" {
		nc, err = natsgo.Connect(natsURL,
			natsgo.Name("gnwaas-meter-ingestor"),
			natsgo.ReconnectWait(2*time.Second),
			natsgo.MaxReconnects(10),
			natsgo.DisconnectErrHandler(func(_ *natsgo.Conn, err error) {
				logger.Warn("NATS disconnected", zap.Error(err))
			}),
			natsgo.ReconnectHandler(func(nc *natsgo.Conn) {
				logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
			}),
		)
		if err != nil {
			logger.Warn("NATS connection failed — running without event publishing", zap.Error(err))
			nc = nil
		} else {
			defer nc.Close()
			logger.Info("Connected to NATS", zap.String("url", natsURL))
		}
	} else {
		logger.Info("NATS_URL not set — meter readings will not publish events")
	}

	// ── Wire dependencies ─────────────────────────────────────────────────────
	repo := repository.NewMeterReadingRepository(db, logger)
	svc := service.NewMeterIngestorService(repo, nc, logger)
	grpcHandler := handler.NewGRPCHandler(svc, repo, logger)
	httpHandler := handler.NewHTTPHandler(svc, repo, logger)

	// ── gRPC Server ───────────────────────────────────────────────────────────
	grpcPort := getEnv("GRPC_PORT", "9090")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port", zap.String("port", grpcPort), zap.Error(err))
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(4 * 1024 * 1024), // 4 MB max message
	)
	pb.RegisterMeterIngestorServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer) // Enable gRPC reflection for grpcurl/Postman

	go func() {
		logger.Info("gRPC server listening", zap.String("port", grpcPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	// ── HTTP/REST Server ──────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "GN-WAAS Meter Ingestor v1.0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	app.Get("/health", httpHandler.HealthCheck)

	api := app.Group("/api/v1")
	// Single reading (from field officer mobile app)
	api.Post("/readings", httpHandler.SubmitReading)
	// Batch upload (from AMR gateway HTTP fallback)
	api.Post("/readings/batch", httpHandler.SubmitBatch)
	// Query
	api.Get("/readings/:account_number/latest", httpHandler.GetLatestReading)

	httpPort := getEnv("PORT", "8086")
	go func() {
		logger.Info("HTTP server listening", zap.String("port", httpPort))
		if err := app.Listen(":" + httpPort); err != nil {
			logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down meter ingestor...")
	grpcServer.GracefulStop()
	if err := app.Shutdown(); err != nil {
		logger.Error("HTTP shutdown error", zap.Error(err))
	}
	logger.Info("Meter ingestor stopped")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
