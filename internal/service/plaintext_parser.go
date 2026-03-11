package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anlisys-rfx/internal/domain"
)

var (
	dateRegex        = regexp.MustCompile(`\b(\d{2}/\d{2}/\d{4})\b`)
	annualRateRegex  = regexp.MustCompile(`(\d{1,3},\d{1,4})%\s*a\.a\.`)
	cdiRateRegex     = regexp.MustCompile(`(\d{1,3},\d{1,4})%\s*do\s*CDI`)
	percentRateRegex = regexp.MustCompile(`(\d{1,3},\d{1,4})%`)
)

func ParsePlainTextBatch(raw string) ([]domain.AnalyzeInvestmentRequest, []string) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	blocks := collectProductBlocks(lines)
	items := make([]domain.AnalyzeInvestmentRequest, 0, len(blocks))
	parseErrors := make([]string, 0)

	for idx, block := range blocks {
		item, err := parseProductBlock(block)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("item %d: %v", idx, err))
			continue
		}
		items = append(items, item)
	}

	return items, parseErrors
}

func collectProductBlocks(lines []string) []string {
	blocks := make([]string, 0)
	current := make([]string, 0, 12)

	flush := func() {
		if len(current) == 0 {
			return
		}
		blocks = append(blocks, strings.Join(current, "\n"))
		current = current[:0]
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if isHeaderLine(line) {
			continue
		}

		if isProductStartLine(line) && len(current) > 0 {
			flush()
		}

		if len(current) == 0 && !isProductStartLine(line) {
			continue
		}

		current = append(current, line)
		if strings.EqualFold(line, "Investir") {
			flush()
		}
	}

	flush()
	return blocks
}

func isHeaderLine(line string) bool {
	switch strings.ToUpper(line) {
	case "PRODUTO E RISCO", "VENCIMENTO", "TAXA", "TAXA EQ. DO CDB", "APLICAÇÃO MÍNIMA", "ESTOQUE", "APLICACAO MINIMA":
		return true
	default:
		return false
	}
}

func isProductStartLine(line string) bool {
	upper := strings.ToUpper(line)
	return strings.HasPrefix(upper, "LCI") ||
		strings.HasPrefix(upper, "LCA") ||
		strings.HasPrefix(upper, "CDB") ||
		strings.HasPrefix(upper, "TESOURO SELIC") ||
		strings.HasPrefix(upper, "TESOURO PREFIXADO") ||
		strings.HasPrefix(upper, "TESOURO IPCA+")
}

func parseProductBlock(block string) (domain.AnalyzeInvestmentRequest, error) {
	lines := strings.Split(block, "\n")

	investmentType, err := extractInvestmentType(lines)
	if err != nil {
		return domain.AnalyzeInvestmentRequest{}, err
	}
	issuer := extractIssuer(lines)

	modality, index := extractModalityAndIndex(lines, investmentType)

	rate, err := extractRate(lines, index)
	if err != nil {
		return domain.AnalyzeInvestmentRequest{}, err
	}

	maturityDate, err := extractMaturityDate(lines)
	if err != nil {
		return domain.AnalyzeInvestmentRequest{}, err
	}

	return domain.AnalyzeInvestmentRequest{
		Type:         investmentType,
		Rate:         rate,
		Index:        index,
		Modality:     modality,
		MaturityDate: maturityDate,
		Issuer:       issuer,
	}, nil
}

func extractInvestmentType(lines []string) (string, error) {
	for _, line := range lines {
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "LCI"):
			return domain.TypeLCI, nil
		case strings.HasPrefix(upper, "LCA"):
			return domain.TypeLCA, nil
		case strings.HasPrefix(upper, "CDB"):
			return domain.TypeCDB, nil
		case strings.HasPrefix(upper, "TESOURO SELIC"):
			return domain.TypeTesouroSelic, nil
		case strings.HasPrefix(upper, "TESOURO PREFIXADO"):
			return domain.TypeTesouroPrefixado, nil
		case strings.HasPrefix(upper, "TESOURO IPCA+"):
			return domain.TypeTesouroIPCA, nil
		}
	}
	return "", fmt.Errorf("could not identify investment type")
}

func extractIssuer(lines []string) string {
	for _, line := range lines {
		parts := strings.SplitN(line, "-", 2)
		if len(parts) != 2 {
			continue
		}

		left := strings.ToUpper(strings.TrimSpace(parts[0]))
		if left != "LCI" && left != "LCA" && left != "CDB" {
			continue
		}

		return strings.TrimSpace(parts[1])
	}
	return ""
}

func extractRate(lines []string, index string) (float64, error) {
	regexes := []*regexp.Regexp{annualRateRegex, cdiRateRegex, percentRateRegex}
	switch index {
	case domain.IndexCDI:
		regexes = []*regexp.Regexp{cdiRateRegex, annualRateRegex, percentRateRegex}
	case domain.IndexIPCA, domain.IndexPrefixado:
		regexes = []*regexp.Regexp{annualRateRegex, percentRateRegex, cdiRateRegex}
	}

	for _, line := range lines {
		for _, rx := range regexes {
			match := rx.FindStringSubmatch(line)
			if len(match) < 2 {
				continue
			}

			rateString := strings.ReplaceAll(match[1], ",", ".")
			rate, err := strconv.ParseFloat(rateString, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid rate %q", match[1])
			}
			return rate, nil
		}
	}
	return 0, fmt.Errorf("could not identify rate")
}

func extractMaturityDate(lines []string) (string, error) {
	for _, line := range lines {
		match := dateRegex.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}

		dt, err := time.Parse("02/01/2006", match[1])
		if err != nil {
			return "", fmt.Errorf("invalid maturity date %q", match[1])
		}
		return dt.Format(time.DateOnly), nil
	}
	return "", nil
}

func extractModalityAndIndex(lines []string, investmentType string) (string, string) {
	if investmentType == domain.TypeTesouroSelic {
		return domain.ModalityPOS, domain.IndexSELIC
	}

	for _, line := range lines {
		upper := strings.ToUpper(line)

		if strings.Contains(upper, "PRÉ-FIXADO") || strings.Contains(upper, "PRE-FIXADO") {
			return domain.ModalityPRE, domain.IndexPrefixado
		}
		if strings.Contains(upper, "PÓS-FIXADO") || strings.Contains(upper, "POS-FIXADO") {
			return domain.ModalityPOS, domain.IndexCDI
		}
		if strings.Contains(upper, "IPCA") {
			return domain.ModalityIPCA, domain.IndexIPCA
		}
	}

	switch investmentType {
	case domain.TypeTesouroPrefixado:
		return domain.ModalityPRE, domain.IndexPrefixado
	case domain.TypeTesouroIPCA:
		return domain.ModalityIPCA, domain.IndexIPCA
	default:
		return domain.ModalityPOS, domain.IndexCDI
	}
}
