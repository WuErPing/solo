package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/usage/provider"
)

func fptr(v float64) *float64 { return &v }

func TestTable(t *testing.T) {
	resetAt := time.Now().Add(3 * time.Hour)
	snaps := []*provider.Snapshot{
		{
			Provider: "kimi",
			Quotas: []provider.Quota{
				{Name: "weekly_usage", Label: "Weekly Usage", Used: fptr(20), Limit: fptr(100), UsedPct: fptr(20), ResetAt: &resetAt, ResetIn: "3h 0m"},
				{Name: "parallel", Label: "Parallel Limit", Limit: fptr(20)},
			},
		},
		{
			Provider: "deepseek",
			Quotas: []provider.Quota{
				{Name: "balance_cny", Label: "Balance (CNY)", Used: fptr(9.9), Unit: "CNY"},
			},
		},
	}

	var buf bytes.Buffer
	Table(&buf, snaps)
	out := buf.String()

	for _, want := range []string{
		"PROVIDER", "QUOTA", "USED%", "USED/LIMIT", "RESETS IN",
		"kimi", "Weekly Usage", "20.0%", "20/100", "3h 0m",
		"Parallel Limit", "-/20",
		"deepseek", "Balance (CNY)", "9.90 CNY",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatNum(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{100, "100"},
		{9.9, "9.90"},
		{0.5, "0.50"},
		{0, "0"},
	}
	for _, tc := range cases {
		if got := formatNum(tc.in); got != tc.want {
			t.Errorf("formatNum(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
