// Copyright (C) 2025 Kent Behrends
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	AggrPorts    []string // member ports when Port is a link-aggregation (AGGR/*) port
	MAC          string
	LastSeen     string
	IP           string
	Hostname     string
	VLAN         int
	PortMode     string // "access", "trunk", or ""
	IsUplink     bool   // true when port appears in link-layer topology as an inter-device link
}

// aggrPortsStr returns the AggrPorts as a comma-separated string, or empty string if none.
func aggrPortsStr(row ResultRow) string {
	if len(row.AggrPorts) == 0 {
		return ""
	}
	return strings.Join(row.AggrPorts, ", ")
}

// WriteCSV writes results in CSV format with headers.
func WriteCSV(w io.Writer, rows []ResultRow) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"Org", "Network", "Switch", "Serial", "Port", "AggrPorts", "MAC", "IP", "Hostname", "LastSeen", "Uplink"})
	for _, row := range rows {
		uplinkStr := ""
		if row.IsUplink {
			uplinkStr = "yes"
		}
		_ = writer.Write([]string{
			row.OrgName, row.NetworkName, row.SwitchName, row.SwitchSerial,
			row.Port, aggrPortsStr(row), row.MAC, row.IP, row.Hostname, row.LastSeen, uplinkStr,
		})
	}
}

// WriteText writes results in plain text table format with aligned columns.
func WriteText(w io.Writer, rows []ResultRow) {
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(w, "No results")
		return
	}

	headers := []string{"Org", "Network", "Switch", "Serial", "Port", "AggrPorts", "MAC", "IP", "Hostname", "LastSeen", "Uplink"}
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
		widths[5] = max(widths[5], len(aggrPortsStr(row)))
		widths[6] = max(widths[6], len(row.MAC))
		widths[7] = max(widths[7], len(row.IP))
		widths[8] = max(widths[8], len(row.Hostname))
		widths[9] = max(widths[9], len(row.LastSeen))
		// widths[10] is "Uplink"/"yes"/"" — max is len("Uplink")=6
	}

	separator := strings.Repeat("-", sum(widths)+len(widths)*3-1)
	_, _ = fmt.Fprintln(w, separator)
	_, _ = fmt.Fprintln(w, formatRow(headers, widths))
	_, _ = fmt.Fprintln(w, separator)
	for _, row := range rows {
		uplinkStr := ""
		if row.IsUplink {
			uplinkStr = "yes"
		}
		values := []string{row.OrgName, row.NetworkName, row.SwitchName, row.SwitchSerial, row.Port, aggrPortsStr(row), row.MAC, row.IP, row.Hostname, row.LastSeen, uplinkStr}
		_, _ = fmt.Fprintln(w, formatRow(values, widths))
	}
	_, _ = fmt.Fprintln(w, separator)
}

// WriteHTML writes results in HTML table format.
func WriteHTML(w io.Writer, rows []ResultRow) {
	_, _ = fmt.Fprintln(w, "<table>")
	_, _ = fmt.Fprintln(w, "  <thead>")
	_, _ = fmt.Fprintln(w, "    <tr>")
	_, _ = fmt.Fprintln(w, "      <th>Org</th><th>Network</th><th>Switch</th><th>Serial</th><th>Port</th><th>AggrPorts</th><th>MAC</th><th>IP</th><th>Hostname</th><th>Last Seen</th><th>Uplink</th>")
	_, _ = fmt.Fprintln(w, "    </tr>")
	_, _ = fmt.Fprintln(w, "  </thead>")
	_, _ = fmt.Fprintln(w, "  <tbody>")
	for _, row := range rows {
		uplinkStr := ""
		if row.IsUplink {
			uplinkStr = "yes"
		}
		_, _ = fmt.Fprintf(w, "    <tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			html.EscapeString(row.OrgName),
			html.EscapeString(row.NetworkName),
			html.EscapeString(row.SwitchName),
			html.EscapeString(row.SwitchSerial),
			html.EscapeString(row.Port),
			html.EscapeString(aggrPortsStr(row)),
			html.EscapeString(row.MAC),
			html.EscapeString(row.IP),
			html.EscapeString(row.Hostname),
			html.EscapeString(row.LastSeen),
			html.EscapeString(uplinkStr),
		)
	}
	_, _ = fmt.Fprintln(w, "  </tbody>")
	_, _ = fmt.Fprintln(w, "</table>")
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
