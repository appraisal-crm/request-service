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
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/MicahParks/keyfunc/v3"
	_ "github.com/Meidorislav/appraisal-crm/services/request-service/docs"
	"github.com/Meidorislav/appraisal-crm/services/request-service/config"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/handler"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/repository"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
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

	jwks, err := keyfunc.NewDefault([]string{cfg.JWKSUrl})
	if err != nil {
		slog.Error("failed to initialize JWKS", "error", err)
		os.Exit(1)
	}
	slog.Info("JWKS initialized", "url", cfg.JWKSUrl)

	repo := repository.NewPostgresRepository(db)
	svc := service.NewRequestService(repo)
	router := handler.NewRouter(svc, jwks)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	slog.Info("starting server", "addr", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
