package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"investment-analyzer/internal/domain"
	"investment-analyzer/internal/service"
)

// economiaFixa substitui a API do Banco Central por indicadores fixos, para que
// os testes do handler sejam deterministicos e nao toquem a rede.
type economiaFixa struct {
	indicadores service.EconomyIndicators
	err         error
}

func (e economiaFixa) GetIndicators(_ context.Context) (service.EconomyIndicators, error) {
	return e.indicadores, e.err
}

var indicadoresPadrao = service.EconomyIndicators{SELIC: 10.75, IPCA: 4.50, CDI: 10.65}

func novoHandler() *InvestmentHandler {
	return NewInvestmentHandler(service.NewAnalyzerService(economiaFixa{indicadores: indicadoresPadrao}))
}

func novoHandlerComErro() *InvestmentHandler {
	return NewInvestmentHandler(service.NewAnalyzerService(economiaFixa{err: errors.New("bcb indisponivel")}))
}

func requisicao(metodo, caminho, corpo, contentType string) *http.Request {
	req := httptest.NewRequest(metodo, caminho, strings.NewReader(corpo))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func executa(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := executa(novoHandler().Health, requisicao(http.MethodGet, "/health", "", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: esperado application/json, obtido %q", ct)
	}

	var corpo map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("resposta nao e JSON: %v", err)
	}
	if corpo["status"] != "ok" {
		t.Errorf("status: esperado ok, obtido %q", corpo["status"])
	}
}

func TestAnalyze(t *testing.T) {
	casos := []struct {
		nome          string
		corpo         string
		handler       *InvestmentHandler
		status        int
		contemErro    string
		classificacao string
	}{
		{
			nome:          "CDB valido",
			corpo:         `{"type":"CDB","rate":120,"index":"CDI"}`,
			status:        http.StatusOK,
			classificacao: domain.ClassificationExceptional,
		},
		{
			nome:       "JSON malformado",
			corpo:      `{"type":`,
			status:     http.StatusBadRequest,
			contemErro: "invalid JSON request body",
		},
		{
			nome:       "items no lugar de objeto unico",
			corpo:      `{"data":"nao-e-request"}`,
			status:     http.StatusBadRequest,
			contemErro: domain.ErrTypeRequired.Error(),
		},
		{
			nome:       "taxa zero reprovada na validacao",
			corpo:      `{"type":"CDB","rate":0}`,
			status:     http.StatusBadRequest,
			contemErro: domain.ErrRateInvalid.Error(),
		},
		{
			nome:       "tipo nao suportado",
			corpo:      `{"type":"CRI","rate":100}`,
			status:     http.StatusBadRequest,
			contemErro: domain.ErrUnsupportedType.Error(),
		},
		{
			nome:       "falha ao buscar indicadores vira 500",
			corpo:      `{"type":"CDB","rate":120,"index":"CDI"}`,
			handler:    novoHandlerComErro(),
			status:     http.StatusInternalServerError,
			contemErro: "bcb indisponivel",
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h := c.handler
			if h == nil {
				h = novoHandler()
			}

			rec := executa(h.Analyze, requisicao(http.MethodPost, "/analyze", c.corpo, "application/json"))

			if rec.Code != c.status {
				t.Fatalf("esperado %d, obtido %d (corpo: %s)", c.status, rec.Code, rec.Body.String())
			}
			if c.contemErro != "" && !strings.Contains(rec.Body.String(), c.contemErro) {
				t.Errorf("esperado erro contendo %q, obtido %s", c.contemErro, rec.Body.String())
			}
			if c.classificacao != "" {
				var resp domain.AnalyzeInvestmentResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("resposta nao e JSON: %v", err)
				}
				if resp.Classification != c.classificacao {
					t.Errorf("classificacao: esperado %q, obtido %q", c.classificacao, resp.Classification)
				}
			}
		})
	}
}

func TestAnalyzeBatch(t *testing.T) {
	t.Run("mistura itens validos e invalidos", func(t *testing.T) {
		corpo := `{"items":[
			{"type":"CDB","rate":120,"index":"CDI"},
			{"type":"CRI","rate":100},
			{"type":"LCI","rate":97,"index":"CDI"}
		]}`

		rec := executa(novoHandler().AnalyzeBatch, requisicao(http.MethodPost, "/analyze/batch", corpo, "application/json"))

		if rec.Code != http.StatusOK {
			t.Fatalf("esperado 200, obtido %d", rec.Code)
		}

		var resp domain.AnalyzeBatchResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("resposta nao e JSON: %v", err)
		}
		if resp.Total != 3 || resp.Ok != 2 || resp.Failed != 1 {
			t.Errorf("esperado total=3 ok=2 failed=1, obtido total=%d ok=%d failed=%d", resp.Total, resp.Ok, resp.Failed)
		}
		if len(resp.Items) != 3 {
			t.Fatalf("esperado 3 itens, obtido %d", len(resp.Items))
		}
		// O item que falhou preserva o indice e a entrada original.
		if resp.Items[1].Index != 1 || resp.Items[1].Error == "" || resp.Items[1].Result != nil {
			t.Errorf("item 1 deveria ter erro e nenhum resultado: %+v", resp.Items[1])
		}
		if resp.Items[0].Result == nil {
			t.Errorf("item 0 deveria ter resultado")
		}
	})

	casos := []struct {
		nome       string
		corpo      string
		contemErro string
	}{
		{"JSON malformado", `{"items":`, "invalid JSON request body"},
		{"lista vazia", `{"items":[]}`, "at least one item"},
		{"sem o campo items", `{}`, "at least one item"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := executa(novoHandler().AnalyzeBatch, requisicao(http.MethodPost, "/analyze/batch", c.corpo, "application/json"))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("esperado 400, obtido %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), c.contemErro) {
				t.Errorf("esperado erro contendo %q, obtido %s", c.contemErro, rec.Body.String())
			}
		})
	}

	t.Run("acima do teto de itens", func(t *testing.T) {
		h := NewInvestmentHandlerWithLimits(service.NewAnalyzerService(economiaFixa{indicadores: indicadoresPadrao}), DefaultMaxBodyBytes, 2)

		corpo := `{"items":[{"type":"CDB","rate":110},{"type":"CDB","rate":110},{"type":"CDB","rate":110}]}`
		rec := executa(h.AnalyzeBatch, requisicao(http.MethodPost, "/analyze/batch", corpo, "application/json"))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("esperado 400, obtido %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "at most 2 items") {
			t.Errorf("esperado erro de teto, obtido %s", rec.Body.String())
		}
	})
}

const textoPlanoValido = `CDB - Banco Teste
Pós-fixado
15/03/2029
110,00% do CDI`

func TestAnalyzeBatchFromPlainText(t *testing.T) {
	t.Run("corpo text/plain", func(t *testing.T) {
		rec := executa(novoHandler().AnalyzeBatchFromPlainText,
			requisicao(http.MethodPost, "/analyze/batch/from/plaintxt", textoPlanoValido, "text/plain"))

		if rec.Code != http.StatusOK {
			t.Fatalf("esperado 200, obtido %d (corpo: %s)", rec.Code, rec.Body.String())
		}

		var resp struct {
			Parsed      int                         `json:"parsed"`
			ParseFailed int                         `json:"parse_failed"`
			Batch       domain.AnalyzeBatchResponse `json:"batch"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("resposta nao e JSON: %v", err)
		}
		if resp.Parsed != 1 || resp.Batch.Ok != 1 {
			t.Errorf("esperado parsed=1 ok=1, obtido parsed=%d ok=%d", resp.Parsed, resp.Batch.Ok)
		}
	})

	t.Run("corpo JSON com campo text", func(t *testing.T) {
		corpo, err := json.Marshal(map[string]string{"text": textoPlanoValido})
		if err != nil {
			t.Fatalf("falha ao montar corpo: %v", err)
		}

		rec := executa(novoHandler().AnalyzeBatchFromPlainText,
			requisicao(http.MethodPost, "/analyze/batch/from/plaintxt", string(corpo), "application/json"))

		if rec.Code != http.StatusOK {
			t.Fatalf("esperado 200, obtido %d (corpo: %s)", rec.Code, rec.Body.String())
		}
	})

	casos := []struct {
		nome        string
		corpo       string
		contentType string
		contemErro  string
	}{
		{"corpo vazio", "", "text/plain", "plain text input is empty"},
		{"so espacos", "   \n  ", "text/plain", "plain text input is empty"},
		{"JSON malformado", `{"text":`, "application/json", "invalid JSON request body"},
		{"JSON com text vazio", `{"text":"  "}`, "application/json", "plain text input is empty"},
		{"texto sem produto reconhecivel", "linha qualquer\noutra linha", "text/plain", "no valid products found"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := executa(novoHandler().AnalyzeBatchFromPlainText,
				requisicao(http.MethodPost, "/analyze/batch/from/plaintxt", c.corpo, c.contentType))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("esperado 400, obtido %d (corpo: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.contemErro) {
				t.Errorf("esperado erro contendo %q, obtido %s", c.contemErro, rec.Body.String())
			}
		})
	}
}

func TestAnalyzeBatchFromPlainTextCSV(t *testing.T) {
	t.Run("gera CSV com cabecalho e uma linha por item", func(t *testing.T) {
		rec := executa(novoHandler().AnalyzeBatchFromPlainTextCSV,
			requisicao(http.MethodPost, "/analyze/batch/from/plaintxt/csv", textoPlanoValido, "text/plain"))

		if rec.Code != http.StatusOK {
			t.Fatalf("esperado 200, obtido %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
			t.Errorf("Content-Type: obtido %q", ct)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
			t.Errorf("Content-Disposition: obtido %q", cd)
		}

		leitor := csv.NewReader(strings.NewReader(rec.Body.String()))
		leitor.Comma = ';'
		leitor.FieldsPerRecord = -1

		linhas, err := leitor.ReadAll()
		if err != nil {
			t.Fatalf("CSV invalido: %v", err)
		}
		if len(linhas) != 2 {
			t.Fatalf("esperado cabecalho + 1 linha, obtido %d linhas", len(linhas))
		}
		if linhas[0][0] != "index" || linhas[0][2] != "issuer" {
			t.Errorf("cabecalho inesperado: %v", linhas[0])
		}
		if linhas[1][1] != "CDB" || linhas[1][2] != "Banco Teste" {
			t.Errorf("linha inesperada: %v", linhas[1])
		}
	})

	t.Run("neutraliza formula no campo do emissor", func(t *testing.T) {
		malicioso := strings.Replace(textoPlanoValido, "Banco Teste", `=cmd|'/c calc'!A1`, 1)

		rec := executa(novoHandler().AnalyzeBatchFromPlainTextCSV,
			requisicao(http.MethodPost, "/analyze/batch/from/plaintxt/csv", malicioso, "text/plain"))

		if rec.Code != http.StatusOK {
			t.Fatalf("esperado 200, obtido %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `'=cmd`) {
			t.Errorf("formula deveria vir prefixada com apostrofo, obtido: %s", rec.Body.String())
		}
	})

	t.Run("texto sem produto devolve 400 em vez de CSV vazio", func(t *testing.T) {
		rec := executa(novoHandler().AnalyzeBatchFromPlainTextCSV,
			requisicao(http.MethodPost, "/analyze/batch/from/plaintxt/csv", "linha qualquer", "text/plain"))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("esperado 400, obtido %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "no valid products found") {
			t.Errorf("corpo inesperado: %s", rec.Body.String())
		}
	})
}

func TestLimiteDeCorpo(t *testing.T) {
	h := NewInvestmentHandlerWithLimits(service.NewAnalyzerService(economiaFixa{indicadores: indicadoresPadrao}), 64, DefaultMaxBatchItems)

	grande := strings.Repeat("x", 4096)

	rec := executa(h.AnalyzeBatchFromPlainText,
		requisicao(http.MethodPost, "/analyze/batch/from/plaintxt", grande, "text/plain"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("corpo acima do limite deveria dar 400, obtido %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "failed to read request body") {
		t.Errorf("corpo inesperado: %s", rec.Body.String())
	}
}

func TestNewInvestmentHandlerWithLimitsCaiNoPadrao(t *testing.T) {
	svc := service.NewAnalyzerService(economiaFixa{indicadores: indicadoresPadrao})

	for _, c := range []struct {
		corpo int64
		itens int
	}{{0, 0}, {-1, -1}} {
		h := NewInvestmentHandlerWithLimits(svc, c.corpo, c.itens)

		if h.maxBodyBytes != DefaultMaxBodyBytes || h.maxBatchItems != DefaultMaxBatchItems {
			t.Errorf("limites invalidos deveriam cair no padrao, obtido corpo=%d itens=%d", h.maxBodyBytes, h.maxBatchItems)
		}
	}
}

func TestNeutralizarFormula(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"", ""},
		{"Banco Teste", "Banco Teste"},
		{"CDB", "CDB"},
		{"=1+1", "'=1+1"},
		{"+55 11 99999", "'+55 11 99999"},
		{"-100", "'-100"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tvalor", "'\tvalor"},
		{"\rvalor", "'\rvalor"},
		{"a=1", "a=1"},
	}

	for _, c := range casos {
		if obtido := neutralizarFormula(c.entrada); obtido != c.esperado {
			t.Errorf("neutralizarFormula(%q): esperado %q, obtido %q", c.entrada, c.esperado, obtido)
		}
	}
}

func TestIsValidationError(t *testing.T) {
	validacao := []error{
		domain.ErrTypeRequired, domain.ErrRateInvalid, domain.ErrInvalidIndex,
		domain.ErrInvalidModality, domain.ErrInvalidMaturityDate, domain.ErrUnsupportedType,
	}

	for _, err := range validacao {
		if !isValidationError(err) {
			t.Errorf("%v deveria ser erro de validacao", err)
		}
		// Precisa funcionar tambem quando embrulhado.
		if !isValidationError(fmt.Errorf("contexto: %w", err)) {
			t.Errorf("%v embrulhado deveria ser erro de validacao", err)
		}
	}

	if isValidationError(nil) {
		t.Error("nil nao e erro de validacao")
	}
	if isValidationError(errors.New("falha de rede")) {
		t.Error("erro arbitrario nao e erro de validacao")
	}
}
