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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Find-Meraki-Ports-With-MAC/pkg/meraki"
	"Find-Meraki-Ports-With-MAC/pkg/output"
)

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "first non-empty",
			values: []string{"", "second", "third"},
			want:   "second",
		},
		{
			name:   "all empty",
			values: []string{"", "", ""},
			want:   "",
		},
		{
			name:   "first is non-empty",
			values: []string{"first", "second", "third"},
			want:   "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.values...)
			if got != tt.want {
				t.Errorf("firstNonEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectOrganization(t *testing.T) {
	orgs := []meraki.Organization{
		{ID: "org1", Name: "Test Org 1"},
		{ID: "org2", Name: "Test Org 2"},
	}

	tests := []struct {
		name    string
		orgName string
		wantID  string
		wantErr bool
	}{
		{
			name:    "exact match",
			orgName: "Test Org 1",
			wantID:  "org1",
			wantErr: false,
		},
		{
			name:    "not found",
			orgName: "Non-existent Org",
			wantErr: true,
		},
		{
			name:    "empty with single org",
			orgName: "",
			wantErr: true, // We have 2 orgs, so should error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectOrganization(tt.orgName, orgs)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectOrganization() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.ID != tt.wantID {
				t.Errorf("selectOrganization() = %v, want %v", got.ID, tt.wantID)
			}
		})
	}
}

func TestSelectNetworks(t *testing.T) {
	networks := []meraki.Network{
		{ID: "net1", Name: "Network 1"},
		{ID: "net2", Name: "Network 2"},
		{ID: "net3", Name: "Network 3"},
	}

	tests := []struct {
		name        string
		networkName string
		wantCount   int
		wantErr     bool
	}{
		{
			name:        "ALL networks",
			networkName: "ALL",
			wantCount:   3,
			wantErr:     false,
		},
		{
			name:        "specific network",
			networkName: "Network 1",
			wantCount:   1,
			wantErr:     false,
		},
		{
			name:        "not found",
			networkName: "Non-existent",
			wantCount:   0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectNetworks(tt.networkName, networks)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectNetworks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantCount {
				t.Errorf("selectNetworks() returned %d networks, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestAddResult(t *testing.T) {
	index := make(map[string]struct{})
	var results []output.ResultRow

	row1 := output.ResultRow{
		SwitchSerial: "S1",
		Port:         "3",
		MAC:          "00:11:22:33:44:55",
		LastSeen:     "2026-02-13T10:30:00Z",
	}

	// Add first time
	addResult(index, &results, row1)
	if len(results) != 1 {
		t.Errorf("addResult() first add: got %d results, want 1", len(results))
	}

	// Add duplicate
	addResult(index, &results, row1)
	if len(results) != 1 {
		t.Errorf("addResult() duplicate: got %d results, want 1", len(results))
	}

	// Add different MAC
	row2 := row1
	row2.MAC = "00:11:22:33:44:56"
	addResult(index, &results, row2)
	if len(results) != 2 {
		t.Errorf("addResult() different MAC: got %d results, want 2", len(results))
	}
}

func TestResolveHostname(t *testing.T) {
	// Test with empty IP
	hostname, err := meraki.ResolveHostname("")
	if hostname != "" || err != nil {
		t.Errorf("ResolveHostname(\"\") = (%q, %v), want (\"\", nil)", hostname, err)
	}

	// Test with invalid IP (should not panic, just return error)
	hostname, _ = meraki.ResolveHostname("invalid")
	if hostname != "" {
		t.Errorf("ResolveHostname(\"invalid\") returned hostname %q, expected empty", hostname)
	}
	// Note: err might be nil for invalid IPs that don't cause lookup errors

	// Test with localhost (should work in most environments)
	_, err = meraki.ResolveHostname("127.0.0.1")
	// We don't check the exact result since it depends on system configuration
	// Just ensure it doesn't panic and returns something reasonable
	if err != nil {
		t.Logf("ResolveHostname(\"127.0.0.1\") returned error: %v", err)
	}
}

// TestParseAggrPort is kept here for backward compat; the function now lives in resolve.go.
// Additional parseAggrPort and isPortUplink tests are in resolve_test.go.
func TestParseAggrPort(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantClean  string
		wantMember []string
	}{
		{
			name:       "plain port unchanged",
			raw:        "42",
			wantClean:  "42",
			wantMember: nil,
		},
		{
			name:       "AGGR without member list",
			raw:        "AGGR/1",
			wantClean:  "AGGR/1",
			wantMember: nil,
		},
		{
			name:       "AGGR with embedded member list",
			raw:        "AGGR/0=98:18:88:63:BA:37/49,98:18:88:63:BA:37/50,98:18:88:63:BA:37/52",
			wantClean:  "AGGR/0",
			wantMember: []string{"49", "50", "52"},
		},
		{
			name:       "AGGR single member",
			raw:        "AGGR/2=Q2HP-ABCD/23",
			wantClean:  "AGGR/2",
			wantMember: []string{"23"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clean, members := parseAggrPort(tc.raw)
			if clean != tc.wantClean {
				t.Errorf("parseAggrPort(%q) clean = %q, want %q", tc.raw, clean, tc.wantClean)
			}
			if len(members) != len(tc.wantMember) {
				t.Errorf("parseAggrPort(%q) members = %v, want %v", tc.raw, members, tc.wantMember)
				return
			}
			for i := range members {
				if members[i] != tc.wantMember[i] {
					t.Errorf("parseAggrPort(%q) members[%d] = %q, want %q", tc.raw, i, members[i], tc.wantMember[i])
				}
			}
		})
	}
}

// ── firstNonZeroInt ───────────────────────────────────────────────────────────

func TestFirstNonZeroInt(t *testing.T) {
	tests := []struct {
		name   string
		values []int
		want   int
	}{
		{"first non-zero", []int{0, 0, 5}, 5},
		{"all zero", []int{0, 0, 0}, 0},
		{"first is non-zero", []int{3, 0, 9}, 3},
		{"single non-zero", []int{7}, 7},
		{"empty", []int{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonZeroInt(tt.values...)
			if got != tt.want {
				t.Errorf("firstNonZeroInt(%v) = %d, want %d", tt.values, got, tt.want)
			}
		})
	}
}

// ── parseIntEnv ───────────────────────────────────────────────────────────────

func TestParseIntEnv(t *testing.T) {
	t.Setenv("TEST_INT_ENV_42", "42")
	t.Setenv("TEST_INT_ENV_BAD", "notanint")
	t.Setenv("TEST_INT_ENV_EMPTY", "")

	tests := []struct {
		key  string
		want int
	}{
		{"TEST_INT_ENV_42", 42},
		{"TEST_INT_ENV_BAD", 0},
		{"TEST_INT_ENV_EMPTY", 0},
		{"TEST_INT_ENV_UNSET", 0},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := parseIntEnv(tt.key)
			if got != tt.want {
				t.Errorf("parseIntEnv(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

// ── resolveEnvFile ────────────────────────────────────────────────────────────

func TestResolveEnvFile_Default(t *testing.T) {
	// With no --env in os.Args (test runner args won't contain it) the default
	// should be ~/.env.find-mac
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}
	want := filepath.Join(home, ".env.find-mac")
	got := resolveEnvFile()
	if got != want {
		t.Errorf("resolveEnvFile() = %q, want %q", got, want)
	}
}

func TestResolveEnvFile_FlagEquals(t *testing.T) {
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"cmd", "--env=/tmp/custom.env"}

	got := resolveEnvFile()
	if got != "/tmp/custom.env" {
		t.Errorf("resolveEnvFile() = %q, want /tmp/custom.env", got)
	}
}

func TestResolveEnvFile_FlagSpace(t *testing.T) {
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"cmd", "--env", "/tmp/spaced.env"}

	got := resolveEnvFile()
	if got != "/tmp/spaced.env" {
		t.Errorf("resolveEnvFile() = %q, want /tmp/spaced.env", got)
	}
}

// ── printVersion ─────────────────────────────────────────────────────────────

func TestPrintVersion(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printVersion(w)
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	for _, want := range []string{"Find-Meraki-Ports-With-MAC", "Copyright", "GNU General Public License", "https://www.gnu.org/licenses"} {
		if !strings.Contains(out, want) {
			t.Errorf("printVersion() output missing %q\nfull output:\n%s", want, out)
		}
	}
}

// ── printUsage ────────────────────────────────────────────────────────────────

func TestPrintUsage(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printUsage(w)
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	for _, want := range []string{"--mac", "--env", "--interactive", "MERAKI_API_KEY", "Examples"} {
		if !strings.Contains(out, want) {
			t.Errorf("printUsage() output missing %q", want)
		}
	}
}

// ── writeOrganizations / writeNetworksForOrg ──────────────────────────────────

func TestWriteOrganizations(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orgs := []meraki.Organization{
		{ID: "o1", Name: "Acme"},
		{ID: "o2", Name: "Globex"},
	}
	writeOrganizations(w, orgs)
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	for _, want := range []string{"Acme", "o1", "Globex", "o2"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeOrganizations() output missing %q\nfull:\n%s", want, out)
		}
	}
}

func TestWriteNetworksForOrg(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	org := meraki.Organization{ID: "o1", Name: "Acme"}
	nets := []meraki.Network{
		{ID: "n1", Name: "HQ"},
		{ID: "n2", Name: "Branch"},
	}
	writeNetworksForOrg(w, org, nets)
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	for _, want := range []string{"Acme", "HQ", "n1", "Branch", "n2"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeNetworksForOrg() output missing %q\nfull:\n%s", want, out)
		}
	}
}

func TestWriteNetworksForOrg_Empty(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	writeNetworksForOrg(w, meraki.Organization{ID: "o1", Name: "Acme"}, nil)
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "no networks") {
		t.Errorf("writeNetworksForOrg() with no networks should say 'no networks', got:\n%s", out)
	}
}

// ── OUI cache ─────────────────────────────────────────────────────────────────

func TestGetManufacturer_CacheHit(t *testing.T) {
	// Pre-populate the cache so no HTTP call is made
	ouiCache.Store("AA:BB:CC", "TestVendor")
	got := getManufacturer("aa:bb:cc:dd:ee:ff")
	if got != "TestVendor" {
		t.Errorf("getManufacturer() = %q, want TestVendor (cache hit)", got)
	}
}

func TestLookupOUI_EmptyMAC(t *testing.T) {
	got := lookupOUI("")
	if got != "" {
		t.Errorf("lookupOUI(\"\") = %q, want \"\"", got)
	}
}

func TestLookupOUI_ShortMAC(t *testing.T) {
	got := lookupOUI("AA:BB")
	if got != "" {
		t.Errorf("lookupOUI(\"AA:BB\") = %q, want \"\" (too short)", got)
	}
}
