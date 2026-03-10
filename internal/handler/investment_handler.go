package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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

func (h *InvestmentHandler) AnalyzeBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req domain.AnalyzeBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if len(req.Items) == 0 {
		writeJSONError(w, http.StatusBadRequest, "field 'items' must contain at least one item")
		return
	}

	writeJSON(w, http.StatusOK, h.analyzeItems(r, req.Items))
}

type plainTextBatchPayload struct {
	Text string `json:"text"`
}

func (h *InvestmentHandler) AnalyzeBatchFromPlainText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rawText, err := extractRawPlainTextInput(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, parseErrors := service.ParsePlainTextBatch(rawText)
	if len(items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":        "no valid products found in plain text",
			"parse_errors": parseErrors,
		})
		return
	}

	batch := h.analyzeItems(r, items)
	writeJSON(w, http.StatusOK, map[string]any{
		"parsed":       len(items),
		"parse_failed": len(parseErrors),
		"parse_errors": parseErrors,
		"batch":        batch,
	})
}

func (h *InvestmentHandler) AnalyzeBatchFromPlainTextCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rawText, err := extractRawPlainTextInput(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	inputItems, parseErrors := service.ParsePlainTextBatch(rawText)
	batch := h.analyzeItems(r, inputItems)

	csvBytes, err := buildBatchCSV(batch, parseErrors)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to generate CSV output")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="analysis_batch.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csvBytes)
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
		errors.Is(err, domain.ErrInvalidModality) ||
		errors.Is(err, domain.ErrInvalidMaturityDate) ||
		errors.Is(err, domain.ErrUnsupportedType)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *InvestmentHandler) analyzeItems(r *http.Request, input []domain.AnalyzeInvestmentRequest) domain.AnalyzeBatchResponse {
	items := make([]domain.AnalyzeBatchItemResult, 0, len(input))
	okCount := 0
	failedCount := 0

	for i, item := range input {
		resp, err := h.analyzerService.Analyze(r.Context(), item)
		if err != nil {
			failedCount++
			items = append(items, domain.AnalyzeBatchItemResult{
				Index: i,
				Input: item,
				Error: err.Error(),
			})
			continue
		}

		okCount++
		items = append(items, domain.AnalyzeBatchItemResult{
			Index:  i,
			Input:  item,
			Result: &resp,
		})
	}

	return domain.AnalyzeBatchResponse{
		Total:  len(input),
		Ok:     okCount,
		Failed: failedCount,
		Items:  items,
	}
}

func extractRawPlainTextInput(r *http.Request) (string, error) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read request body")
	}

	rawText := strings.TrimSpace(string(rawBody))
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var payload plainTextBatchPayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return "", fmt.Errorf("invalid JSON request body")
		}
		rawText = strings.TrimSpace(payload.Text)
	}

	if rawText == "" {
		return "", fmt.Errorf("plain text input is empty")
	}

	return rawText, nil
}

func buildBatchCSV(batch domain.AnalyzeBatchResponse, parseErrors []string) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'

	header := []string{
		"index",
		"type",
		"issuer",
		"rate",
		"indexer",
		"modality",
		"maturity_date",
		"classification",
		"score",
		"equivalent_cdb",
		"equivalent_cdi_return",
		"real_return",
		"description",
		"error",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, item := range batch.Items {
		row := []string{
			strconv.Itoa(item.Index),
			item.Input.Type,
			item.Input.Issuer,
			formatFloat(item.Input.Rate),
			item.Input.Index,
			item.Input.Modality,
			item.Input.MaturityDate,
			"",
			"",
			"",
			"",
			"",
			"",
			item.Error,
		}

		if item.Result != nil {
			row[7] = item.Result.Classification
			row[8] = formatFloat(item.Result.Score)
			row[9] = formatFloat(item.Result.EquivalentCDB)
			row[10] = formatFloat(item.Result.EquivalentCDIReturn)
			row[11] = formatFloat(item.Result.RealReturn)
			row[12] = item.Result.Description
			row[13] = ""
		}

		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	if len(parseErrors) > 0 {
		_ = writer.Write([]string{})
		_ = writer.Write([]string{"parse_errors"})
		for _, parseErr := range parseErrors {
			_ = writer.Write([]string{parseErr})
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
