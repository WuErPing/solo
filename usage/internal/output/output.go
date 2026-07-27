package output

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"text/tabwriter"

	"github.com/WuErPing/solo/usage/provider"
)

func JSON(w io.Writer, snapshots []*provider.Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snapshots)
}

func Table(w io.Writer, snapshots []*provider.Snapshot) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "PROVIDER\tQUOTA\tUSED%%\tUSED/LIMIT\tRESETS IN\n")
	for _, snap := range snapshots {
		for _, q := range snap.Quotas {
			pct := "-"
			if q.UsedPct != nil {
				pct = fmt.Sprintf("%.1f%%", *q.UsedPct)
			}
			usedLimit := "-"
			if q.Used != nil && q.Limit != nil {
				usedLimit = fmt.Sprintf("%s/%s", formatNum(*q.Used), formatNum(*q.Limit))
			} else if q.Used != nil {
				usedLimit = formatNum(*q.Used)
				if q.Unit != "" {
					usedLimit += " " + q.Unit
				}
			} else if q.Limit != nil {
				usedLimit = "-/" + formatNum(*q.Limit)
			}
			resetIn := "-"
			if q.ResetIn != "" {
				resetIn = q.ResetIn
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", snap.Provider, q.Label, pct, usedLimit, resetIn)
		}
	}
	tw.Flush()
}

// formatNum renders integers without decimals and keeps cents for
// fractional amounts (e.g. balances like 9.90).
func formatNum(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}
