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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"Find-Meraki-Ports-With-MAC/pkg/filters"
	"Find-Meraki-Ports-With-MAC/pkg/meraki"
	"Find-Meraki-Ports-With-MAC/pkg/output"
)

func handleValidateKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		APIKey string `json:"apiKey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		http.Error(w, `{"error": "API key is required"}`, http.StatusBadRequest)
		return
	}

	// Create Meraki client with the provided API key
	client := meraki.NewClient(req.APIKey, "", 0)
	ctx := context.Background()

	// Test the API key by fetching organizations
	orgs, err := client.GetOrganizations(ctx)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Invalid API key: %v", err)})
		return
	}

	// Return organizations
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"organizations": orgs,
	})
}

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"apiKey":        webAPIKey,
		"presetMAC":     webPresetMAC,
		"presetIP":      webPresetIP,
		"presetOrg":     webPresetOrgName,
		"presetNetwork": webPresetNetwork,
	})
}

func handleGetNetworks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	orgID := r.URL.Query().Get("orgId")
	apiKey := r.URL.Query().Get("apiKey")

	if orgID == "" || apiKey == "" {
		http.Error(w, `{"error": "Organization ID and API key are required"}`, http.StatusBadRequest)
		return
	}

	client := meraki.NewClient(apiKey, "", 0)
	ctx := context.Background()

	networks, err := client.GetNetworks(ctx, orgID)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to get networks: %v", err)})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"networks": networks,
	})
}

func handleResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		MAC        string   `json:"mac"`
		IP         string   `json:"ip"`
		NetworkID  string   `json:"networkId"`
		NetworkIDs []string `json:"networkIds"`
		OrgID      string   `json:"orgId"`
		APIKey     string   `json:"apiKey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		http.Error(w, `{"error": "API key is required"}`, http.StatusBadRequest)
		return
	}

	// Normalise: if single networkId given, treat as list of one
	networkIDs := req.NetworkIDs
	if len(networkIDs) == 0 && req.NetworkID != "" {
		networkIDs = []string{req.NetworkID}
	}
	if len(networkIDs) == 0 {
		http.Error(w, `{"error": "At least one network ID is required"}`, http.StatusBadRequest)
		return
	}

	if req.MAC == "" && req.IP == "" {
		http.Error(w, `{"error": "MAC address or IP address is required"}`, http.StatusBadRequest)
		return
	}

	// Resolve across all requested networks and aggregate
	var allResults []output.ResultRow
	for _, netID := range networkIDs {
		cfg := Config{
			APIKey:       req.APIKey,
			OrgID:        req.OrgID,
			NetworkName:  netID,
			LogLevel:     "INFO",
			MacTablePoll: firstNonZeroInt(parseIntEnv("MERAKI_MAC_POLL"), 15),
		}
		results, err := resolveDevices(cfg, req.MAC, req.IP)
		if err != nil {
			// Skip networks that error (e.g. not a switch network)
			continue
		}
		allResults = append(allResults, results...)
	}

	// Convert to web-friendly format
	webResults := make([]map[string]interface{}, len(allResults))
	for i, result := range allResults {
		webResults[i] = map[string]interface{}{
			"orgName":      result.OrgName,
			"networkName":  result.NetworkName,
			"deviceName":   result.SwitchName,
			"deviceSerial": result.SwitchSerial,
			"port":         result.Port,
			"aggrPorts":    result.AggrPorts,
			"mac":          result.MAC,
			"ip":           result.IP,
			"hostname":     result.Hostname,
			"lastSeen":     result.LastSeen,
			"manufacturer": getManufacturer(result.MAC),
			"vlan":         result.VLAN,
			"portMode":     result.PortMode,
			"isUplink":     result.IsUplink,
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"results": webResults,
	})
}

func handleGetManufacturer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mac := r.URL.Query().Get("mac")
	vendor := lookupOUI(mac)
	_ = json.NewEncoder(w).Encode(map[string]string{"manufacturer": vendor})
}

// handleTopology serves the D3 force-graph topology page.
// All CSS and JS are loaded from /static/ — the handler only injects
// per-request config values into <meta> tags so topology.js can read them.
func handleTopology(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	networkID := firstNonEmpty(q.Get("networkId"), "unknown")
	orgID := q.Get("orgId")
	apiKey := firstNonEmpty(q.Get("apiKey"), webAPIKey)
	highlightSerial := q.Get("highlightSerial")
	highlightPort := q.Get("highlightPort")
	highlightName := q.Get("highlightName")
	portMode := q.Get("portMode")
	mac := q.Get("mac")
	hostname := q.Get("hostname")

	// html/template would auto-escape, but we're building a small fixed page
	// with no user-controlled content in element bodies — only in meta content
	// attributes, which strings.NewReplacer handles safely here.
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Network Topology</title>
<meta name="topo-network-id"       content="__NETWORK_ID__">
<meta name="topo-org-id"           content="__ORG_ID__">
<meta name="topo-api-key"          content="__API_KEY__">
<meta name="topo-highlight-serial" content="__HIGHLIGHT_SERIAL__">
<meta name="topo-highlight-port"   content="__HIGHLIGHT_PORT__">
<meta name="topo-highlight-name"   content="__HIGHLIGHT_NAME__">
<meta name="topo-port-mode"        content="__PORT_MODE__">
<meta name="topo-highlight-mac"    content="__HIGHLIGHT_MAC__">
<meta name="topo-highlight-host"   content="__HIGHLIGHT_HOSTNAME__">
<link rel="stylesheet" href="/static/css/topology.css">
</head>
<body>
<div id="topbar">
  <h2>Topology</h2>
  <span class="pill" id="netPill">Loading...</span>
  <span class="pill highlight" id="hlPill" style="display:none"></span>
  <span class="spacer"></span>
  <button class="tbtn" id="resetBtn">Reset Zoom</button>
  <button class="tbtn danger" onclick="window.close()">Close</button>
</div>
<div id="canvas"><svg id="svg"><g id="root"></g></svg></div>
<div id="tooltip"></div>
<div id="legend">
  <div class="li"><div class="dot" style="background:#3b82f6"></div>Switch</div>
  <div class="li"><div class="dot" style="background:#a855f7"></div>Wireless AP</div>
  <div class="li"><div class="dot" style="background:#10b981"></div>Appliance/VPN</div>
  <div class="li"><div class="dot" style="background:#ec4899"></div>Camera</div>
  <div class="li"><div class="dot" style="background:#64748b"></div>Other</div>
  <div class="li"><div class="dot" style="background:#f59e0b"></div>Target Switch</div>
  <div class="li" id="pcLegend" style="display:none"><div class="dot sq" style="background:#f59e0b"></div>Found Device</div>
</div>
<script src="https://d3js.org/d3.v7.min.js"></script>
<script src="/static/js/topology.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	page := strings.NewReplacer(
		"__NETWORK_ID__", networkID,
		"__ORG_ID__", orgID,
		"__API_KEY__", apiKey,
		"__HIGHLIGHT_SERIAL__", highlightSerial,
		"__HIGHLIGHT_PORT__", highlightPort,
		"__HIGHLIGHT_NAME__", highlightName,
		"__PORT_MODE__", portMode,
		"__HIGHLIGHT_MAC__", mac,
		"__HIGHLIGHT_HOSTNAME__", hostname,
	).Replace(tmpl)
	_, _ = w.Write([]byte(page))
}

func handleGetTopology(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	networkID := r.URL.Query().Get("networkId")
	apiKey := r.URL.Query().Get("apiKey")
	if apiKey == "" {
		apiKey = webAPIKey
	}
	if networkID == "" || apiKey == "" {
		http.Error(w, `{"error":"networkId and apiKey are required"}`, http.StatusBadRequest)
		return
	}

	client := meraki.NewClient(apiKey, "", 0)

	type outNode struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Type  string `json:"type"`
		Model string `json:"model,omitempty"`
	}
	type outLink struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	type outResponse struct {
		NetworkName string    `json:"networkName"`
		Nodes       []outNode `json:"nodes"`
		Links       []outLink `json:"links"`
	}

	resp := outResponse{NetworkName: networkID}

	topo, err := client.GetNetworkTopology(ctx, networkID)
	if err == nil && topo != nil {
		// Deduplicate nodes by serial / id
		seen := map[string]bool{}
		for _, raw := range topo.Nodes {
			serial, _ := raw["serial"].(string)
			if serial == "" {
				serial, _ = raw["mac"].(string)
			}
			if serial == "" || seen[serial] {
				continue
			}
			seen[serial] = true
			name, _ := raw["name"].(string)
			typ, _ := raw["type"].(string)
			model, _ := raw["model"].(string)
			if name == "" {
				name = serial
			}
			resp.Nodes = append(resp.Nodes, outNode{ID: serial, Name: name, Type: typ, Model: model})
		}
		// Build links from link-layer ends
		for _, link := range topo.Links {
			if len(link.Ends) < 2 {
				continue
			}
			src := link.Ends[0].Device.Serial
			tgt := link.Ends[1].Device.Serial
			if src == "" || tgt == "" {
				continue
			}
			resp.Links = append(resp.Links, outLink{Source: src, Target: tgt})
			// Ensure both ends are in the node list
			if !seen[src] {
				seen[src] = true
				name := firstNonEmpty(link.Ends[0].Device.Name, src)
				resp.Nodes = append(resp.Nodes, outNode{ID: src, Name: name, Type: link.Ends[0].Device.Type})
			}
			if !seen[tgt] {
				seen[tgt] = true
				name := firstNonEmpty(link.Ends[1].Device.Name, tgt)
				resp.Nodes = append(resp.Nodes, outNode{ID: tgt, Name: name, Type: link.Ends[1].Device.Type})
			}
		}
	} else {
		// Fallback: list devices in network as flat nodes (no links)
		devices, devErr := client.GetDevices(ctx, networkID)
		if devErr != nil {
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		for _, d := range devices {
			resp.Nodes = append(resp.Nodes, outNode{
				ID:    d.Serial,
				Name:  firstNonEmpty(d.Name, d.Serial),
				Type:  firstNonEmpty(d.ProductType, d.Model),
				Model: d.Model,
			})
		}
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	// Stub implementation
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"alerts": []}`))
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	// Stub implementation
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"logs": []}`))
}

// handleDebugNetwork dumps raw Meraki API responses for a network to aid in diagnosing
// topology and AGGR port resolution issues.
// Query params: networkId (required), orgId, apiKey
// Returns: raw topology JSON, network link aggregations, per-switch LLDP uplinks and port list.
func handleDebugNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	networkID := q.Get("networkId")
	orgID := q.Get("orgId")
	apiKey := q.Get("apiKey")
	if apiKey == "" {
		apiKey = webAPIKey
	}
	if networkID == "" {
		http.Error(w, `{"error":"networkId required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	client := meraki.NewClient(apiKey, "", 2)

	out := map[string]interface{}{
		"networkId": networkID,
		"orgId":     orgID,
	}

	// 1. Raw topology (shows device-level links but no port IDs on this firmware)
	topoRaw, topoErr := client.GetNetworkTopologyRaw(ctx, networkID)
	if topoErr != nil {
		out["topologyError"] = topoErr.Error()
	} else {
		var topoParsed interface{}
		_ = json.Unmarshal(topoRaw, &topoParsed)
		out["topology"] = topoParsed
	}

	// 2. Network-level link aggregations (correct AGGR member source)
	netLAGs := client.GetNetworkLinkAggregations(ctx, networkID)
	out["networkLinkAggregations"] = netLAGs

	// 3. Per-switch: LLDP/CDP uplinks + raw port list
	devices, devErr := client.GetDevices(ctx, networkID)
	if devErr != nil {
		out["devicesError"] = devErr.Error()
	} else {
		switchData := []map[string]interface{}{}
		for _, dev := range filters.FilterSwitches(devices) {
			entry := map[string]interface{}{
				"serial": dev.Serial,
				"name":   firstNonEmpty(dev.Name, dev.Serial),
				"model":  dev.Model,
			}
			// LLDP/CDP uplink ports
			uplinkSet := client.GetDeviceUplinkPorts(ctx, dev.Serial)
			uplinkList := make([]string, 0, len(uplinkSet))
			for p := range uplinkSet {
				uplinkList = append(uplinkList, p)
			}
			sort.Strings(uplinkList)
			entry["lldpUplinkPorts"] = uplinkList

			// Raw switch port list (for reference)
			portsRaw, pErr := client.GetSwitchPortsRaw(ctx, dev.Serial)
			if pErr != nil {
				entry["switchPortsError"] = pErr.Error()
			} else {
				var portsParsed interface{}
				_ = json.Unmarshal(portsRaw, &portsParsed)
				entry["switchPorts"] = portsParsed
			}
			switchData = append(switchData, entry)
		}
		out["switches"] = switchData
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
