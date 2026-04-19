package limits

import "testing"

func TestDelta(t *testing.T) {
	cases := []struct {
		name       string
		prev, curr int64
		want       int64
	}{
		{"monotonic_growth", 100, 250, 150},
		{"no_change", 42, 42, 0},
		{"counter_reset_to_zero", 500, 0, 0},
		{"counter_reset_to_small", 500, 30, 30},
		{"both_zero", 0, 0, 0},
		{"zero_to_value", 0, 200, 200},
		{"negative_prev_is_ignored", -1, 100, 0},
		{"negative_curr_is_ignored", 10, -1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Delta(tc.prev, tc.curr); got != tc.want {
				t.Fatalf("Delta(%d,%d)=%d want %d", tc.prev, tc.curr, got, tc.want)
			}
		})
	}
}

func TestUsagePercent(t *testing.T) {
	cases := []struct {
		name        string
		used, limit int64
		want        int
	}{
		{"half", 50, 100, 50},
		{"over_limit_clamped", 10_000, 100, 999},
		{"zero_limit_returns_zero", 42, 0, 0},
		{"negative_limit_returns_zero", 42, -10, 0},
		{"zero_used", 0, 100, 0},
		{"exact_80_percent", 80, 100, 80},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := UsagePercent(tc.used, tc.limit); got != tc.want {
				t.Fatalf("UsagePercent(%d,%d)=%d want %d", tc.used, tc.limit, got, tc.want)
			}
		})
	}
}

func TestClassifyQuota(t *testing.T) {
	// limit=100; 80%=80, 100%=100
	cases := []struct {
		name          string
		oldU, newU, l int64
		want          QuotaEvent
	}{
		{"still_under_80", 50, 70, 100, QuotaNone},
		{"just_crossed_80", 70, 85, 100, QuotaEighty},
		{"crossed_80_edge_inclusive", 79, 80, 100, QuotaEighty},
		{"already_past_80_not_crossing", 85, 95, 100, QuotaNone},
		{"just_crossed_100", 95, 105, 100, QuotaFull},
		{"crossed_both_reports_full", 50, 150, 100, QuotaFull},
		{"no_limit_returns_none", 9999, 99999, 0, QuotaNone},
		{"exact_100_is_full", 99, 100, 100, QuotaFull},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyQuota(tc.oldU, tc.newU, tc.l); got != tc.want {
				t.Fatalf("ClassifyQuota(%d,%d,%d)=%v want %v",
					tc.oldU, tc.newU, tc.l, got, tc.want)
			}
		})
	}
}
