package billing

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Every expected value here is hand-calculated, not derived from running
// the code — this is exactly what Checkpoint 4.2 asks to be verified
// against.
func TestCalculateTotal_HandCalculatedCases(t *testing.T) {
	tests := []struct {
		name      string
		callCount int64
		want      string
	}{
		{"zero calls", 0, "0.00"},
		{"within free tier", 500, "0.00"},
		{"exactly at free tier boundary", 1_000, "0.00"},
		// 1,000 free + 1 call @ $0.01
		{"one call past the free tier", 1_001, "0.01"},
		// 1,000 free + 9,000 @ $0.01 = $90.00
		{"exactly at second tier boundary", 10_000, "90.00"},
		// 1,000 free + 9,000 @ $0.01 ($90.00) + 5,000 @ $0.005 ($25.00) = $115.00
		{"spanning all three tiers", 15_000, "115.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := decimal.NewFromString(tt.want)
			require.NoError(t, err)

			got := CalculateTotal(tt.callCount, DefaultTiers)
			require.True(t, got.Equal(want), "CalculateTotal(%d) = %s, want %s", tt.callCount, got, want)
		})
	}
}

func TestCalculateTotal_NeverGoesNegative(t *testing.T) {
	got := CalculateTotal(1, DefaultTiers)
	require.True(t, got.GreaterThanOrEqual(decimal.Zero))
}
