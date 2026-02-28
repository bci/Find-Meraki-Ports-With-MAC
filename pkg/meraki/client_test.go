package meraki

import (
	"testing"
)

// ---------------------------------------------------------------------------
// SetHostOverrides
// ---------------------------------------------------------------------------

func TestSetHostOverrides_Empty(t *testing.T) {
	SetHostOverrides("")
	if hostOverrides != nil {
		t.Error("SetHostOverrides(\"\") should leave hostOverrides nil")
	}
}

func TestSetHostOverrides_InvalidJSON(t *testing.T) {
	SetHostOverrides("not json at all")
	if hostOverrides != nil {
		t.Error("SetHostOverrides with invalid JSON should leave hostOverrides nil")
	}
}

func TestSetHostOverrides_MissingIPOrHostname(t *testing.T) {
	// Entries without ip or hostname should be silently skipped.
	SetHostOverrides(`[{"org":"Acme","net":"HQ","ip":"","hostname":"gw"}]`)
	if hostOverrides != nil {
		t.Error("entry with empty ip should produce nil map")
	}
	SetHostOverrides(`[{"org":"Acme","net":"HQ","ip":"192.168.1.1","hostname":""}]`)
	if hostOverrides != nil {
		t.Error("entry with empty hostname should produce nil map")
	}
}

func TestSetHostOverrides_SingleEntry(t *testing.T) {
	SetHostOverrides(`[{"org":"Acme","net":"HQ","ip":"192.168.1.1","hostname":"gateway"}]`)
	if hostOverrides == nil {
		t.Fatal("hostOverrides should not be nil after valid input")
	}
	scope := "Acme/HQ"
	if m, ok := hostOverrides[scope]; !ok {
		t.Errorf("expected scope %q in hostOverrides", scope)
	} else if hn := m["192.168.1.1"]; hn != "gateway" {
		t.Errorf("hostOverrides[%q][%q] = %q, want %q", scope, "192.168.1.1", hn, "gateway")
	}
}

func TestSetHostOverrides_OmittedOrgNetDefaultToWildcard(t *testing.T) {
	SetHostOverrides(`[{"ip":"10.0.0.1","hostname":"mgmt"}]`)
	if hostOverrides == nil {
		t.Fatal("hostOverrides should not be nil")
	}
	scope := "*/*"
	if m, ok := hostOverrides[scope]; !ok {
		t.Errorf("omitted org/net should produce scope %q", scope)
	} else if hn := m["10.0.0.1"]; hn != "mgmt" {
		t.Errorf("hostOverrides[%q][%q] = %q, want %q", scope, "10.0.0.1", hn, "mgmt")
	}
}

func TestSetHostOverrides_OmittedNetDefaultsToWildcard(t *testing.T) {
	SetHostOverrides(`[{"org":"Acme","ip":"10.0.0.1","hostname":"core"}]`)
	scope := "Acme/*"
	if m, ok := hostOverrides[scope]; !ok {
		t.Errorf("omitted net should produce scope %q", scope)
	} else if hn := m["10.0.0.1"]; hn != "core" {
		t.Errorf("hostOverrides[%q][%q] = %q, want %q", scope, "10.0.0.1", hn, "core")
	}
}

func TestSetHostOverrides_MultiEntry(t *testing.T) {
	SetHostOverrides(`[
		{"org":"Acme","net":"HQ","ip":"192.168.1.1","hostname":"hq-gw"},
		{"org":"Acme","net":"*",  "ip":"10.0.0.1",  "hostname":"core"},
		{"org":"*",   "net":"*",  "ip":"172.16.0.1","hostname":"global"}
	]`)
	if hostOverrides == nil {
		t.Fatal("hostOverrides should not be nil")
	}
	cases := []struct{ scope, ip, want string }{
		{"Acme/HQ", "192.168.1.1", "hq-gw"},
		{"Acme/*", "10.0.0.1", "core"},
		{"*/*", "172.16.0.1", "global"},
	}
	for _, c := range cases {
		m, ok := hostOverrides[c.scope]
		if !ok {
			t.Errorf("expected scope %q", c.scope)
			continue
		}
		if hn := m[c.ip]; hn != c.want {
			t.Errorf("hostOverrides[%q][%q] = %q, want %q", c.scope, c.ip, hn, c.want)
		}
	}
}

func TestSetHostOverrides_ClearsOldState(t *testing.T) {
	SetHostOverrides(`[{"ip":"1.2.3.4","hostname":"old"}]`)
	SetHostOverrides("") // should clear
	if hostOverrides != nil {
		t.Error("second call with empty string should clear hostOverrides")
	}
}

// ---------------------------------------------------------------------------
// LookupHostOverride — priority order
// ---------------------------------------------------------------------------

// setupOverrides is a helper that loads a fixed multi-scope set for lookup tests.
func setupOverrides(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetHostOverrides("") })
	SetHostOverrides(`[
		{"org":"Acme","net":"HQ",  "ip":"192.168.1.1","hostname":"exact"},
		{"org":"Acme","net":"*",   "ip":"192.168.1.1","hostname":"org-wildcard"},
		{"org":"*",   "net":"HQ",  "ip":"192.168.1.1","hostname":"net-wildcard"},
		{"org":"*",   "net":"*",   "ip":"192.168.1.1","hostname":"global"},
		{"org":"Acme","net":"*",   "ip":"10.0.0.1",   "hostname":"core"}
	]`)
}

func TestLookupHostOverride_NilMap(t *testing.T) {
	SetHostOverrides("")
	if hn := LookupHostOverride("192.168.1.1", "Acme", "HQ"); hn != "" {
		t.Errorf("expected empty string with nil map, got %q", hn)
	}
}

func TestLookupHostOverride_ExactMatch(t *testing.T) {
	setupOverrides(t)
	if hn := LookupHostOverride("192.168.1.1", "Acme", "HQ"); hn != "exact" {
		t.Errorf("exact org/net should return %q, got %q", "exact", hn)
	}
}

func TestLookupHostOverride_OrgWildcard(t *testing.T) {
	// Load only org-wildcard and global entries so exact doesn't win.
	t.Cleanup(func() { SetHostOverrides("") })
	SetHostOverrides(`[
		{"org":"Acme","net":"*","ip":"192.168.1.1","hostname":"org-wildcard"},
		{"org":"*",   "net":"*","ip":"192.168.1.1","hostname":"global"}
	]`)
	if hn := LookupHostOverride("192.168.1.1", "Acme", "Branch"); hn != "org-wildcard" {
		t.Errorf("org/* should return %q, got %q", "org-wildcard", hn)
	}
}

func TestLookupHostOverride_NetWildcard(t *testing.T) {
	// Only net-wildcard and global — org doesn't match exactly.
	t.Cleanup(func() { SetHostOverrides("") })
	SetHostOverrides(`[
		{"org":"*",  "net":"HQ","ip":"192.168.1.1","hostname":"net-wildcard"},
		{"org":"*",  "net":"*", "ip":"192.168.1.1","hostname":"global"}
	]`)
	if hn := LookupHostOverride("192.168.1.1", "OtherOrg", "HQ"); hn != "net-wildcard" {
		t.Errorf("*/net should return %q, got %q", "net-wildcard", hn)
	}
}

func TestLookupHostOverride_GlobalFallback(t *testing.T) {
	t.Cleanup(func() { SetHostOverrides("") })
	SetHostOverrides(`[{"org":"*","net":"*","ip":"192.168.1.1","hostname":"global"}]`)
	if hn := LookupHostOverride("192.168.1.1", "Any", "Any"); hn != "global" {
		t.Errorf("*/* should return %q, got %q", "global", hn)
	}
}

func TestLookupHostOverride_NoMatch(t *testing.T) {
	setupOverrides(t)
	if hn := LookupHostOverride("9.9.9.9", "Acme", "HQ"); hn != "" {
		t.Errorf("unknown IP should return empty, got %q", hn)
	}
}

func TestLookupHostOverride_OrgIsolation(t *testing.T) {
	// Entry scoped to "Acme/*" should NOT match a different org.
	t.Cleanup(func() { SetHostOverrides("") })
	SetHostOverrides(`[{"org":"Acme","net":"*","ip":"10.0.0.1","hostname":"acme-core"}]`)
	if hn := LookupHostOverride("10.0.0.1", "Other", "HQ"); hn != "" {
		t.Errorf("Acme-scoped entry should not match org=Other, got %q", hn)
	}
}

func TestLookupHostOverride_Priority(t *testing.T) {
	// Verify the full priority chain: exact > org/* > */net > */*
	setupOverrides(t)

	// 1. exact wins over everything
	if hn := LookupHostOverride("192.168.1.1", "Acme", "HQ"); hn != "exact" {
		t.Errorf("priority 1 (exact): got %q, want %q", hn, "exact")
	}

	// 2. with only org-wildcard + net-wildcard + global loaded, org/* wins
	t.Cleanup(func() { SetHostOverrides("") })
	SetHostOverrides(`[
		{"org":"Acme","net":"*","ip":"1.1.1.1","hostname":"org-wins"},
		{"org":"*",   "net":"X","ip":"1.1.1.1","hostname":"net-wins"},
		{"org":"*",   "net":"*","ip":"1.1.1.1","hostname":"global-wins"}
	]`)
	if hn := LookupHostOverride("1.1.1.1", "Acme", "X"); hn != "org-wins" {
		t.Errorf("priority 2 (org/*): got %q, want %q", hn, "org-wins")
	}

	// 3. with only net-wildcard + global, */net wins
	SetHostOverrides(`[
		{"org":"*","net":"X","ip":"1.1.1.1","hostname":"net-wins"},
		{"org":"*","net":"*","ip":"1.1.1.1","hostname":"global-wins"}
	]`)
	if hn := LookupHostOverride("1.1.1.1", "Other", "X"); hn != "net-wins" {
		t.Errorf("priority 3 (*/net): got %q, want %q", hn, "net-wins")
	}

	// 4. only global matches
	SetHostOverrides(`[{"org":"*","net":"*","ip":"1.1.1.1","hostname":"global-wins"}]`)
	if hn := LookupHostOverride("1.1.1.1", "Other", "Y"); hn != "global-wins" {
		t.Errorf("priority 4 (*/*): got %q, want %q", hn, "global-wins")
	}
}
