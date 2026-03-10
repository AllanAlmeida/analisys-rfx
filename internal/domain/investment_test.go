package domain

import (
	"errors"
	"testing"
)

func TestAnalyzeInvestmentRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     AnalyzeInvestmentRequest
		wantErr error
	}{
		{
			name: "valid cdb defaults index to CDI",
			req: AnalyzeInvestmentRequest{
				Type: "CDB",
				Rate: 110,
			},
		},
		{
			name: "missing type",
			req: AnalyzeInvestmentRequest{
				Rate: 100,
			},
			wantErr: ErrTypeRequired,
		},
		{
			name: "invalid rate",
			req: AnalyzeInvestmentRequest{
				Type: "LCI",
				Rate: 0,
			},
			wantErr: ErrRateInvalid,
		},
		{
			name: "invalid index",
			req: AnalyzeInvestmentRequest{
				Type:  "LCA",
				Rate:  90,
				Index: "IPCA",
			},
			wantErr: ErrInvalidIndex,
		},
		{
			name: "unsupported type",
			req: AnalyzeInvestmentRequest{
				Type:  "CRI",
				Rate:  100,
				Index: "CDI",
			},
			wantErr: ErrUnsupportedType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			req.Normalize()
			err := req.Validate()
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}
