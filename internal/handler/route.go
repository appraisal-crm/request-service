package handler

import (
	"github.com/MicahParks/keyfunc/v3"
	"github.com/appraisal-crm/request-service/internal/middleware"
	"github.com/appraisal-crm/request-service/internal/service"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(svc service.RequestService, jwks keyfunc.Keyfunc, allowedOrigins []string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", Health)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwks))

		rh := newRequestHandler(svc)

		r.With(middleware.RequireRoles("client")).Post("/requests", rh.Create)
		r.With(middleware.RequireRoles("client", "appraiser", "admin")).Get("/requests", rh.ListByClientID)
		r.With(middleware.RequireRoles("client", "appraiser", "admin")).Get("/requests/{id}", rh.GetByID)
		r.With(middleware.RequireRoles("appraiser", "admin")).Patch("/requests/{id}", rh.Update)
		r.With(middleware.RequireRoles("appraiser")).Patch("/requests/{id}/status", rh.ChangeStatus)
	})

	return r
}
