package main

import (
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
