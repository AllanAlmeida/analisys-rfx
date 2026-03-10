package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	TypeCDB              = "CDB"
	TypeLCI              = "LCI"
	TypeLCA              = "LCA"
	TypeTesouroPrefixado = "TESOURO PREFIXADO"
	TypeTesouroIPCA      = "TESOURO IPCA+"
)

var (
	ErrTypeRequired        = errors.New("field 'type' is required")
	ErrRateInvalid         = errors.New("field 'rate' must be greater than 0")
	ErrInvalidIndex        = errors.New("invalid index for investment type")
	ErrInvalidModality     = errors.New("invalid modality for investment type")
	ErrInvalidMaturityDate = errors.New("field 'maturity_date' must be in YYYY-MM-DD format")
	ErrUnsupportedType     = errors.New("unsupported investment type")
)

const (
	IndexCDI       = "CDI"
	IndexPrefixado = "PREFIXADO"
	IndexIPCA      = "IPCA"
)

const (
	ModalityPOS  = "POS"
	ModalityPRE  = "PRE"
	ModalityIPCA = "IPCA"
)

const (
	ClassificationExceptional = "Excepcional"
	ClassificationGood        = "Bom"
	ClassificationAcceptable  = "Aceitável"
	ClassificationWeak        = "Fraco"
)

type AnalyzeInvestmentRequest struct {
	Type         string  `json:"type"`
	Rate         float64 `json:"rate"`
	Index        string  `json:"index"`
	Modality     string  `json:"modality,omitempty"`
	MaturityDate string  `json:"maturity_date,omitempty"`
	Issuer       string  `json:"issuer,omitempty"`
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

type AnalyzeBatchRequest struct {
	Items []AnalyzeInvestmentRequest `json:"items"`
}

type AnalyzeBatchItemResult struct {
	Index  int                        `json:"index"`
	Input  AnalyzeInvestmentRequest   `json:"input"`
	Result *AnalyzeInvestmentResponse `json:"result,omitempty"`
	Error  string                     `json:"error,omitempty"`
}

type AnalyzeBatchResponse struct {
	Total  int                      `json:"total"`
	Ok     int                      `json:"ok"`
	Failed int                      `json:"failed"`
	Items  []AnalyzeBatchItemResult `json:"items"`
}

func (r *AnalyzeInvestmentRequest) Normalize() {
	r.Type = strings.ToUpper(strings.TrimSpace(r.Type))
	r.Index = strings.ToUpper(strings.TrimSpace(r.Index))
	r.Modality = strings.ToUpper(strings.TrimSpace(r.Modality))
	r.MaturityDate = strings.TrimSpace(r.MaturityDate)
	r.Issuer = strings.TrimSpace(r.Issuer)
}

func (r *AnalyzeInvestmentRequest) Validate() error {
	if strings.TrimSpace(r.Type) == "" {
		return ErrTypeRequired
	}
	if r.Rate <= 0 {
		return ErrRateInvalid
	}

	switch r.Type {
	case TypeCDB:
		if r.Index == "" {
			r.Index = IndexCDI
		}
		if r.Index != IndexCDI {
			return ErrInvalidIndex
		}
		if r.Modality == "" {
			r.Modality = ModalityPOS
		}
		if r.Modality != ModalityPOS {
			return ErrInvalidModality
		}
	case TypeLCI, TypeLCA:
		if r.Index == "" {
			r.Index = IndexCDI
		}
		if r.Modality == "" {
			if r.Index == IndexPrefixado {
				r.Modality = ModalityPRE
			} else {
				r.Modality = ModalityPOS
			}
		}
		if (r.Index == IndexCDI && r.Modality != ModalityPOS) ||
			(r.Index == IndexPrefixado && r.Modality != ModalityPRE) {
			return ErrInvalidModality
		}
		if r.Index != IndexCDI && r.Index != IndexPrefixado {
			return ErrInvalidIndex
		}
	case TypeTesouroPrefixado:
		if r.Index == "" {
			r.Index = IndexPrefixado
		}
		if r.Index != IndexPrefixado {
			return ErrInvalidIndex
		}
		if r.Modality == "" {
			r.Modality = ModalityPRE
		}
		if r.Modality != ModalityPRE {
			return ErrInvalidModality
		}
	case TypeTesouroIPCA:
		if r.Index == "" {
			r.Index = IndexIPCA
		}
		if r.Index != IndexIPCA {
			return ErrInvalidIndex
		}
		if r.Modality == "" {
			r.Modality = ModalityIPCA
		}
		if r.Modality != ModalityIPCA {
			return ErrInvalidModality
		}
	default:
		return ErrUnsupportedType
	}

	if r.MaturityDate != "" {
		if _, err := time.Parse(time.DateOnly, r.MaturityDate); err != nil {
			return ErrInvalidMaturityDate
		}
	}

	return nil
}
