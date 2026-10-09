package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// servidorBCB imita a API do Banco Central: responde por serie, contando as
// requisicoes para que os testes possam verificar o cache.
type servidorBCB struct {
	*httptest.Server
	chamadas atomic.Int64
}

func novoServidorBCB(t *testing.T, porSerie map[int]string) *servidorBCB {
	t.Helper()

	s := &servidorBCB{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.chamadas.Add(1)

		for serie, corpo := range porSerie {
			if strings.Contains(r.URL.Path, fmt.Sprintf(".%d/", serie)) {
				if corpo == "" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(corpo))
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(s.Close)

	return s
}

func servicoApontandoPara(srv *servidorBCB, ttl time.Duration) *BCBEconomyService {
	svc := NewBCBEconomyService(ttl)
	svc.baseURL = srv.URL + "/dados/serie/bcdata.sgs"
	return svc
}

// Respostas no formato real do SGS, com virgula como separador decimal.
var respostasValidas = map[int]string{
	serieSELICMetaAnual: `[{"data":"01/08/2026","valor":"13.75"}]`,
	serieIPCAAcum12m:    `[{"data":"01/08/2026","valor":"4,22"}]`,
	serieCDIDiario:      `[{"data":"01/09/2026","valor":"0,050788"}]`,
}

func TestGetIndicatorsLeAsTresSeries(t *testing.T) {
	srv := novoServidorBCB(t, respostasValidas)

	indicadores, err := servicoApontandoPara(srv, time.Hour).GetIndicators(context.Background())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// SELIC e IPCA ja vem anuais; so o CDI e diario e precisa ser anualizado.
	if indicadores.SELIC != 13.75 {
		t.Errorf("SELIC: esperado 13.75, obtido %v", indicadores.SELIC)
	}
	if indicadores.IPCA != 4.22 {
		t.Errorf("IPCA: esperado 4.22 (virgula decimal convertida), obtido %v", indicadores.IPCA)
	}

	esperadoCDI := annualizePeriodicRate(0.050788, businessDaysPerYear)
	if math.Abs(indicadores.CDI-esperadoCDI) > 1e-9 {
		t.Errorf("CDI: esperado %v, obtido %v", esperadoCDI, indicadores.CDI)
	}
	if indicadores.CDI < 12 || indicadores.CDI > 14 {
		t.Errorf("CDI anualizado fora da faixa plausivel: %v", indicadores.CDI)
	}

	if srv.chamadas.Load() != 3 {
		t.Errorf("esperado 3 requisicoes (uma por serie), obtido %d", srv.chamadas.Load())
	}
}

func TestGetIndicatorsUsaCache(t *testing.T) {
	srv := novoServidorBCB(t, respostasValidas)
	svc := servicoApontandoPara(srv, time.Hour)

	for i := 0; i < 5; i++ {
		if _, err := svc.GetIndicators(context.Background()); err != nil {
			t.Fatalf("chamada %d falhou: %v", i, err)
		}
	}

	if srv.chamadas.Load() != 3 {
		t.Errorf("cache deveria evitar novas requisicoes: esperado 3, obtido %d", srv.chamadas.Load())
	}
}

func TestGetIndicatorsRebuscaQuandoOCacheExpira(t *testing.T) {
	srv := novoServidorBCB(t, respostasValidas)
	svc := servicoApontandoPara(srv, time.Nanosecond)

	if _, err := svc.GetIndicators(context.Background()); err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := svc.GetIndicators(context.Background()); err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}

	if srv.chamadas.Load() != 6 {
		t.Errorf("com TTL expirado esperava 6 requisicoes, obtido %d", srv.chamadas.Load())
	}
}

func TestGetIndicatorsPropagaFalhaDeCadaSerie(t *testing.T) {
	casos := []struct {
		nome       string
		serie      int
		contemErro string
	}{
		{"SELIC", serieSELICMetaAnual, "failed to fetch SELIC"},
		{"IPCA", serieIPCAAcum12m, "failed to fetch IPCA"},
		{"CDI", serieCDIDiario, "failed to fetch CDI"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			respostas := map[int]string{}
			for serie, corpo := range respostasValidas {
				respostas[serie] = corpo
			}
			respostas[c.serie] = "" // faz o servidor responder 500

			_, err := servicoApontandoPara(novoServidorBCB(t, respostas), time.Hour).GetIndicators(context.Background())

			if err == nil {
				t.Fatal("esperado erro")
			}
			if !strings.Contains(err.Error(), c.contemErro) {
				t.Errorf("esperado erro contendo %q, obtido %v", c.contemErro, err)
			}
			if !strings.Contains(err.Error(), "unexpected status code 500") {
				t.Errorf("o erro deveria citar o status: %v", err)
			}
		})
	}
}

func TestFetchLatestValue(t *testing.T) {
	casos := []struct {
		nome       string
		resposta   string
		esperado   float64
		contemErro string
	}{
		{nome: "virgula decimal", resposta: `[{"data":"01/08/2026","valor":"4,22"}]`, esperado: 4.22},
		{nome: "ponto decimal", resposta: `[{"data":"01/08/2026","valor":"13.75"}]`, esperado: 13.75},
		{nome: "valor negativo", resposta: `[{"data":"01/08/2026","valor":"-0,32"}]`, esperado: -0.32},
		{nome: "mais de um registro usa o primeiro", resposta: `[{"valor":"1,5"},{"valor":"9,9"}]`, esperado: 1.5},
		{nome: "lista vazia", resposta: `[]`, contemErro: "empty payload"},
		{nome: "valor nao numerico", resposta: `[{"valor":"indisponivel"}]`, contemErro: "invalid syntax"},
		{nome: "JSON malformado", resposta: `{nao e json`, contemErro: "invalid character"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			srv := novoServidorBCB(t, map[int]string{serieSELICMetaAnual: c.resposta})

			valor, err := servicoApontandoPara(srv, time.Hour).fetchLatestValue(context.Background(), serieSELICMetaAnual)

			if c.contemErro != "" {
				if err == nil {
					t.Fatalf("esperado erro, obtido valor %v", valor)
				}
				if !strings.Contains(err.Error(), c.contemErro) {
					t.Errorf("esperado erro contendo %q, obtido %v", c.contemErro, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if valor != c.esperado {
				t.Errorf("esperado %v, obtido %v", c.esperado, valor)
			}
		})
	}
}

func TestFetchLatestValueComServidorInacessivel(t *testing.T) {
	srv := novoServidorBCB(t, respostasValidas)
	svc := servicoApontandoPara(srv, time.Hour)
	srv.Close()

	if _, err := svc.fetchLatestValue(context.Background(), serieSELICMetaAnual); err == nil {
		t.Fatal("esperado erro de conexao")
	}
}

func TestFetchLatestValueRespeitaOContexto(t *testing.T) {
	srv := novoServidorBCB(t, respostasValidas)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := servicoApontandoPara(srv, time.Hour).fetchLatestValue(ctx, serieSELICMetaAnual); err == nil {
		t.Fatal("esperado erro de contexto cancelado")
	}
}
