package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"portmon/internal/storage"
)

func printRows(w io.Writer, rows []storage.SummaryRow) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Port/Group\tName\tOwner\tInbound\tOutbound\tTotal")

	var totalIn uint64
	var totalOut uint64
	for _, row := range rows {
		totalIn += row.BytesIn
		totalOut += row.BytesOut
		fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			row.Key,
			row.Name,
			row.Owner,
			formatBytes(row.BytesIn),
			formatBytes(row.BytesOut),
			formatBytes(row.BytesIn+row.BytesOut),
		)
	}

	fmt.Fprintf(tw, "Total\t\t\t%s\t%s\t%s\n", formatBytes(totalIn), formatBytes(totalOut), formatBytes(totalIn+totalOut))
	_ = tw.Flush()
}

func formatBytes(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	value := float64(bytes)
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", bytes, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
