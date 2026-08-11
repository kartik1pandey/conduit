// Command generate-invoices is Checkpoint 4.2's "scheduled job": it turns a
// billing period's recorded usage into invoices. It's a plain CLI command,
// not an in-process scheduler — the actual "run this monthly" scheduling is
// an external cron job (or a PaaS's own scheduled-task feature) invoking
// this binary, which is the boring, well-understood way to run a periodic
// batch job without pulling in a scheduling library for something a
// platform-level cron already does.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"time"

	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/billing"
	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/config"
	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/db"
)

func main() {
	periodFlag := flag.String("period", "", "billing period as YYYY-MM (default: the previous calendar month)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	period, err := resolvePeriod(*periodFlag)
	if err != nil {
		log.Fatalf("invalid -period: %v", err)
	}

	store := billing.NewStore(pool)
	merchantIDs, err := store.AllMerchantsWithUsage(ctx, period)
	if err != nil {
		log.Fatalf("listing merchants with usage for %s: %v", period.Format("2006-01"), err)
	}

	log.Printf("generating invoices for %s: %d merchant(s) with recorded usage", period.Format("2006-01"), len(merchantIDs))

	for _, merchantID := range merchantIDs {
		callCount, err := store.UsageForPeriod(ctx, merchantID, period)
		if err != nil {
			log.Printf("merchant %s: could not read usage: %v", merchantID, err)
			continue
		}

		total := billing.CalculateTotal(callCount, billing.DefaultTiers)

		_, err = store.CreateInvoice(ctx, merchantID, period, callCount, total)
		switch {
		case errors.Is(err, billing.ErrAlreadyInvoiced):
			log.Printf("merchant %s: already invoiced for %s, skipping", merchantID, period.Format("2006-01"))
		case err != nil:
			log.Printf("merchant %s: could not create invoice: %v", merchantID, err)
		default:
			log.Printf("merchant %s: invoiced %d calls -> $%s", merchantID, callCount, total.StringFixed(2))
		}
	}
}

// resolvePeriod defaults to the previous calendar month — you invoice a
// completed billing period, never the one still in progress, so
// generate-invoices run with no flag at all (e.g. from a "1st of the
// month" cron entry) bills the month that just ended.
func resolvePeriod(flagValue string) (time.Time, error) {
	if flagValue == "" {
		now := time.Now().UTC()
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return firstOfThisMonth.AddDate(0, -1, 0), nil
	}
	return time.Parse("2006-01", flagValue)
}
