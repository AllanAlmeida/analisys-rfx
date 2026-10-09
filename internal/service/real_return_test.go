package service

import (
	"context"
	"math"
	"testing"

	"investment-analyzer/internal/domain"
)

// Para CDB, LCI e LCA pos-fixados, req.Rate e percentual do CDI, e nao taxa
// anual. Tratar os dois como a mesma coisa fazia um CDB a 120% do CDI aparecer
// com 128% de retorno real. Nenhum teste cobria esse caminho.
func TestRealReturnConverteTaxaPercentualDoCDI(t *testing.T) {
	const cdi, ipca = 10.65, 4.50

	casos := []struct {
		nome     string
		req      domain.AnalyzeInvestmentRequest
		nominal  float64
		esperado float64
	}{
		{
			nome:    "CDB a 120% do CDI usa CDI*1.20 como nominal",
			req:     domain.AnalyzeInvestmentRequest{Type: domain.TypeCDB, Rate: 120, Index: domain.IndexCDI},
			nominal: cdi * 1.20,
			// (1 + 12.78/100) / (1 + 4.50/100) - 1 = 7.92%
			esperado: 7.92,
		},
		{
			nome:     "LCI pos-fixada a 97% do CDI usa CDI*0.97 como nominal",
			req:      domain.AnalyzeInvestmentRequest{Type: domain.TypeLCI, Rate: 97, Index: domain.IndexCDI},
			nominal:  cdi * 0.97,
			esperado: 5.58,
		},
		{
			nome: "LCI pre-fixada informa taxa anual direta, sem conversao",
			req: domain.AnalyzeInvestmentRequest{
				Type: domain.TypeLCI, Rate: 13.5, Index: domain.IndexPrefixado, Modality: domain.ModalityPRE,
			},
			nominal:  13.5,
			esperado: 8.61,
		},
		{
			nome:     "Tesouro Prefixado informa taxa anual direta",
			req:      domain.AnalyzeInvestmentRequest{Type: domain.TypeTesouroPrefixado, Rate: 14.8},
			nominal:  14.8,
			esperado: 9.86,
		},
	}

	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{SELIC: 10.75, IPCA: ipca, CDI: cdi},
	})

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtido := nominalRate(c.req, EconomyIndicators{SELIC: 10.75, IPCA: ipca, CDI: cdi})
			if math.Abs(obtido-c.nominal) > 1e-9 {
				t.Errorf("nominalRate: esperado %v, obtido %v", c.nominal, obtido)
			}

			resp, err := svc.Analyze(context.Background(), c.req)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if resp.RealReturn != c.esperado {
				t.Errorf("real_return: esperado %v, obtido %v", c.esperado, resp.RealReturn)
			}
		})
	}
}

// Tesouro IPCA+ e cotado em taxa real, acima da inflacao: o valor passa direto.
func TestRealReturnDoTesouroIPCAPassaDireto(t *testing.T) {
	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{SELIC: 10.75, IPCA: 4.50, CDI: 10.65},
	})

	resp, err := svc.Analyze(context.Background(), domain.AnalyzeInvestmentRequest{
		Type: domain.TypeTesouroIPCA, Rate: 5.2,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resp.RealReturn != 5.2 {
		t.Errorf("real_return: esperado 5.2, obtido %v", resp.RealReturn)
	}
}
