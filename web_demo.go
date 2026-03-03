package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ── Demo mode ─────────────────────────────────────────────────────────────────
// Shared constants used by all demo handlers and testDemoResults.
// No API key is required and no real API calls are made.
const (
	demoMAC      = "a4:c3:f0:85:1d:3e"
	demoIP       = "10.10.1.42"
	demoHostname = "laptop-jsmith.acme.local"
	demoOrg      = "Acme Corporation"
	demoMfr      = "Apple"
	demoOUI      = "a4:c3:f0" // real OUI is Intel; we override to Apple in demo
)

func handleTestValidateKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"organizations": []map[string]string{
			{"id": "demo-org-1", "name": demoOrg},
		},
	})
}

func handleTestGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"apiKey":        "demo-key",
		"presetMAC":     firstNonEmpty(webPresetMAC, demoMAC),
		"presetIP":      webPresetIP,
		"presetOrg":     demoOrg,
		"presetNetwork": firstNonEmpty(webPresetNetwork, "ALL"),
		"testData":      true,
	})
}

func handleTestGetManufacturer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mac := r.URL.Query().Get("mac")
	// The demo OUI belongs to Intel in the real registry; return Apple so the
	// Device Lookup panel matches the manufacturer shown in the results table.
	vendor := demoMfr
	if len(mac) >= 8 && strings.ToLower(mac[:8]) != demoOUI {
		vendor = lookupOUI(mac)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"manufacturer": vendor})
}

func handleTestGetNetworks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"networks": []map[string]string{
			{"id": "demo-net-1", "name": "HQ Campus"},
			{"id": "demo-net-2", "name": "Warehouse"},
			{"id": "demo-net-3", "name": "City Parks"},
			{"id": "demo-net-4", "name": "Remote Office"},
		},
	})
}

// testDemoResults returns a realistic set of demo results for a single MAC
// address.  A MAC is burned into hardware — it lives on exactly one access
// port (the switch the device is physically plugged into).  That same MAC
// then appears in the MAC table of every upstream switch as the frame
// travels toward the core, always on an uplink/trunk port.
//
// HQ Campus (3 hops — device lives here):
//
//	laptop → sw-hq-access-ms355  port 12   (access)
//	       → sw-hq-dist-ms450    AGGR/0    (uplink, ports 49+50)
//	       → sw-hq-core-c9300    port 25   (uplink)
//
// Warehouse and City Parks share the same WAN/VPN infrastructure so the
// MAC is visible on their border/uplink ports — uplink hits only, no
// access port, because the device is not physically present there.
func testDemoResults(mac string) []map[string]interface{} {
	if mac == "" {
		mac = demoMAC
	}
	const lastSeen = "2026-03-02T14:23:00Z"
	const vlan = 100
	return []map[string]interface{}{
		// ── HQ Campus layer 1: edge MS355 — device physically plugged in here ─
		{
			"orgName":      demoOrg,
			"networkName":  "HQ Campus",
			"deviceName":   "sw-hq-access-ms355",
			"deviceSerial": "Q2HP-XXXX-0001",
			"port":         "12",
			"aggrPorts":    nil,
			"mac":          mac,
			"ip":           demoIP,
			"hostname":     demoHostname,
			"lastSeen":     lastSeen,
			"manufacturer": demoMfr,
			"vlan":         vlan,
			"portMode":     "access",
			"isUplink":     false,
		},
		// ── HQ Campus layer 2: distribution MS450 — AGGR uplink to core ───────
		{
			"orgName":      demoOrg,
			"networkName":  "HQ Campus",
			"deviceName":   "sw-hq-dist-ms450",
			"deviceSerial": "Q2EK-XXXX-0002",
			"port":         "AGGR/0",
			"aggrPorts":    []string{"49", "50"},
			"mac":          mac,
			"ip":           demoIP,
			"hostname":     demoHostname,
			"lastSeen":     lastSeen,
			"manufacturer": demoMfr,
			"vlan":         vlan,
			"portMode":     "trunk",
			"isUplink":     true,
		},
		// ── HQ Campus layer 3: C9300 core — uplink toward router ──────────────
		{
			"orgName":      demoOrg,
			"networkName":  "HQ Campus",
			"deviceName":   "sw-hq-core-c9300",
			"deviceSerial": "FCW-XXXX-0003",
			"port":         "25",
			"aggrPorts":    nil,
			"mac":          mac,
			"ip":           demoIP,
			"hostname":     demoHostname,
			"lastSeen":     lastSeen,
			"manufacturer": demoMfr,
			"vlan":         vlan,
			"portMode":     "trunk",
			"isUplink":     true,
		},
		// ── Warehouse: uplink-only hit on border MS355 ─────────────────────────
		// The device is not physically here; MAC appears in the forwarding table
		// on the WAN uplink port only (traffic routed back to HQ Campus).
		{
			"orgName":      demoOrg,
			"networkName":  "Warehouse",
			"deviceName":   "sw-wh-border-ms355",
			"deviceSerial": "Q2HP-XXXX-0004",
			"port":         "49",
			"aggrPorts":    nil,
			"mac":          mac,
			"ip":           demoIP,
			"hostname":     demoHostname,
			"lastSeen":     lastSeen,
			"manufacturer": demoMfr,
			"vlan":         vlan,
			"portMode":     "trunk",
			"isUplink":     true,
		},
		// ── City Parks: uplink-only hit on C9300 WAN uplink ───────────────────
		{
			"orgName":      demoOrg,
			"networkName":  "City Parks",
			"deviceName":   "sw-parks-c9300-border",
			"deviceSerial": "FCW-XXXX-0005",
			"port":         "25",
			"aggrPorts":    nil,
			"mac":          mac,
			"ip":           demoIP,
			"hostname":     demoHostname,
			"lastSeen":     lastSeen,
			"manufacturer": demoMfr,
			"vlan":         vlan,
			"portMode":     "trunk",
			"isUplink":     true,
		},
	}
}

func handleTestResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req struct {
		MAC string `json:"mac"`
		IP  string `json:"ip"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	mac := req.MAC
	if mac == "" {
		mac = demoMAC
	}

	// Stream realistic log messages over WebSocket in a goroutine so the HTTP
	// response is returned immediately and the logs appear to trickle in live.
	go func(mac string) {
		log := func(msg string) {
			wsLogHub.broadcast(msg)
			time.Sleep(120 * time.Millisecond)
		}
		log(fmt.Sprintf("[INFO] Starting MAC lookup for %s across all networks", mac))
		log(fmt.Sprintf("[INFO] Fetching organization list from %s", demoOrg))
		log("[INFO] Found 4 networks: HQ Campus, Warehouse, City Parks, Remote Office")
		log(fmt.Sprintf("[INFO] Resolving IP address for MAC %s via ARP table...", mac))
		log(fmt.Sprintf("[INFO] Resolved: %s → %s (hostname: %s)", mac, demoIP, demoHostname))

		log("[INFO] [HQ Campus] Scanning 3 switches...")
		log(fmt.Sprintf("[INFO] [HQ Campus] sw-hq-access-ms355 — MAC %s found on port 12 (access, VLAN 100)", mac))
		log(fmt.Sprintf("[INFO] [HQ Campus] sw-hq-dist-ms450 — MAC %s found on AGGR/0 (uplink, ports 49+50)", mac))
		log(fmt.Sprintf("[INFO] [HQ Campus] sw-hq-core-c9300 — MAC %s found on port 25 (uplink)", mac))

		log("[INFO] [Warehouse] Scanning 2 switches...")
		log("[DEBUG] [Warehouse] sw-wh-c9300-floor — MAC not found")
		log(fmt.Sprintf("[INFO] [Warehouse] sw-wh-border-ms355 — MAC %s found on port 49 (uplink)", mac))

		log("[INFO] [City Parks] Scanning 3 switches...")
		log("[DEBUG] [City Parks] sw-parks-ms355-01 — MAC not found")
		log("[DEBUG] [City Parks] sw-parks-ms355-02 — MAC not found")
		log(fmt.Sprintf("[INFO] [City Parks] sw-parks-c9300-border — MAC %s found on port 25 (uplink)", mac))

		log("[INFO] [Remote Office] Scanning 1 switch...")
		log("[DEBUG] [Remote Office] sw-remote-ms355-01 — MAC not found")

		log(fmt.Sprintf("[INFO] Lookup complete — 5 result(s) found for %s", mac))
	}(mac)

	results := testDemoResults(mac)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}
