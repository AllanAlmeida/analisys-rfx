package utils

import "testing"

func TestRound(t *testing.T) {
	got := Round(1.236, 2)
	if got != 1.24 {
		t.Fatalf("expected 1.24, got %v", got)
	}
}

func TestEquivalentCDBForTaxFree(t *testing.T) {
	got := Round(EquivalentCDBForTaxFree(95), 2)
	if got != 111.76 {
		t.Fatalf("expected 111.76, got %v", got)
	}
}

func TestNominalRateFromIPCAPlus(t *testing.T) {
	got := Round(NominalRateFromIPCAPlus(4.5, 6), 2)
	if got != 10.77 {
		t.Fatalf("expected 10.77, got %v", got)
	}
}

func TestRealReturn(t *testing.T) {
	got := Round(RealReturn(12, 4.5), 2)
	if got != 7.18 {
		t.Fatalf("expected 7.18, got %v", got)
	}
}
