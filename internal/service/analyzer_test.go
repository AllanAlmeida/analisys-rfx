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
