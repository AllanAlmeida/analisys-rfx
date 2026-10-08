package service

import (
	"context"
	"math"
	"strings"

	"investment-analyzer/internal/domain"
	"investment-analyzer/pkg/utils"
)

type AnalyzerService struct {
	economyService EconomyService
}

func NewAnalyzerService(economyService EconomyService) *AnalyzerService {
	return &AnalyzerService{
		economyService: economyService,
	}
}

func (s *AnalyzerService) Analyze(ctx context.Context, req domain.AnalyzeInvestmentRequest) (domain.AnalyzeInvestmentResponse, error) {
	req.Normalize()
	if err := req.Validate(); err != nil {
		return domain.AnalyzeInvestmentResponse{}, err
	}

	indicators, err := s.economyService.GetIndicators(ctx)
	if err != nil {
		return domain.AnalyzeInvestmentResponse{}, err
	}

	equivalentCDI := calculateEquivalentCDI(req, indicators)
	classification, description := classify(req, equivalentCDI)
	equivalentCDB := calculateEquivalentCDB(req, indicators, equivalentCDI)
	realReturn := calculateRealReturn(req, indicators)
	score := calculateScore(classification, req, indicators)

	return domain.AnalyzeInvestmentResponse{
		Classification:      classification,
		Score:               utils.Round(score, 1),
		EquivalentCDB:       utils.Round(equivalentCDB, 2),
		EquivalentCDIReturn: utils.Round(equivalentCDI, 2),
		RealReturn:          utils.Round(realReturn, 2),
		Description:         description,
		Indicators: domain.Indicators{
			SELIC: utils.Round(indicators.SELIC, 2),
			IPCA:  utils.Round(indicators.IPCA, 2),
			CDI:   utils.Round(indicators.CDI, 2),
		},
	}, nil
}

// faixaClassificacao e uma fronteira: valor >= min resulta na classificacao e
// na descricao correspondentes. As faixas de cada tipo ficam em ordem
// decrescente de min, e a ultima usa math.Inf(-1) para servir de fallback.
type faixaClassificacao struct {
	min          float64
	classe       string
	descricao    string
	descricaoPre string // usada quando o indice e PREFIXADO; vazia = usa descricao
}

// regraClassificacao descreve como classificar um tipo de investimento: de onde
// sai o valor comparado e quais sao as faixas.
type regraClassificacao struct {
	// prefixaTipo indica que a descricao comeca com o tipo ("LCI entre ...").
	prefixaTipo bool
	valor       func(req domain.AnalyzeInvestmentRequest, equivalentCDI float64) float64
	faixas      []faixaClassificacao
}

func (r regraClassificacao) descricaoDa(faixa faixaClassificacao, req domain.AnalyzeInvestmentRequest) string {
	texto := faixa.descricao
	if faixa.descricaoPre != "" && req.Index == domain.IndexPrefixado {
		texto = faixa.descricaoPre
	}

	if r.prefixaTipo {
		return req.Type + " " + texto
	}

	return texto
}

func valorTaxa(req domain.AnalyzeInvestmentRequest, _ float64) float64 {
	return req.Rate
}

func valorEquivalenteCDI(_ domain.AnalyzeInvestmentRequest, equivalentCDI float64) float64 {
	return equivalentCDI
}

// Acrescentar um tipo de investimento e acrescentar uma entrada aqui.
var regrasClassificacao = map[string]regraClassificacao{
	domain.TypeCDB: {
		valor: valorTaxa,
		faixas: []faixaClassificacao{
			{min: 120, classe: domain.ClassificationExceptional, descricao: "CDB com 120% ou mais do CDI"},
			{min: 105, classe: domain.ClassificationGood, descricao: "CDB entre 105% e 119% do CDI"},
			{min: 100, classe: domain.ClassificationAcceptable, descricao: "CDB entre 100% e 104% do CDI"},
			{min: math.Inf(-1), classe: domain.ClassificationWeak, descricao: "CDB abaixo de 100% do CDI"},
		},
	},
	domain.TypeLCI: regraLetraDeCredito,
	domain.TypeLCA: regraLetraDeCredito,
	domain.TypeTesouroSelic: {
		valor: valorTaxa,
		faixas: []faixaClassificacao{
			{min: 0.15, classe: domain.ClassificationExceptional, descricao: "Tesouro Selic com spread >= 0.15% a.a."},
			{min: 0.05, classe: domain.ClassificationGood, descricao: "Tesouro Selic com spread entre 0.05% e 0.14% a.a."},
			{min: 0, classe: domain.ClassificationAcceptable, descricao: "Tesouro Selic com spread entre 0.00% e 0.04% a.a."},
			{min: math.Inf(-1), classe: domain.ClassificationWeak, descricao: "Tesouro Selic com spread abaixo de 0.00% a.a."},
		},
	},
	domain.TypeTesouroPrefixado: {
		valor: valorTaxa,
		faixas: []faixaClassificacao{
			{min: 15.5, classe: domain.ClassificationExceptional, descricao: "Tesouro Prefixado com taxa >= 15.5%"},
			{min: 14.5, classe: domain.ClassificationGood, descricao: "Tesouro Prefixado entre 14.5% e 15.4%"},
			{min: 13.5, classe: domain.ClassificationAcceptable, descricao: "Tesouro Prefixado entre 13.5% e 14.4%"},
			{min: math.Inf(-1), classe: domain.ClassificationWeak, descricao: "Tesouro Prefixado abaixo de 13.5%"},
		},
	},
	domain.TypeTesouroIPCA: {
		valor: valorTaxa,
		faixas: []faixaClassificacao{
			{min: 6.5, classe: domain.ClassificationExceptional, descricao: "Tesouro IPCA+ com taxa real >= 6.5%"},
			{min: 5.8, classe: domain.ClassificationGood, descricao: "Tesouro IPCA+ entre 5.8% e 6.4%"},
			{min: 5.0, classe: domain.ClassificationAcceptable, descricao: "Tesouro IPCA+ entre 5.0% e 5.7%"},
			{min: math.Inf(-1), classe: domain.ClassificationWeak, descricao: "Tesouro IPCA+ abaixo de 5.0%"},
		},
	},
}

// LCI e LCA compartilham as mesmas faixas; a descricao recebe o tipo como
// prefixo e varia quando o papel e pre-fixado.
var regraLetraDeCredito = regraClassificacao{
	prefixaTipo: true,
	valor:       valorEquivalenteCDI,
	faixas: []faixaClassificacao{
		{min: 100, classe: domain.ClassificationExceptional,
			descricao: "com 100% ou mais do CDI"},
		{min: 90, classe: domain.ClassificationGood,
			descricao:    "entre 90% e 99% do CDI",
			descricaoPre: "pre-fixado equivalente entre 90% e 99% do CDI"},
		{min: 85, classe: domain.ClassificationAcceptable,
			descricao:    "entre 85% e 89% do CDI",
			descricaoPre: "pre-fixado equivalente entre 85% e 89% do CDI"},
		{min: math.Inf(-1), classe: domain.ClassificationWeak,
			descricao:    "abaixo de 85% do CDI",
			descricaoPre: "pre-fixado equivalente abaixo de 85% do CDI"},
	},
}

func classify(req domain.AnalyzeInvestmentRequest, equivalentCDI float64) (string, string) {
	regra, conhecido := regrasClassificacao[strings.ToUpper(req.Type)]
	if !conhecido {
		return domain.ClassificationWeak, descricaoTipoNaoSuportado
	}

	valor := regra.valor(req, equivalentCDI)

	for _, faixa := range regra.faixas {
		if valor >= faixa.min {
			return faixa.classe, regra.descricaoDa(faixa, req)
		}
	}

	// Inalcancavel com faixas bem formadas: a ultima tem min = -Inf. So chega
	// aqui se valor for NaN.
	return domain.ClassificationWeak, descricaoTipoNaoSuportado
}

const descricaoTipoNaoSuportado = "Tipo de investimento nao suportado"

func calculateEquivalentCDB(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators, equivalentCDI float64) float64 {
	switch req.Type {
	case domain.TypeLCI, domain.TypeLCA:
		if req.Index == domain.IndexPrefixado {
			return utils.EquivalentCDBForTaxFree(equivalentCDI)
		}
		return utils.EquivalentCDBForTaxFree(req.Rate)
	case domain.TypeCDB:
		return req.Rate
	case domain.TypeTesouroSelic:
		if indicators.CDI == 0 {
			return 0
		}
		return (tesouroSelicNominalRate(req, indicators) / indicators.CDI) * 100
	case domain.TypeTesouroPrefixado:
		if indicators.CDI == 0 {
			return 0
		}
		return (req.Rate / indicators.CDI) * 100
	case domain.TypeTesouroIPCA:
		nominal := utils.NominalRateFromIPCAPlus(indicators.IPCA, req.Rate)
		if indicators.CDI == 0 {
			return 0
		}
		return (nominal / indicators.CDI) * 100
	default:
		return 0
	}
}

func calculateEquivalentCDI(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	switch req.Type {
	case domain.TypeCDB:
		return req.Rate
	case domain.TypeLCI, domain.TypeLCA:
		if req.Index == domain.IndexPrefixado {
			if indicators.CDI == 0 {
				return 0
			}
			return (req.Rate / indicators.CDI) * 100
		}
		return req.Rate
	case domain.TypeTesouroSelic:
		if indicators.CDI == 0 {
			return 0
		}
		return (tesouroSelicNominalRate(req, indicators) / indicators.CDI) * 100
	case domain.TypeTesouroPrefixado:
		if indicators.CDI == 0 {
			return 0
		}
		return (req.Rate / indicators.CDI) * 100
	case domain.TypeTesouroIPCA:
		nominal := utils.NominalRateFromIPCAPlus(indicators.IPCA, req.Rate)
		if indicators.CDI == 0 {
			return 0
		}
		return (nominal / indicators.CDI) * 100
	default:
		return 0
	}
}

func calculateRealReturn(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	switch req.Type {
	case domain.TypeTesouroSelic:
		return utils.RealReturn(tesouroSelicNominalRate(req, indicators), indicators.IPCA)
	case domain.TypeTesouroIPCA:
		return req.Rate
	default:
		return utils.RealReturn(req.Rate, indicators.IPCA)
	}
}

func calculateScore(classification string, req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	base := map[string]float64{
		domain.ClassificationExceptional: 9.5,
		domain.ClassificationGood:        8.0,
		domain.ClassificationAcceptable:  6.5,
		domain.ClassificationWeak:        4.0,
	}[classification]

	extra := 0.0

	if req.Type == domain.TypeCDB || req.Type == domain.TypeLCI || req.Type == domain.TypeLCA {
		baseRate := req.Rate
		if req.Index == domain.IndexPrefixado {
			baseRate = calculateEquivalentCDI(req, indicators)
		}

		if baseRate >= 120 {
			extra += 0.5
		} else if baseRate >= 110 {
			extra += 0.3
		} else if baseRate >= 100 {
			extra += 0.1
		}
	}

	realReturn := calculateRealReturn(req, indicators)
	if realReturn >= 6 {
		extra += 0.3
	} else if realReturn >= 4 {
		extra += 0.2
	} else if realReturn >= 2 {
		extra += 0.1
	}

	if req.Type == domain.TypeLCI || req.Type == domain.TypeLCA {
		extra += 0.2
	}

	score := base + extra
	if score > 10 {
		return 10
	}
	if score < 0 {
		return 0
	}
	return score
}

func tesouroSelicNominalRate(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	return indicators.SELIC + req.Rate
}
