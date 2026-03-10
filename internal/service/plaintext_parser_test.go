package service

import (
	"testing"

	"anlisys-rfx/internal/domain"
)

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

	first := items[0]
	if first.Type != domain.TypeLCI {
		t.Fatalf("expected first type %q, got %q", domain.TypeLCI, first.Type)
	}
	if first.Issuer != "Banco BTG Pactual" {
		t.Fatalf("expected first issuer Banco BTG Pactual, got %q", first.Issuer)
	}
	if first.Index != domain.IndexPrefixado || first.Modality != domain.ModalityPRE {
		t.Fatalf("expected first index/modality PREFIXADO/PRE, got %s/%s", first.Index, first.Modality)
	}
	if first.Rate != 11.77 {
		t.Fatalf("expected first rate 11.77, got %v", first.Rate)
	}
	if first.MaturityDate != "2026-09-10" {
		t.Fatalf("expected first maturity 2026-09-10, got %s", first.MaturityDate)
	}

	second := items[1]
	if second.Type != domain.TypeLCA {
		t.Fatalf("expected second type %q, got %q", domain.TypeLCA, second.Type)
	}
	if second.Issuer != "Banco BTG Pactual" {
		t.Fatalf("expected second issuer Banco BTG Pactual, got %q", second.Issuer)
	}
	if second.Rate != 11.31 {
		t.Fatalf("expected second rate 11.31, got %v", second.Rate)
	}
	if second.MaturityDate != "2028-09-11" {
		t.Fatalf("expected second maturity 2028-09-11, got %s", second.MaturityDate)
	}
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
