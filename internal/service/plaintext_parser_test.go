package service

import (
	"testing"

	"anlisys-rfx/internal/domain"
)

func assertParsedItem(t *testing.T, item domain.AnalyzeInvestmentRequest, expectedType string, expectedRate float64, expectedIndex string, expectedModality string, expectedIssuer string, expectedMaturity string) {
	t.Helper()

	if item.Type != expectedType {
		t.Fatalf("expected type %q, got %q", expectedType, item.Type)
	}
	if item.Rate != expectedRate {
		t.Fatalf("expected rate %v, got %v", expectedRate, item.Rate)
	}
	if item.Index != expectedIndex {
		t.Fatalf("expected index %q, got %q", expectedIndex, item.Index)
	}
	if item.Modality != expectedModality {
		t.Fatalf("expected modality %q, got %q", expectedModality, item.Modality)
	}
	if item.Issuer != expectedIssuer {
		t.Fatalf("expected issuer %q, got %q", expectedIssuer, item.Issuer)
	}
	if item.MaturityDate != expectedMaturity {
		t.Fatalf("expected maturity %q, got %q", expectedMaturity, item.MaturityDate)
	}
}

func TestParsePlainTextBatch(t *testing.T) {
	input := `Produto e risco
Vencimento
Taxa
Taxa Eq. do CDB
Aplicação mínima
Estoque
LCI - Banco BTG Pactual
Pré-Fixado
Conservador
Juros no vencimento
10/09/2026
Prazo: 184 dias
11,77% a.a.	14,81% a.a.	R$ 1.000,00	-
Investir
LCA - Banco BTG Pactual
Pré-Fixado
Conservador
Juros no vencimento
11/09/2028
Prazo: 916 dias
11,31% a.a.	13,12% a.a.	R$ 1.000,00	-
Investir`

	items, parseErrors := ParsePlainTextBatch(input)
	if len(parseErrors) != 0 {
		t.Fatalf("expected no parse errors, got %v", parseErrors)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	assertParsedItem(t, items[0], domain.TypeLCI, 11.77, domain.IndexPrefixado, domain.ModalityPRE, "Banco BTG Pactual", "2026-09-10")
	assertParsedItem(t, items[1], domain.TypeLCA, 11.31, domain.IndexPrefixado, domain.ModalityPRE, "Banco BTG Pactual", "2028-09-11")
}

func TestParsePlainTextBatchWithInvalidBlock(t *testing.T) {
	input := `LCI - Banco BTG Pactual
Pré-Fixado
Conservador
Juros no vencimento
10/09/2026
Prazo: 184 dias
Investir`

	items, parseErrors := ParsePlainTextBatch(input)
	if len(items) != 0 {
		t.Fatalf("expected 0 valid items, got %d", len(items))
	}
	if len(parseErrors) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(parseErrors))
	}
}

func TestParsePlainTextBatchWithCDIRates(t *testing.T) {
	input := `LCI - Banco BTG Pactual
Pós-Fixado
Conservador
Juros no vencimento
11/09/2026
Prazo: 184 dias
88,00% do CDI	110,10% do CDI	R$ 1.000,00	-
Investir
LCA - Banco BTG Pactual
Pós-Fixado
Conservador
Juros no vencimento
12/03/2029
Prazo: 1097 dias
88,00% do CDI	101,47% do CDI	R$ 1.000,00	-
Investir`

	items, parseErrors := ParsePlainTextBatch(input)
	if len(parseErrors) != 0 {
		t.Fatalf("expected no parse errors, got %v", parseErrors)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	assertParsedItem(t, items[0], domain.TypeLCI, 88, domain.IndexCDI, domain.ModalityPOS, "Banco BTG Pactual", "2026-09-11")
	assertParsedItem(t, items[1], domain.TypeLCA, 88, domain.IndexCDI, domain.ModalityPOS, "Banco BTG Pactual", "2029-03-12")
}

func TestParsePlainTextBatchPrioritizesDisplayedProductRate(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedType     string
		expectedRate     float64
		expectedIndex    string
		expectedModality string
		expectedMaturity string
	}{
		{
			name: "pre fixed uses annual rate before equivalent cdb rate",
			input: `LCI - Banco BTG Pactual
Pré-Fixado
Conservador
Juros no vencimento
10/09/2026
Prazo: 184 dias
11,77% a.a.	14,81% a.a.	R$ 1.000,00	-
Investir`,
			expectedType:     domain.TypeLCI,
			expectedRate:     11.77,
			expectedIndex:    domain.IndexPrefixado,
			expectedModality: domain.ModalityPRE,
			expectedMaturity: "2026-09-10",
		},
		{
			name: "post fixed uses cdi rate before equivalent cdb rate",
			input: `LCA - Banco BTG Pactual
Pós-Fixado
Conservador
Juros no vencimento
11/09/2028
Prazo: 915 dias
90,00% do CDI	104,19% do CDI	R$ 1.000,00	-
Investir`,
			expectedType:     domain.TypeLCA,
			expectedRate:     90,
			expectedIndex:    domain.IndexCDI,
			expectedModality: domain.ModalityPOS,
			expectedMaturity: "2028-09-11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, parseErrors := ParsePlainTextBatch(tt.input)
			if len(parseErrors) != 0 {
				t.Fatalf("expected no parse errors, got %v", parseErrors)
			}
			if len(items) != 1 {
				t.Fatalf("expected 1 item, got %d", len(items))
			}

			assertParsedItem(t, items[0], tt.expectedType, tt.expectedRate, tt.expectedIndex, tt.expectedModality, "Banco BTG Pactual", tt.expectedMaturity)
		})
	}
}

func TestParsePlainTextBatchParsesTesouroSelicAndKeepsSupportedItems(t *testing.T) {
	input := `Tesouro Selic 2029
Pós-Fixado
Liquidez diária
18/03/2029
0,0800%
R$ 163,54
Investir
Tesouro IPCA+ 2032
IPCA + Juros Semestrais
15/08/2032
7,70%
R$ 1.000,00
Investir
Ver mais produtos
	Atualizado às 14:30
Investir`

	items, parseErrors := ParsePlainTextBatch(input)
	if len(parseErrors) != 0 {
		t.Fatalf("expected no parse errors, got %v", parseErrors)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 parsed items, got %d", len(items))
	}

	assertParsedItem(t, items[0], domain.TypeTesouroSelic, 0.08, domain.IndexSELIC, domain.ModalityPOS, "", "2029-03-18")
	assertParsedItem(t, items[1], domain.TypeTesouroIPCA, 7.70, domain.IndexIPCA, domain.ModalityIPCA, "", "2032-08-15")
}

func TestParsePlainTextBatchParsesTesouroSelic(t *testing.T) {
	input := `Produto e risco
Vencimento
Taxa
Indexador
Aplicação mínima
Tesouro Selic 2031
LFT
Conservador
01/03/2031
SELIC + 0,09% a.a.
SELIC
R$ 184,81
Preço unitário: R$ 18.481,30
Investir`

	items, parseErrors := ParsePlainTextBatch(input)
	if len(parseErrors) != 0 {
		t.Fatalf("expected no parse errors, got %v", parseErrors)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 parsed item, got %d", len(items))
	}
	assertParsedItem(t, items[0], domain.TypeTesouroSelic, 0.09, domain.IndexSELIC, domain.ModalityPOS, "", "2031-03-01")
}
