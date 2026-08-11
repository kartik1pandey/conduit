package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolvePeriod_ExplicitFlag(t *testing.T) {
	period, err := resolvePeriod("2026-03")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), period)
}

func TestResolvePeriod_DefaultsToPreviousMonth(t *testing.T) {
	period, err := resolvePeriod("")
	require.NoError(t, err)

	now := time.Now().UTC()
	wantMonth := now.Month() - 1
	wantYear := now.Year()
	if wantMonth == 0 {
		wantMonth = time.December
		wantYear--
	}
	require.Equal(t, time.Date(wantYear, wantMonth, 1, 0, 0, 0, 0, time.UTC), period)
}

func TestResolvePeriod_RejectsInvalidFormat(t *testing.T) {
	_, err := resolvePeriod("not-a-period")
	require.Error(t, err)
}
