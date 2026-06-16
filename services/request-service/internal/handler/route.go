package handler

import (
	"github.com/Meidorislav/appraisal-crm/services/request-service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(svc service.RequestService) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", Health)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/requests", func(r chi.Router) {
		rh := newRequestHandler(svc)
		r.Post("/", rh.Create)
		r.Get("/{id}", rh.GetByID)
		r.Patch("/{id}", rh.Update)
		r.Patch("/{id}/status", rh.ChangeStatus)
		r.Get("/", rh.ListByClientID)
	})

	return r
}
