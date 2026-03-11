package service

import (
	"context"
	"testing"

	"anlisys-rfx/internal/domain"
)

type economyServiceMock struct {
	indicators EconomyIndicators
	err        error
}

func (m economyServiceMock) GetIndicators(_ context.Context) (EconomyIndicators, error) {
	return m.indicators, m.err
}

func TestAnalyzeCDBExceptional(t *testing.T) {
	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{
			SELIC: 10.75,
			IPCA:  4.50,
			CDI:   10.65,
		},
	})

	resp, err := svc.Analyze(context.Background(), domain.AnalyzeInvestmentRequest{
		Type:  "CDB",
		Rate:  120,
		Index: "CDI",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Classification != domain.ClassificationExceptional {
		t.Fatalf("expected classification %q, got %q", domain.ClassificationExceptional, resp.Classification)
	}
	if resp.EquivalentCDB != 120 {
		t.Fatalf("expected equivalent CDB 120, got %v", resp.EquivalentCDB)
	}
	if resp.EquivalentCDIReturn != 120 {
		t.Fatalf("expected equivalent CDI 120, got %v", resp.EquivalentCDIReturn)
	}
	if resp.Score < 9.5 || resp.Score > 10 {
		t.Fatalf("expected score between 9.5 and 10, got %v", resp.Score)
	}
}

func TestAnalyzeLCIEquivalentCDB(t *testing.T) {
	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{
			SELIC: 10.75,
			IPCA:  4.50,
			CDI:   10.65,
		},
	})

	resp, err := svc.Analyze(context.Background(), domain.AnalyzeInvestmentRequest{
		Type:  "LCI",
		Rate:  95,
		Index: "CDI",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Classification != domain.ClassificationGood {
		t.Fatalf("expected classification %q, got %q", domain.ClassificationGood, resp.Classification)
	}
	if resp.EquivalentCDB != 111.76 {
		t.Fatalf("expected equivalent CDB 111.76, got %v", resp.EquivalentCDB)
	}
	if resp.EquivalentCDIReturn != 95 {
		t.Fatalf("expected equivalent CDI 95, got %v", resp.EquivalentCDIReturn)
	}
}

func TestClassifyBoundaryRules(t *testing.T) {
	tests := []struct {
		name      string
		req       domain.AnalyzeInvestmentRequest
		eqCDI     float64
		wantClass string
	}{
		{"CDB exceptional", domain.AnalyzeInvestmentRequest{Type: domain.TypeCDB, Rate: 120}, 120, domain.ClassificationExceptional},
		{"CDB good", domain.AnalyzeInvestmentRequest{Type: domain.TypeCDB, Rate: 105}, 105, domain.ClassificationGood},
		{"CDB acceptable", domain.AnalyzeInvestmentRequest{Type: domain.TypeCDB, Rate: 100}, 100, domain.ClassificationAcceptable},
		{"CDB weak", domain.AnalyzeInvestmentRequest{Type: domain.TypeCDB, Rate: 99.9}, 99.9, domain.ClassificationWeak},
		{"LCI exceptional", domain.AnalyzeInvestmentRequest{Type: domain.TypeLCI, Index: domain.IndexCDI}, 100, domain.ClassificationExceptional},
		{"LCI good", domain.AnalyzeInvestmentRequest{Type: domain.TypeLCI, Index: domain.IndexCDI}, 90, domain.ClassificationGood},
		{"LCI acceptable", domain.AnalyzeInvestmentRequest{Type: domain.TypeLCI, Index: domain.IndexCDI}, 85, domain.ClassificationAcceptable},
		{"Tesouro Selic good", domain.AnalyzeInvestmentRequest{Type: domain.TypeTesouroSelic, Rate: 0.09, Index: domain.IndexSELIC}, 0, domain.ClassificationGood},
		{"Prefixado good", domain.AnalyzeInvestmentRequest{Type: domain.TypeTesouroPrefixado, Rate: 14.5}, 0, domain.ClassificationGood},
		{"IPCA acceptable", domain.AnalyzeInvestmentRequest{Type: domain.TypeTesouroIPCA, Rate: 5.1}, 0, domain.ClassificationAcceptable},
		{"LCI pre good by cdi equivalent", domain.AnalyzeInvestmentRequest{Type: domain.TypeLCI, Index: domain.IndexPrefixado}, 92, domain.ClassificationGood},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := classify(tc.req, tc.eqCDI)
			if got != tc.wantClass {
				t.Fatalf("expected %q, got %q", tc.wantClass, got)
			}
		})
	}
}

func TestAnalyzeLCIPrefixadoUsesCDIEquivalent(t *testing.T) {
	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{
			SELIC: 10.75,
			IPCA:  4.50,
			CDI:   10.00,
		},
	})

	resp, err := svc.Analyze(context.Background(), domain.AnalyzeInvestmentRequest{
		Type:     "LCI",
		Rate:     11.00,
		Index:    "PREFIXADO",
		Modality: "PRE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.EquivalentCDIReturn != 110 {
		t.Fatalf("expected equivalent CDI 110, got %v", resp.EquivalentCDIReturn)
	}
	if resp.Classification != domain.ClassificationExceptional {
		t.Fatalf("expected classification %q, got %q", domain.ClassificationExceptional, resp.Classification)
	}
}

func TestAnalyzeLCAPrefixadoWithAnnualizedIndicators(t *testing.T) {
	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{
			SELIC: 15.00,
			IPCA:  annualizePeriodicRate(0.33, monthlyPeriodsPerYear),
			CDI:   annualizePeriodicRate(0.06, businessDaysPerYear),
		},
	})

	resp, err := svc.Analyze(context.Background(), domain.AnalyzeInvestmentRequest{
		Type:         domain.TypeLCA,
		Rate:         12.11,
		Index:        domain.IndexPrefixado,
		Modality:     domain.ModalityPRE,
		MaturityDate: "2029-03-12",
		Issuer:       "Banco Original",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Classification != domain.ClassificationWeak {
		t.Fatalf("expected classification %q, got %q", domain.ClassificationWeak, resp.Classification)
	}
	if resp.Description != "LCA pre-fixado equivalente abaixo de 85% do CDI" {
		t.Fatalf("unexpected description: %q", resp.Description)
	}
	if resp.EquivalentCDIReturn != 74.21 {
		t.Fatalf("expected equivalent CDI 74.21, got %v", resp.EquivalentCDIReturn)
	}
	if resp.EquivalentCDB != 87.31 {
		t.Fatalf("expected equivalent CDB 87.31, got %v", resp.EquivalentCDB)
	}
	if resp.RealReturn != 7.76 {
		t.Fatalf("expected real return 7.76, got %v", resp.RealReturn)
	}
	if resp.Score != 4.5 {
		t.Fatalf("expected score 4.5, got %v", resp.Score)
	}
}

func TestAnalyzeTesouroSelic(t *testing.T) {
	svc := NewAnalyzerService(economyServiceMock{
		indicators: EconomyIndicators{
			SELIC: 15.00,
			IPCA:  annualizePeriodicRate(0.33, monthlyPeriodsPerYear),
			CDI:   annualizePeriodicRate(0.06, businessDaysPerYear),
		},
	})

	resp, err := svc.Analyze(context.Background(), domain.AnalyzeInvestmentRequest{
		Type:         domain.TypeTesouroSelic,
		Rate:         0.09,
		Index:        domain.IndexSELIC,
		Modality:     domain.ModalityPOS,
		MaturityDate: "2031-03-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Classification != domain.ClassificationGood {
		t.Fatalf("expected classification %q, got %q", domain.ClassificationGood, resp.Classification)
	}
	if resp.Description != "Tesouro Selic com spread entre 0.05% e 0.14% a.a." {
		t.Fatalf("unexpected description: %q", resp.Description)
	}
	if resp.EquivalentCDIReturn != 92.48 {
		t.Fatalf("expected equivalent CDI 92.48, got %v", resp.EquivalentCDIReturn)
	}
	if resp.EquivalentCDB != 92.48 {
		t.Fatalf("expected equivalent CDB 92.48, got %v", resp.EquivalentCDB)
	}
	if resp.RealReturn != 10.63 {
		t.Fatalf("expected real return 10.63, got %v", resp.RealReturn)
	}
	if resp.Score != 8.3 {
		t.Fatalf("expected score 8.3, got %v", resp.Score)
	}
}
