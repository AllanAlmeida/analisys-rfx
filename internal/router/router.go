package router

import (
	"net/http"
	"time"

	"anlisys-rfx/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func New(investmentHandler *handler.InvestmentHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/health", investmentHandler.Health)
	r.Post("/analyze", investmentHandler.Analyze)

	return r
}
