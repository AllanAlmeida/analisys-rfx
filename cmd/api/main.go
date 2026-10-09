package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"investment-analyzer/internal/handler"
	"investment-analyzer/internal/router"
	"investment-analyzer/internal/service"
)

// intFromEnv le um inteiro da variavel de ambiente, caindo no padrao quando
// ela esta ausente ou invalida.
func intFromEnv(nome string, padrao int64) int64 {
	bruto := os.Getenv(nome)
	if bruto == "" {
		return padrao
	}

	valor, err := strconv.ParseInt(bruto, 10, 64)
	if err != nil || valor <= 0 {
		log.Printf("%s invalido (%q), usando %d", nome, bruto, padrao)
		return padrao
	}

	return valor
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	economyService := service.NewBCBEconomyService(1 * time.Hour)
	analyzerService := service.NewAnalyzerService(economyService)
	investmentHandler := handler.NewInvestmentHandlerWithLimits(
		analyzerService,
		intFromEnv("MAX_BODY_BYTES", handler.DefaultMaxBodyBytes),
		int(intFromEnv("MAX_BATCH_ITEMS", handler.DefaultMaxBatchItems)),
	)
	r := router.New(investmentHandler)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("API listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			log.Printf("force close failed: %v", closeErr)
		}
	}
}
