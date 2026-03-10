package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"anlisys-rfx/internal/domain"
	"anlisys-rfx/internal/service"
)

type InvestmentHandler struct {
	analyzerService *service.AnalyzerService
}

func NewInvestmentHandler(analyzerService *service.AnalyzerService) *InvestmentHandler {
	return &InvestmentHandler{
		analyzerService: analyzerService,
	}
}

func (h *InvestmentHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req domain.AnalyzeInvestmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	resp, err := h.analyzerService.Analyze(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if isValidationError(err) {
			status = http.StatusBadRequest
		}
		writeJSONError(w, status, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *InvestmentHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, domain.ErrTypeRequired) ||
		errors.Is(err, domain.ErrRateInvalid) ||
		errors.Is(err, domain.ErrInvalidIndex) ||
		errors.Is(err, domain.ErrUnsupportedType)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
