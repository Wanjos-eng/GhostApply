package main

import (
	"context"
	"testing"
)

func TestRunPerformanceSuiteSnapshot(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())

	perf := app.RunPerformanceSuite()

	if perf.Samples <= 0 {
		t.Fatalf("expected samples > 0, got %d", perf.Samples)
	}
	if perf.TotalSuiteMS < 0 {
		t.Fatalf("expected non-negative total suite ms, got %.3f", perf.TotalSuiteMS)
	}

	t.Logf("PERF_RESULT ran_at=%s samples=%d db_p50=%.3fms db_p95=%.3fms db_p99=%.3fms history_p50=%.3fms history_p95=%.3fms history_p99=%.3fms emails_p50=%.3fms emails_p95=%.3fms emails_p99=%.3fms total_p50=%.3fms total_p95=%.3fms total_p99=%.3fms rows_history=%d rows_emails=%d rows_interviews=%d db_reachable=%t",
		perf.RanAt,
		perf.Samples,
		perf.DatabasePingMS,
		perf.DatabasePingP95MS,
		perf.DatabasePingP99MS,
		perf.FetchHistoryMS,
		perf.FetchHistoryP95MS,
		perf.FetchHistoryP99MS,
		perf.FetchEmailsMS,
		perf.FetchEmailsP95MS,
		perf.FetchEmailsP99MS,
		perf.TotalSuiteMS,
		perf.TotalSuiteP95MS,
		perf.TotalSuiteP99MS,
		perf.HistoryRows,
		perf.EmailRows,
		perf.InterviewRows,
		perf.DatabaseReachable,
	)
}
