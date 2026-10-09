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

// percentOfCDI expressa uma taxa nominal anual como percentual do CDI.
// Devolve 0 quando o CDI nao esta disponivel, em vez de dividir por zero —
// esse guarda aparecia repetido cinco vezes em cada uma das duas funcoes de
// equivalencia.
func percentOfCDI(rate, cdi float64) float64 {
	if cdi == 0 {
		return 0
	}

	return (rate / cdi) * 100
}

// calculateEquivalentCDI expressa o rendimento do papel como percentual do CDI.
// Para os papeis que ja sao cotados em percentual do CDI, e a propria taxa.
func calculateEquivalentCDI(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	switch req.Type {
	case domain.TypeCDB:
		return req.Rate
	case domain.TypeLCI, domain.TypeLCA:
		if req.Index == domain.IndexPrefixado {
			return percentOfCDI(req.Rate, indicators.CDI)
		}
		return req.Rate
	case domain.TypeTesouroSelic:
		return percentOfCDI(tesouroSelicNominalRate(req, indicators), indicators.CDI)
	case domain.TypeTesouroPrefixado:
		return percentOfCDI(req.Rate, indicators.CDI)
	case domain.TypeTesouroIPCA:
		return percentOfCDI(utils.NominalRateFromIPCAPlus(indicators.IPCA, req.Rate), indicators.CDI)
	default:
		return 0
	}
}

// calculateEquivalentCDB traduz o papel para o percentual do CDI que um CDB
// precisaria pagar para render o mesmo liquido. LCI e LCA sao isentas de IR,
// por isso passam por EquivalentCDBForTaxFree.
func calculateEquivalentCDB(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators, equivalentCDI float64) float64 {
	switch req.Type {
	case domain.TypeLCI, domain.TypeLCA:
		if req.Index == domain.IndexPrefixado {
			return utils.EquivalentCDBForTaxFree(equivalentCDI)
		}
		return utils.EquivalentCDBForTaxFree(req.Rate)
	default:
		// Para os demais, o equivalente em CDB e o proprio equivalente em CDI.
		return calculateEquivalentCDI(req, indicators)
	}
}

func nominalRate(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	switch req.Type {
	case domain.TypeTesouroSelic:
		return tesouroSelicNominalRate(req, indicators)
	case domain.TypeCDB:
		return indicators.CDI * req.Rate / 100
	case domain.TypeLCI, domain.TypeLCA:
		// Pre-fixado ja informa a taxa anual; pos-fixado e percentual do CDI.
		if req.Index == domain.IndexPrefixado {
			return req.Rate
		}
		return indicators.CDI * req.Rate / 100
	default:
		// Tesouro Prefixado informa a taxa anual diretamente.
		return req.Rate
	}
}

func calculateRealReturn(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	// Tesouro IPCA+ ja e cotado em taxa real, acima da inflacao.
	if req.Type == domain.TypeTesouroIPCA {
		return req.Rate
	}

	return utils.RealReturn(nominalRate(req, indicators), indicators.IPCA)
}

// faixaBonus acrescenta bonus ao score quando o valor avaliado alcanca min.
// As faixas ficam em ordem decrescente e apenas a primeira que casar e aplicada.
type faixaBonus struct {
	min   float64
	bonus float64
}

var (
	pontuacaoBase = map[string]float64{
		domain.ClassificationExceptional: 9.5,
		domain.ClassificationGood:        8.0,
		domain.ClassificationAcceptable:  6.5,
		domain.ClassificationWeak:        4.0,
	}

	bonusPorTaxa = []faixaBonus{
		{min: 120, bonus: 0.5},
		{min: 110, bonus: 0.3},
		{min: 100, bonus: 0.1},
	}

	bonusPorRetornoReal = []faixaBonus{
		{min: 6, bonus: 0.3},
		{min: 4, bonus: 0.2},
		{min: 2, bonus: 0.1},
	}

	// Papeis cotados em percentual do CDI concorrem ao bonus por taxa.
	concorreAoBonusPorTaxa = map[string]bool{
		domain.TypeCDB: true,
		domain.TypeLCI: true,
		domain.TypeLCA: true,
	}

	// LCI e LCA sao isentas de imposto de renda para pessoa fisica.
	isentoDeIR = map[string]bool{
		domain.TypeLCI: true,
		domain.TypeLCA: true,
	}
)

const bonusIsencaoIR = 0.2

func bonusDe(valor float64, faixas []faixaBonus) float64 {
	for _, faixa := range faixas {
		if valor >= faixa.min {
			return faixa.bonus
		}
	}

	return 0
}

func calculateScore(classification string, req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	// Os bonus sao somados entre si antes de entrar na base: acumular direto no
	// score daria um resultado diferente no ultimo bit.
	extra := 0.0

	if concorreAoBonusPorTaxa[req.Type] {
		extra += bonusDe(taxaBaseDoBonus(req, indicators), bonusPorTaxa)
	}

	extra += bonusDe(calculateRealReturn(req, indicators), bonusPorRetornoReal)

	if isentoDeIR[req.Type] {
		extra += bonusIsencaoIR
	}

	return min(max(pontuacaoBase[classification]+extra, 0), 10)
}

// taxaBaseDoBonus devolve a taxa comparada com bonusPorTaxa. Papel pre-fixado
// informa taxa anual, entao precisa ser traduzido para percentual do CDI antes.
func taxaBaseDoBonus(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	if req.Index == domain.IndexPrefixado {
		return calculateEquivalentCDI(req, indicators)
	}

	return req.Rate
}

func tesouroSelicNominalRate(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators) float64 {
	return indicators.SELIC + req.Rate
}
