package cmd

import (
	"testing"
	"time"
)

func TestReportRangeSortsReversedDates(t *testing.T) {
	oldFrom := reportFrom
	oldTo := reportTo
	oldMonthly := reportMonthly
	t.Cleanup(func() {
		reportFrom = oldFrom
		reportTo = oldTo
		reportMonthly = oldMonthly
	})

	reportFrom = "2026-06-30"
	reportTo = "2026-06-01"
	reportMonthly = false

	from, to, err := reportRange(nil)
	if err != nil {
		t.Fatalf("reportRange returned error: %v", err)
	}

	wantFrom := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	wantTo := time.Date(2026, 6, 30, 0, 0, 0, 0, time.Local)
	if !from.Equal(wantFrom) {
		t.Fatalf("from = %s, want %s", from.Format("2006-01-02"), wantFrom.Format("2006-01-02"))
	}
	if !to.Equal(wantTo) {
		t.Fatalf("to = %s, want %s", to.Format("2006-01-02"), wantTo.Format("2006-01-02"))
	}
}
