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
	// ClientIPFromRemoteAddr em vez de RealIP: o RealIP foi depreciado no chi
	// 5.3.0 por ser vulneravel a IP spoofing — ele sobrescreve r.RemoteAddr com
	// o valor de X-Forwarded-For, True-Client-IP ou X-Real-IP, venham de onde
	// vierem. Este servico nao fica atras de um proxy conhecido, entao o IP
	// confiavel e o do proprio socket. Leia-o com middleware.GetClientIP(ctx).
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/health", investmentHandler.Health)
	r.Post("/analyze", investmentHandler.Analyze)
	r.Post("/analyze/batch", investmentHandler.AnalyzeBatch)
	r.Post("/analyze/batch/from/plaintxt", investmentHandler.AnalyzeBatchFromPlainText)
	r.Post("/analyze/batch/from/plaintxt/csv", investmentHandler.AnalyzeBatchFromPlainTextCSV)

	return r
}
