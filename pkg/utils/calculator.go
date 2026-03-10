package utils

import "math"

func Round(value float64, precision int) float64 {
	pow := math.Pow(10, float64(precision))
	return math.Round(value*pow) / pow
}

func RealReturn(nominalRate, inflationRate float64) float64 {
	nominal := 1 + nominalRate/100
	inflation := 1 + inflationRate/100
	return ((nominal / inflation) - 1) * 100
}

func EquivalentCDBForTaxFree(rate float64) float64 {
	return rate / 0.85
}

func NominalRateFromIPCAPlus(ipcaRate, realRate float64) float64 {
	ipca := 1 + ipcaRate/100
	real := 1 + realRate/100
	return (ipca*real - 1) * 100
}
