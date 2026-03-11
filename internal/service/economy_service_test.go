package service

import (
	"math"
	"testing"
)

func TestAnnualizePeriodicRate(t *testing.T) {
	tests := []struct {
		name           string
		rate           float64
		periodsPerYear float64
		want           float64
	}{
		{
			name:           "annualizes monthly ipca",
			rate:           0.33,
			periodsPerYear: monthlyPeriodsPerYear,
			want:           4.032670515423953,
		},
		{
			name:           "annualizes daily cdi",
			rate:           0.06,
			periodsPerYear: businessDaysPerYear,
			want:           16.317653888658334,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := annualizePeriodicRate(tc.rate, tc.periodsPerYear)
			if math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestAnnualizePeriodicRateKeepsInvalidFloor(t *testing.T) {
	got := annualizePeriodicRate(-100, monthlyPeriodsPerYear)
	if got != -100 {
		t.Fatalf("expected -100, got %v", got)
	}
}
