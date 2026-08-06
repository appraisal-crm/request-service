// @title           Request Service API
// @version         1.0
// @description     API for managing appraisal requests
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and your token

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	_ "github.com/appraisal-crm/request-service/api"
	"github.com/appraisal-crm/request-service/config"
	"github.com/appraisal-crm/request-service/internal/dedup"
	"github.com/appraisal-crm/request-service/internal/handler"
	"github.com/appraisal-crm/request-service/internal/kafka"
	"github.com/appraisal-crm/request-service/internal/outbox"
	"github.com/appraisal-crm/request-service/internal/repository"
	"github.com/appraisal-crm/request-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("database is not reachable", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis is not reachable — dedup might fail", "error", err)
	} else {
		slog.Info("connected to redis", "addr", cfg.RedisAddr)
	}
	defer rdb.Close()

	jwks, err := keyfunc.NewDefault([]string{cfg.JWKSUrl})
	if err != nil {
		slog.Error("failed to initialize JWKS", "error", err)
		os.Exit(1)
	}
	slog.Info("JWKS initialized", "url", cfg.JWKSUrl)

	repo := repository.NewPostgresRepository(db)
	svc := service.NewRequestService(repo)
	allowedOrigins := strings.Split(cfg.AllowedOrigins, ",")
	router := handler.NewRouter(svc, jwks, allowedOrigins)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	producer := outbox.NewProducer(brokers)
	defer producer.Close()
	relay := outbox.NewRelay(db, producer, cfg.OutboxPollInterval)
	relayDone := make(chan struct{})
	go func() {
		relay.Run(ctx)
		close(relayDone)
	}()
	slog.Info("outbox relay started", "interval", cfg.OutboxPollInterval)

	deduplicator := dedup.New(rdb, 24*time.Hour)

	inspectConsumer := kafka.NewConsumer(brokers, cfg.KafkaConsumerGroup, cfg.KafkaInspectTopic, deduplicator, svc)
	defer inspectConsumer.Close()
	inspectConsumerDone := make(chan struct{})
	go func() {
		if err := inspectConsumer.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("inspect consumer failed", "error", err)
		}
		close(inspectConsumerDone)
	}()
	slog.Info("inspect event consumer started", "topic", cfg.KafkaInspectTopic, "group", cfg.KafkaConsumerGroup)

	reviewConsumer := kafka.NewConsumer(brokers, cfg.KafkaConsumerGroup, cfg.KafkaReviewTopic, deduplicator, svc)
	defer reviewConsumer.Close()
	reviewConsumerDone := make(chan struct{})
	go func() {
		if err := reviewConsumer.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("review consumer failed", "error", err)
		}
		close(reviewConsumerDone)
	}()
	slog.Info("review event consumer started", "topic", cfg.KafkaReviewTopic, "group", cfg.KafkaConsumerGroup)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		slog.Error("server error", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
		<-relayDone
		slog.Info("outbox relay stopped")
		<-inspectConsumerDone
		slog.Info("inspect consumer stopped")
		<-reviewConsumerDone
		slog.Info("review consumer stopped")
		slog.Info("server stopped")
	}
}
