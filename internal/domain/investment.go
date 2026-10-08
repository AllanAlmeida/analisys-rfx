package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
)

const (
	TypeCDB              = "CDB"
	TypeLCI              = "LCI"
	TypeLCA              = "LCA"
	TypeTesouroSelic     = "TESOURO SELIC"
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
	IndexSELIC     = "SELIC"
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

// comboIndice e uma combinacao valida de indice e modalidade para um tipo de
// investimento. modalidadeDefault e aplicada quando o request nao traz
// modalidade, e modalidadesOK lista as aceitas para esse indice.
type comboIndice struct {
	indice            string
	modalidadeDefault string
	modalidadesOK     []string
}

// regraValidacao reune os combos de um tipo. O primeiro combo define o indice
// aplicado quando o request nao traz nenhum.
type regraValidacao struct {
	combos []comboIndice
}

func (regra regraValidacao) comboDe(indice string) (comboIndice, bool) {
	for _, combo := range regra.combos {
		if combo.indice == indice {
			return combo, true
		}
	}

	return comboIndice{}, false
}

// Acrescentar um tipo de investimento e acrescentar uma entrada aqui.
var regrasValidacao = map[string]regraValidacao{
	TypeCDB: {combos: []comboIndice{
		{indice: IndexCDI, modalidadeDefault: ModalityPOS, modalidadesOK: []string{ModalityPOS}},
	}},
	// LCI e LCA aceitam dois indices, cada um com a sua modalidade.
	TypeLCI: {combos: comboLetraDeCredito},
	TypeLCA: {combos: comboLetraDeCredito},
	TypeTesouroSelic: {combos: []comboIndice{
		{indice: IndexSELIC, modalidadeDefault: ModalityPOS, modalidadesOK: []string{ModalityPOS}},
	}},
	TypeTesouroPrefixado: {combos: []comboIndice{
		{indice: IndexPrefixado, modalidadeDefault: ModalityPRE, modalidadesOK: []string{ModalityPRE}},
	}},
	TypeTesouroIPCA: {combos: []comboIndice{
		{indice: IndexIPCA, modalidadeDefault: ModalityIPCA, modalidadesOK: []string{ModalityIPCA}},
	}},
}

var comboLetraDeCredito = []comboIndice{
	{indice: IndexCDI, modalidadeDefault: ModalityPOS, modalidadesOK: []string{ModalityPOS}},
	{indice: IndexPrefixado, modalidadeDefault: ModalityPRE, modalidadesOK: []string{ModalityPRE}},
}

// Validate preenche os defaults de indice e modalidade no proprio request e
// devolve o primeiro erro encontrado.
func (r *AnalyzeInvestmentRequest) Validate() error {
	if strings.TrimSpace(r.Type) == "" {
		return ErrTypeRequired
	}
	if r.Rate <= 0 {
		return ErrRateInvalid
	}

	regra, suportado := regrasValidacao[r.Type]
	if !suportado {
		return ErrUnsupportedType
	}

	if err := regra.aplicarA(r); err != nil {
		return err
	}

	return r.validarVencimento()
}

func (regra regraValidacao) aplicarA(r *AnalyzeInvestmentRequest) error {
	if r.Index == "" {
		r.Index = regra.combos[0].indice
	}

	combo, valido := regra.comboDe(r.Index)
	if !valido {
		return ErrInvalidIndex
	}

	if r.Modality == "" {
		r.Modality = combo.modalidadeDefault
	}

	if !slices.Contains(combo.modalidadesOK, r.Modality) {
		return ErrInvalidModality
	}

	return nil
}

func (r *AnalyzeInvestmentRequest) validarVencimento() error {
	if r.MaturityDate == "" {
		return nil
	}

	if _, err := time.Parse(time.DateOnly, r.MaturityDate); err != nil {
		return ErrInvalidMaturityDate
	}

	return nil
}
