package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const bcbBaseURL = "https://api.bcb.gov.br/dados/serie/bcdata.sgs"

type EconomyIndicators struct {
	SELIC float64
	IPCA  float64
	CDI   float64
}

const (
	monthlyPeriodsPerYear = 12
	businessDaysPerYear   = 252
)

type EconomyService interface {
	GetIndicators(ctx context.Context) (EconomyIndicators, error)
}

type BCBEconomyService struct {
	client   *http.Client
	cacheTTL time.Duration

	mu         sync.RWMutex
	cachedAt   time.Time
	indicators EconomyIndicators
}

func NewBCBEconomyService(cacheTTL time.Duration) *BCBEconomyService {
	return &BCBEconomyService{
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
		cacheTTL: cacheTTL,
	}
}

func (s *BCBEconomyService) GetIndicators(ctx context.Context) (EconomyIndicators, error) {
	s.mu.RLock()
	if time.Since(s.cachedAt) < s.cacheTTL {
		indicators := s.indicators
		s.mu.RUnlock()
		return indicators, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.cachedAt) < s.cacheTTL {
		return s.indicators, nil
	}

	selic, err := s.fetchLatestValue(ctx, 432)
	if err != nil {
		return EconomyIndicators{}, fmt.Errorf("failed to fetch SELIC: %w", err)
	}

	ipca, err := s.fetchLatestValue(ctx, 433)
	if err != nil {
		return EconomyIndicators{}, fmt.Errorf("failed to fetch IPCA: %w", err)
	}

	cdi, err := s.fetchLatestValue(ctx, 12)
	if err != nil {
		return EconomyIndicators{}, fmt.Errorf("failed to fetch CDI: %w", err)
	}

	s.indicators = EconomyIndicators{
		SELIC: normalizeSELIC(selic),
		IPCA:  annualizePeriodicRate(ipca, monthlyPeriodsPerYear),
		CDI:   annualizePeriodicRate(cdi, businessDaysPerYear),
	}
	s.cachedAt = time.Now()

	return s.indicators, nil
}

type bcbEntry struct {
	Data  string `json:"data"`
	Valor string `json:"valor"`
}

func (s *BCBEconomyService) fetchLatestValue(ctx context.Context, seriesCode int) (float64, error) {
	url := fmt.Sprintf("%s.%d/dados/ultimos/1?formato=json", bcbBaseURL, seriesCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var payload []bcbEntry
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}

	if len(payload) == 0 {
		return 0, fmt.Errorf("empty payload")
	}

	valueString := strings.ReplaceAll(payload[0].Valor, ",", ".")
	value, err := strconv.ParseFloat(valueString, 64)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func normalizeSELIC(value float64) float64 {
	return value
}

func annualizePeriodicRate(periodicRate float64, periodsPerYear float64) float64 {
	if periodicRate <= -100 {
		return periodicRate
	}

	factor := 1 + periodicRate/100
	return (math.Pow(factor, periodsPerYear) - 1) * 100
}
