// Package output provides multiple output format writers for tabular data.
package output

import (
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"strings"
)

// ResultRow represents a single row of MAC lookup results.
type ResultRow struct {
	OrgName      string
	NetworkName  string
	SwitchName   string
	SwitchSerial string
	Port         string
	MAC          string
	LastSeen     string
}

// WriteCSV writes results in CSV format with headers.
func WriteCSV(w io.Writer, rows []ResultRow) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"Org", "Network", "Switch", "Serial", "Port", "MAC", "LastSeen"})
	for _, row := range rows {
		_ = writer.Write([]string{
			row.OrgName, row.NetworkName, row.SwitchName, row.SwitchSerial,
			row.Port, row.MAC, row.LastSeen,
		})
	}
}

// WriteText writes results in plain text table format with aligned columns.
func WriteText(w io.Writer, rows []ResultRow) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results")
		return
	}

	headers := []string{"Org", "Network", "Switch", "Serial", "Port", "MAC", "LastSeen"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		widths[0] = max(widths[0], len(row.OrgName))
		widths[1] = max(widths[1], len(row.NetworkName))
		widths[2] = max(widths[2], len(row.SwitchName))
		widths[3] = max(widths[3], len(row.SwitchSerial))
		widths[4] = max(widths[4], len(row.Port))
		widths[5] = max(widths[5], len(row.MAC))
		widths[6] = max(widths[6], len(row.LastSeen))
	}

	separator := strings.Repeat("-", sum(widths)+len(widths)*3-1)
	fmt.Fprintln(w, separator)
	fmt.Fprintln(w, formatRow(headers, widths))
	fmt.Fprintln(w, separator)
	for _, row := range rows {
		values := []string{row.OrgName, row.NetworkName, row.SwitchName, row.SwitchSerial, row.Port, row.MAC, row.LastSeen}
		fmt.Fprintln(w, formatRow(values, widths))
	}
	fmt.Fprintln(w, separator)
}

// WriteHTML writes results in HTML table format.
func WriteHTML(w io.Writer, rows []ResultRow) {
	fmt.Fprintln(w, "<table>")
	fmt.Fprintln(w, "  <thead>")
	fmt.Fprintln(w, "    <tr>")
	fmt.Fprintln(w, "      <th>Org</th><th>Network</th><th>Switch</th><th>Serial</th><th>Port</th><th>MAC</th><th>Last Seen</th>")
	fmt.Fprintln(w, "    </tr>")
	fmt.Fprintln(w, "  </thead>")
	fmt.Fprintln(w, "  <tbody>")
	for _, row := range rows {
		fmt.Fprintf(w, "    <tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			html.EscapeString(row.OrgName),
			html.EscapeString(row.NetworkName),
			html.EscapeString(row.SwitchName),
			html.EscapeString(row.SwitchSerial),
			html.EscapeString(row.Port),
			html.EscapeString(row.MAC),
			html.EscapeString(row.LastSeen),
		)
	}
	fmt.Fprintln(w, "  </tbody>")
	fmt.Fprintln(w, "</table>")
}

// formatRow formats a row of values with column widths for text table output.
func formatRow(values []string, widths []int) string {
	var parts []string
	for i, v := range values {
		parts = append(parts, fmt.Sprintf("%-*s", widths[i], v))
	}
	return strings.Join(parts, " | ")
}

// sum calculates the sum of integers in a slice.
func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
