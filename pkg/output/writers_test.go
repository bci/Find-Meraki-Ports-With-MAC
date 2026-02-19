package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteCSV(t *testing.T) {
	rows := []ResultRow{
		{
			OrgName:      "Test Org",
			NetworkName:  "Test Network",
			SwitchName:   "test-switch",
			SwitchSerial: "S123",
			Port:         "3",
			MAC:          "00:11:22:33:44:55",
			IP:           "192.168.1.100",
			Hostname:     "test-host",
			LastSeen:     "2026-02-13T10:30:00Z",
		},
	}

	var buf bytes.Buffer
	WriteCSV(&buf, rows)

	output := buf.String()
	if !strings.Contains(output, "Org,Network,Switch,Serial,Port,MAC,IP,Hostname,LastSeen") {
		t.Error("WriteCSV() missing CSV header")
	}
	if !strings.Contains(output, "Test Org,Test Network,test-switch,S123,3,00:11:22:33:44:55,192.168.1.100,test-host,2026-02-13T10:30:00Z") {
		t.Error("WriteCSV() missing expected row data")
	}
}

func TestWriteText(t *testing.T) {
	rows := []ResultRow{
		{
			OrgName:      "Test Org",
			NetworkName:  "Test Network",
			SwitchName:   "test-switch",
			SwitchSerial: "S123",
			Port:         "3",
			MAC:          "00:11:22:33:44:55",
			IP:           "192.168.1.100",
			Hostname:     "test-host",
			LastSeen:     "2026-02-13T10:30:00Z",
		},
	}

	var buf bytes.Buffer
	WriteText(&buf, rows)

	output := buf.String()
	if !strings.Contains(output, "Test Org") {
		t.Error("WriteText() missing org name")
	}
	if !strings.Contains(output, "00:11:22:33:44:55") {
		t.Error("WriteText() missing MAC address")
	}
	if !strings.Contains(output, "192.168.1.100") {
		t.Error("WriteText() missing IP address")
	}
	if !strings.Contains(output, "test-host") {
		t.Error("WriteText() missing hostname")
	}
}

func TestWriteHTML(t *testing.T) {
	rows := []ResultRow{
		{
			OrgName:      "Test Org",
			NetworkName:  "Test Network",
			SwitchName:   "test-switch",
			SwitchSerial: "S123",
			Port:         "3",
			MAC:          "00:11:22:33:44:55",
			IP:           "192.168.1.100",
			Hostname:     "test-host",
			LastSeen:     "2026-02-13T10:30:00Z",
		},
	}

	var buf bytes.Buffer
	WriteHTML(&buf, rows)

	output := buf.String()
	if !strings.Contains(output, "<table>") {
		t.Error("WriteHTML() missing table tag")
	}
	if !strings.Contains(output, "<th>IP</th>") {
		t.Error("WriteHTML() missing IP header")
	}
	if !strings.Contains(output, "<th>Hostname</th>") {
		t.Error("WriteHTML() missing Hostname header")
	}
	if !strings.Contains(output, "Test Org") {
		t.Error("WriteHTML() missing org name")
	}
	if !strings.Contains(output, "00:11:22:33:44:55") {
		t.Error("WriteHTML() missing MAC address")
	}
	if !strings.Contains(output, "192.168.1.100") {
		t.Error("WriteHTML() missing IP address")
	}
	if !strings.Contains(output, "test-host") {
		t.Error("WriteHTML() missing hostname")
	}
}
