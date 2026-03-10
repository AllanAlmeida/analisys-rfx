package domain

import (
	"errors"
	"strings"
)

const (
	TypeCDB              = "CDB"
	TypeLCI              = "LCI"
	TypeLCA              = "LCA"
	TypeTesouroPrefixado = "TESOURO PREFIXADO"
	TypeTesouroIPCA      = "TESOURO IPCA+"
)

var (
	ErrTypeRequired    = errors.New("field 'type' is required")
	ErrRateInvalid     = errors.New("field 'rate' must be greater than 0")
	ErrInvalidIndex    = errors.New("invalid index for investment type")
	ErrUnsupportedType = errors.New("unsupported investment type")
)

const (
	IndexCDI       = "CDI"
	IndexPrefixado = "PREFIXADO"
	IndexIPCA      = "IPCA"
)

const (
	ClassificationExceptional = "Excepcional"
	ClassificationGood        = "Bom"
	ClassificationAcceptable  = "Aceitável"
	ClassificationWeak        = "Fraco"
)

type AnalyzeInvestmentRequest struct {
	Type  string  `json:"type"`
	Rate  float64 `json:"rate"`
	Index string  `json:"index"`
}

type Indicators struct {
	SELIC float64 `json:"selic"`
	IPCA  float64 `json:"ipca"`
	CDI   float64 `json:"cdi"`
}

type AnalyzeInvestmentResponse struct {
	Classification      string     `json:"classification"`
	Score               float64    `json:"score"`
	EquivalentCDB       float64    `json:"equivalent_cdb"`
	EquivalentCDIReturn float64    `json:"equivalent_cdi_return"`
	RealReturn          float64    `json:"real_return"`
	Description         string     `json:"description"`
	Indicators          Indicators `json:"indicators"`
}

func (r *AnalyzeInvestmentRequest) Normalize() {
	r.Type = strings.ToUpper(strings.TrimSpace(r.Type))
	r.Index = strings.ToUpper(strings.TrimSpace(r.Index))
}

func (r *AnalyzeInvestmentRequest) Validate() error {
	if strings.TrimSpace(r.Type) == "" {
		return ErrTypeRequired
	}
	if r.Rate <= 0 {
		return ErrRateInvalid
	}

	switch r.Type {
	case TypeCDB, TypeLCI, TypeLCA:
		if r.Index == "" {
			r.Index = IndexCDI
		}
		if r.Index != IndexCDI {
			return ErrInvalidIndex
		}
	case TypeTesouroPrefixado:
		if r.Index == "" {
			r.Index = IndexPrefixado
		}
		if r.Index != IndexPrefixado {
			return ErrInvalidIndex
		}
	case TypeTesouroIPCA:
		if r.Index == "" {
			r.Index = IndexIPCA
		}
		if r.Index != IndexIPCA {
			return ErrInvalidIndex
		}
	default:
		return ErrUnsupportedType
	}

	return nil
}
