package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"

	"investment-analyzer/internal/handler"
	"investment-analyzer/internal/service"
)

type economiaFixa struct{}

func (economiaFixa) GetIndicators(_ context.Context) (service.EconomyIndicators, error) {
	return service.EconomyIndicators{SELIC: 10.75, IPCA: 4.50, CDI: 10.65}, nil
}

func novoRouter() http.Handler {
	return New(handler.NewInvestmentHandler(service.NewAnalyzerService(economiaFixa{})))
}

func TestRotasRegistradas(t *testing.T) {
	casos := []struct {
		metodo      string
		caminho     string
		corpo       string
		contentType string
		status      int
	}{
		{http.MethodGet, "/health", "", "", http.StatusOK},
		{http.MethodPost, "/analyze", `{"type":"CDB","rate":120,"index":"CDI"}`, "application/json", http.StatusOK},
		{http.MethodPost, "/analyze/batch", `{"items":[{"type":"CDB","rate":120,"index":"CDI"}]}`, "application/json", http.StatusOK},
		{http.MethodPost, "/analyze/batch/from/plaintxt", "CDB - Banco Teste\nPós-fixado\n15/03/2029\n110,00% do CDI", "text/plain", http.StatusOK},
		{http.MethodPost, "/analyze/batch/from/plaintxt/csv", "CDB - Banco Teste\nPós-fixado\n15/03/2029\n110,00% do CDI", "text/plain", http.StatusOK},
	}

	r := novoRouter()

	for _, c := range casos {
		t.Run(c.metodo+" "+c.caminho, func(t *testing.T) {
			req := httptest.NewRequest(c.metodo, c.caminho, strings.NewReader(c.corpo))
			if c.contentType != "" {
				req.Header.Set("Content-Type", c.contentType)
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != c.status {
				t.Errorf("esperado %d, obtido %d (corpo: %s)", c.status, rec.Code, rec.Body.String())
			}
		})
	}
}

// O chi responde 405 antes de chamar o handler, e e por isso que os checks
// manuais de `r.Method != http.MethodPost` que existiam nos handlers eram
// codigo inalcancavel.
func TestMetodoErradoDevolve405(t *testing.T) {
	r := novoRouter()

	casos := []struct{ metodo, caminho string }{
		{http.MethodGet, "/analyze"},
		{http.MethodGet, "/analyze/batch"},
		{http.MethodGet, "/analyze/batch/from/plaintxt"},
		{http.MethodGet, "/analyze/batch/from/plaintxt/csv"},
		{http.MethodPost, "/health"},
		{http.MethodDelete, "/analyze"},
	}

	for _, c := range casos {
		t.Run(c.metodo+" "+c.caminho, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(c.metodo, c.caminho, nil))

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("esperado 405, obtido %d", rec.Code)
			}
		})
	}
}

func TestRotaInexistenteDevolve404(t *testing.T) {
	rec := httptest.NewRecorder()
	novoRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nao-existe", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("esperado 404, obtido %d", rec.Code)
	}
}

// Regressao do middleware de IP: o RealIP, depreciado no chi 5.3.0, sobrescrevia
// r.RemoteAddr com o X-Forwarded-For recebido, de onde quer que viesse. O
// ClientIPFromRemoteAddr nao toca o RemoteAddr e expoe o IP do socket no
// contexto.
func TestCabecalhosDeIPNaoSobrescremRemoteAddr(t *testing.T) {
	var remoteAddrVisto, clientIPVisto string

	r := New(handler.NewInvestmentHandler(service.NewAnalyzerService(economiaFixa{})))

	// Envolve o router para inspecionar o que o handler enxerga.
	alvo := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.ServeHTTP(w, req)
	})

	espiao := func(w http.ResponseWriter, req *http.Request) {
		remoteAddrVisto = req.RemoteAddr
		clientIPVisto = middleware.GetClientIP(req.Context())
	}

	for _, cabecalho := range []string{"X-Forwarded-For", "X-Real-IP", "True-Client-IP"} {
		t.Run(cabecalho, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.RemoteAddr = "10.0.0.1:5555"
			req.Header.Set(cabecalho, "203.0.113.9")

			rec := httptest.NewRecorder()
			alvo.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("esperado 200, obtido %d", rec.Code)
			}
		})
	}

	// Verifica diretamente a cadeia de middleware, sem passar pelo router.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	middleware.ClientIPFromRemoteAddr(http.HandlerFunc(espiao)).ServeHTTP(httptest.NewRecorder(), req)

	if remoteAddrVisto != "10.0.0.1:5555" {
		t.Errorf("RemoteAddr nao deveria ser sobrescrito: obtido %q", remoteAddrVisto)
	}
	if clientIPVisto != "10.0.0.1" {
		t.Errorf("client IP deveria vir do socket: obtido %q", clientIPVisto)
	}
}
