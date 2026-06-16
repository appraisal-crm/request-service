// @title           Request Service API
// @version         1.0
// @description     API for managing appraisal requests
// @host            localhost:8080
// @BasePath        /

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	_ "github.com/Meidorislav/appraisal-crm/services/request-service/docs"
	"github.com/Meidorislav/appraisal-crm/services/request-service/config"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/handler"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/repository"
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("database is not reachable: %v", err)
	}

	repo := repository.NewPostgresRepository(db)
	svc := service.NewRequestService(repo)
	router := handler.NewRouter(svc)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("starting server on %s", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
