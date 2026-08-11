// Package billing's pricing.go computes a tiered invoice total from a raw
// call count — a pure function deliberately kept separate from any
// database/HTTP code specifically so Checkpoint 4.2 ("invoice total
// matches a hand-calculated expected value") can be tested by calling it
// directly with a known call count and comparing against arithmetic done
// by hand, no database or server required.
package billing

import "github.com/shopspring/decimal"

// Tier is one pricing band. UpTo is the cumulative call count this tier's
// band ends at (inclusive); UpTo <= 0 marks the final, unbounded tier that
// absorbs everything past the previous tier's ceiling.
type Tier struct {
	UpTo         int64
	PricePerCall decimal.Decimal
}

// DefaultTiers: the first 1,000 calls/month are free, the next 9,000 are
// $0.01/call, everything beyond that is $0.005/call — mirroring the shape
// of Stripe's own public tiered-pricing documentation (a free tier, then
// declining marginal cost at higher volume), not any specific real pricing
// this project is trying to reproduce.
var DefaultTiers = []Tier{
	{UpTo: 1_000, PricePerCall: decimal.Zero},
	{UpTo: 10_000, PricePerCall: decimal.New(1, -2)}, // $0.01
	{UpTo: 0, PricePerCall: decimal.New(5, -3)},      // $0.005, unbounded
}

// CalculateTotal walks the tiers in order, billing each one only for the
// portion of callCount that actually falls inside its band.
func CalculateTotal(callCount int64, tiers []Tier) decimal.Decimal {
	total := decimal.Zero
	remaining := callCount
	var previousUpTo int64

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		var tierCapacity int64
		if tier.UpTo <= 0 {
			tierCapacity = remaining // the unbounded tier absorbs whatever's left
		} else {
			tierCapacity = tier.UpTo - previousUpTo
		}

		callsInTier := remaining
		if callsInTier > tierCapacity {
			callsInTier = tierCapacity
		}

		total = total.Add(tier.PricePerCall.Mul(decimal.NewFromInt(callsInTier)))
		remaining -= callsInTier
		previousUpTo = tier.UpTo
	}

	return total
}
