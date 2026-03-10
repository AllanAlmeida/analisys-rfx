package service

import (
	"context"
	"fmt"
	"strings"

	"anlisys-rfx/internal/domain"
	"anlisys-rfx/pkg/utils"
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

func classify(req domain.AnalyzeInvestmentRequest, equivalentCDI float64) (string, string) {
	switch strings.ToUpper(req.Type) {
	case domain.TypeCDB:
		if req.Rate >= 120 {
			return domain.ClassificationExceptional, "CDB com 120% ou mais do CDI"
		}
		if req.Rate >= 105 {
			return domain.ClassificationGood, "CDB entre 105% e 119% do CDI"
		}
		if req.Rate >= 100 {
			return domain.ClassificationAcceptable, "CDB entre 100% e 104% do CDI"
		}
		return domain.ClassificationWeak, "CDB abaixo de 100% do CDI"
	case domain.TypeLCI, domain.TypeLCA:
		if equivalentCDI >= 100 {
			return domain.ClassificationExceptional, fmt.Sprintf("%s com 100%% ou mais do CDI", req.Type)
		}
		if equivalentCDI >= 90 {
			if req.Index == domain.IndexPrefixado {
				return domain.ClassificationGood, fmt.Sprintf("%s pre-fixado equivalente entre 90%% e 99%% do CDI", req.Type)
			}
			return domain.ClassificationGood, fmt.Sprintf("%s entre 90%% e 99%% do CDI", req.Type)
		}
		if equivalentCDI >= 85 {
			if req.Index == domain.IndexPrefixado {
				return domain.ClassificationAcceptable, fmt.Sprintf("%s pre-fixado equivalente entre 85%% e 89%% do CDI", req.Type)
			}
			return domain.ClassificationAcceptable, fmt.Sprintf("%s entre 85%% e 89%% do CDI", req.Type)
		}
		if req.Index == domain.IndexPrefixado {
			return domain.ClassificationWeak, fmt.Sprintf("%s pre-fixado equivalente abaixo de 85%% do CDI", req.Type)
		}
		return domain.ClassificationWeak, fmt.Sprintf("%s abaixo de 85%% do CDI", req.Type)
	case domain.TypeTesouroPrefixado:
		if req.Rate >= 15.5 {
			return domain.ClassificationExceptional, "Tesouro Prefixado com taxa >= 15.5%"
		}
		if req.Rate >= 14.5 {
			return domain.ClassificationGood, "Tesouro Prefixado entre 14.5% e 15.4%"
		}
		if req.Rate >= 13.5 {
			return domain.ClassificationAcceptable, "Tesouro Prefixado entre 13.5% e 14.4%"
		}
		return domain.ClassificationWeak, "Tesouro Prefixado abaixo de 13.5%"
	case domain.TypeTesouroIPCA:
		if req.Rate >= 6.5 {
			return domain.ClassificationExceptional, "Tesouro IPCA+ com taxa real >= 6.5%"
		}
		if req.Rate >= 5.8 {
			return domain.ClassificationGood, "Tesouro IPCA+ entre 5.8% e 6.4%"
		}
		if req.Rate >= 5.0 {
			return domain.ClassificationAcceptable, "Tesouro IPCA+ entre 5.0% e 5.7%"
		}
		return domain.ClassificationWeak, "Tesouro IPCA+ abaixo de 5.0%"
	default:
		return domain.ClassificationWeak, "Tipo de investimento nao suportado"
	}
}

func calculateEquivalentCDB(req domain.AnalyzeInvestmentRequest, indicators EconomyIndicators, equivalentCDI float64) float64 {
	switch req.Type {
	case domain.TypeLCI, domain.TypeLCA:
		if req.Index == domain.IndexPrefixado {
			return utils.EquivalentCDBForTaxFree(equivalentCDI)
		}
		return utils.EquivalentCDBForTaxFree(req.Rate)
	case domain.TypeCDB:
		return req.Rate
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
