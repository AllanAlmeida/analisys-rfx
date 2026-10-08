package service

import (
	"testing"

	"investment-analyzer/internal/domain"
)

// Golden test de classify: congela a classificacao e a descricao de cada tipo
// nas suas fronteiras. Serve de rede para a conversao das cascatas de if em
// tabela de faixas — qualquer mudanca de comportamento aparece aqui.
func TestClassifyGolden(t *testing.T) {
	casos := []struct {
		tipo      string
		index     string
		rate      float64
		eqCDI     float64
		classe    string
		descricao string
	}{
		{"CDB", "CDI", 125, 125, "Excepcional", "CDB com 120% ou mais do CDI"},
		{"CDB", "CDI", 120, 120, "Excepcional", "CDB com 120% ou mais do CDI"},
		{"CDB", "CDI", 119, 119, "Bom", "CDB entre 105% e 119% do CDI"},
		{"CDB", "CDI", 105, 105, "Bom", "CDB entre 105% e 119% do CDI"},
		{"CDB", "CDI", 104, 104, "Aceitável", "CDB entre 100% e 104% do CDI"},
		{"CDB", "CDI", 100, 100, "Aceitável", "CDB entre 100% e 104% do CDI"},
		{"CDB", "CDI", 99, 99, "Fraco", "CDB abaixo de 100% do CDI"},
		{"CDB", "CDI", 1, 1, "Fraco", "CDB abaixo de 100% do CDI"},
		{"LCI", "CDI", 97, 110, "Excepcional", "LCI com 100% ou mais do CDI"},
		{"LCI", "CDI", 97, 100, "Excepcional", "LCI com 100% ou mais do CDI"},
		{"LCI", "CDI", 97, 99, "Bom", "LCI entre 90% e 99% do CDI"},
		{"LCI", "CDI", 97, 90, "Bom", "LCI entre 90% e 99% do CDI"},
		{"LCI", "CDI", 97, 89, "Aceitável", "LCI entre 85% e 89% do CDI"},
		{"LCI", "CDI", 97, 85, "Aceitável", "LCI entre 85% e 89% do CDI"},
		{"LCI", "CDI", 97, 84, "Fraco", "LCI abaixo de 85% do CDI"},
		{"LCI", "PREFIXADO", 13.5, 110, "Excepcional", "LCI com 100% ou mais do CDI"},
		{"LCI", "PREFIXADO", 13.5, 100, "Excepcional", "LCI com 100% ou mais do CDI"},
		{"LCI", "PREFIXADO", 13.5, 99, "Bom", "LCI pre-fixado equivalente entre 90% e 99% do CDI"},
		{"LCI", "PREFIXADO", 13.5, 90, "Bom", "LCI pre-fixado equivalente entre 90% e 99% do CDI"},
		{"LCI", "PREFIXADO", 13.5, 89, "Aceitável", "LCI pre-fixado equivalente entre 85% e 89% do CDI"},
		{"LCI", "PREFIXADO", 13.5, 85, "Aceitável", "LCI pre-fixado equivalente entre 85% e 89% do CDI"},
		{"LCI", "PREFIXADO", 13.5, 84, "Fraco", "LCI pre-fixado equivalente abaixo de 85% do CDI"},
		{"LCA", "CDI", 88, 100, "Excepcional", "LCA com 100% ou mais do CDI"},
		{"LCA", "CDI", 88, 92, "Bom", "LCA entre 90% e 99% do CDI"},
		{"LCA", "CDI", 88, 87, "Aceitável", "LCA entre 85% e 89% do CDI"},
		{"LCA", "CDI", 88, 80, "Fraco", "LCA abaixo de 85% do CDI"},
		{"LCA", "PREFIXADO", 13, 92, "Bom", "LCA pre-fixado equivalente entre 90% e 99% do CDI"},
		{"LCA", "PREFIXADO", 13, 87, "Aceitável", "LCA pre-fixado equivalente entre 85% e 89% do CDI"},
		{"LCA", "PREFIXADO", 13, 80, "Fraco", "LCA pre-fixado equivalente abaixo de 85% do CDI"},
		{"TESOURO SELIC", "SELIC", 0.2, 0, "Excepcional", "Tesouro Selic com spread >= 0.15% a.a."},
		{"TESOURO SELIC", "SELIC", 0.15, 0, "Excepcional", "Tesouro Selic com spread >= 0.15% a.a."},
		{"TESOURO SELIC", "SELIC", 0.14, 0, "Bom", "Tesouro Selic com spread entre 0.05% e 0.14% a.a."},
		{"TESOURO SELIC", "SELIC", 0.05, 0, "Bom", "Tesouro Selic com spread entre 0.05% e 0.14% a.a."},
		{"TESOURO SELIC", "SELIC", 0.04, 0, "Aceitável", "Tesouro Selic com spread entre 0.00% e 0.04% a.a."},
		{"TESOURO SELIC", "SELIC", 0, 0, "Aceitável", "Tesouro Selic com spread entre 0.00% e 0.04% a.a."},
		{"TESOURO SELIC", "SELIC", -0.01, 0, "Fraco", "Tesouro Selic com spread abaixo de 0.00% a.a."},
		{"TESOURO PREFIXADO", "PREFIXADO", 16, 0, "Excepcional", "Tesouro Prefixado com taxa >= 15.5%"},
		{"TESOURO PREFIXADO", "PREFIXADO", 15.5, 0, "Excepcional", "Tesouro Prefixado com taxa >= 15.5%"},
		{"TESOURO PREFIXADO", "PREFIXADO", 15.4, 0, "Bom", "Tesouro Prefixado entre 14.5% e 15.4%"},
		{"TESOURO PREFIXADO", "PREFIXADO", 14.5, 0, "Bom", "Tesouro Prefixado entre 14.5% e 15.4%"},
		{"TESOURO PREFIXADO", "PREFIXADO", 14.4, 0, "Aceitável", "Tesouro Prefixado entre 13.5% e 14.4%"},
		{"TESOURO PREFIXADO", "PREFIXADO", 13.5, 0, "Aceitável", "Tesouro Prefixado entre 13.5% e 14.4%"},
		{"TESOURO PREFIXADO", "PREFIXADO", 13.4, 0, "Fraco", "Tesouro Prefixado abaixo de 13.5%"},
		{"TESOURO IPCA+", "IPCA", 7, 0, "Excepcional", "Tesouro IPCA+ com taxa real >= 6.5%"},
		{"TESOURO IPCA+", "IPCA", 6.5, 0, "Excepcional", "Tesouro IPCA+ com taxa real >= 6.5%"},
		{"TESOURO IPCA+", "IPCA", 6.4, 0, "Bom", "Tesouro IPCA+ entre 5.8% e 6.4%"},
		{"TESOURO IPCA+", "IPCA", 5.8, 0, "Bom", "Tesouro IPCA+ entre 5.8% e 6.4%"},
		{"TESOURO IPCA+", "IPCA", 5.7, 0, "Aceitável", "Tesouro IPCA+ entre 5.0% e 5.7%"},
		{"TESOURO IPCA+", "IPCA", 5, 0, "Aceitável", "Tesouro IPCA+ entre 5.0% e 5.7%"},
		{"TESOURO IPCA+", "IPCA", 4.9, 0, "Fraco", "Tesouro IPCA+ abaixo de 5.0%"},
		{"XPTO", "", 10, 0, "Fraco", "Tipo de investimento nao suportado"},
	}

	for _, c := range casos {
		req := domain.AnalyzeInvestmentRequest{Type: c.tipo, Index: c.index, Rate: c.rate}

		classe, descricao := classify(req, c.eqCDI)

		if classe != c.classe || descricao != c.descricao {
			t.Errorf("classify(%s/%s rate=%v eqCDI=%v)\n  esperado: %q / %q\n  obtido:   %q / %q",
				c.tipo, c.index, c.rate, c.eqCDI, c.classe, c.descricao, classe, descricao)
		}
	}
}
