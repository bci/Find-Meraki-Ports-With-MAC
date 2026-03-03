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

func handleTopology(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	networkID := q.Get("networkId")
	orgID := q.Get("orgId")
	apiKey := q.Get("apiKey")
	if apiKey == "" {
		apiKey = webAPIKey
	}
	highlightSerial := q.Get("highlightSerial")
	highlightPort := q.Get("highlightPort")
	highlightName := q.Get("highlightName")
	portMode := q.Get("portMode")
	mac := q.Get("mac")
	hostname := q.Get("hostname")

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Network Topology</title>
<script src="https://d3js.org/d3.v7.min.js"></script>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{background:#0f172a;color:#e2e8f0;font-family:'Segoe UI',system-ui,sans-serif;overflow:hidden;height:100vh;display:flex;flex-direction:column}
  #topbar{display:flex;align-items:center;gap:10px;padding:8px 16px;background:#1e293b;border-bottom:1px solid #334155;flex-shrink:0;flex-wrap:wrap}
  #topbar h2{font-size:.95rem;font-weight:600;color:#94a3b8;margin-right:4px}
  .pill{background:#334155;border-radius:999px;padding:3px 12px;font-size:.8rem;color:#cbd5e1;white-space:nowrap}
  .pill.highlight{background:#78350f;color:#fde68a;font-weight:600}
  .spacer{flex:1}
  button.tbtn{background:#1e40af;border:none;color:#bfdbfe;padding:4px 14px;border-radius:6px;cursor:pointer;font-size:.8rem}
  button.tbtn:hover{background:#1d4ed8}
  button.tbtn.danger{background:#7f1d1d;color:#fca5a5}
  button.tbtn.danger:hover{background:#991b1b}
  #canvas{flex:1;overflow:hidden}
  svg{width:100%;height:100%}
  .link{stroke:#334155;stroke-width:2px}
  .link.highlighted{stroke:#f59e0b;stroke-width:3px;stroke-dasharray:6,3}
  .link.pc-link{stroke:#f59e0b;stroke-width:2.5px;stroke-dasharray:8,4}
  .node circle{stroke:#1e293b;stroke-width:2px;cursor:pointer;transition:filter .2s}
  .node circle:hover{filter:brightness(1.3)}
  .node.switch circle{fill:#3b82f6}
  .node.wireless circle{fill:#a855f7}
  .node.appliance circle{fill:#10b981}
  .node.camera circle{fill:#ec4899}
  .node.other circle{fill:#64748b}
  .node.highlighted circle{fill:#f59e0b;filter:drop-shadow(0 0 8px #f59e0b)}
  .node.highlighted text{fill:#fde68a;font-weight:700}
  .node.pc rect.pc-screen{fill:#f59e0b;stroke:#1e293b;stroke-width:2px;cursor:pointer;transition:filter .2s}
  .node.pc rect.pc-screen:hover{filter:brightness(1.2)}
  .node.pc rect.pc-stand,.node.pc rect.pc-base{fill:#d97706;cursor:pointer}
  .node.pc text{fill:#fde68a;font-weight:700}
  .port-label{fill:#f59e0b;font-size:10px;font-weight:700;pointer-events:none;text-anchor:middle}
  .node text{fill:#cbd5e1;font-size:11px;pointer-events:none;text-anchor:middle}
  #tooltip{position:fixed;background:#1e293b;border:1px solid #475569;border-radius:8px;padding:8px 12px;font-size:.78rem;pointer-events:none;opacity:0;transition:opacity .15s;max-width:260px;z-index:100}
  #tooltip .tt-title{font-weight:700;color:#f1f5f9;margin-bottom:4px}
  #tooltip .tt-row{color:#94a3b8}
  #tooltip .tt-row span{color:#e2e8f0}
  #tooltip .tt-row.hi{color:#f59e0b}
  #tooltip .tt-row.hi span{color:#fde68a}
  #legend{position:fixed;bottom:16px;left:16px;background:#1e293b;border:1px solid #334155;border-radius:8px;padding:8px 12px;font-size:.75rem}
  #legend .li{display:flex;align-items:center;gap:6px;margin-bottom:4px}
  #legend .dot{width:10px;height:10px;border-radius:50%;flex-shrink:0}
  #legend .dot.sq{border-radius:2px}
</style>
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
<script>
const NETWORK_ID         = ` + "`" + `__NETWORK_ID__` + "`" + `;
const ORG_ID             = ` + "`" + `__ORG_ID__` + "`" + `;
const API_KEY            = ` + "`" + `__API_KEY__` + "`" + `;
const HIGHLIGHT_SERIAL   = ` + "`" + `__HIGHLIGHT_SERIAL__` + "`" + `;
const HIGHLIGHT_PORT     = ` + "`" + `__HIGHLIGHT_PORT__` + "`" + `;
const HIGHLIGHT_NAME     = ` + "`" + `__HIGHLIGHT_NAME__` + "`" + `;
const PORT_MODE          = ` + "`" + `__PORT_MODE__` + "`" + `;
const HIGHLIGHT_MAC      = ` + "`" + `__HIGHLIGHT_MAC__` + "`" + `;
const HIGHLIGHT_HOSTNAME = ` + "`" + `__HIGHLIGHT_HOSTNAME__` + "`" + `;

const svg = d3.select('#svg');
const root = d3.select('#root');
const tooltip = document.getElementById('tooltip');
const netPill = document.getElementById('netPill');
const hlPill  = document.getElementById('hlPill');

if (HIGHLIGHT_NAME || HIGHLIGHT_SERIAL) {
  const label = HIGHLIGHT_NAME || HIGHLIGHT_SERIAL;
  hlPill.textContent = '📍 ' + label + (HIGHLIGHT_PORT ? ' port ' + HIGHLIGHT_PORT : '');
  hlPill.style.display = '';
}

const url = '/api/topology?networkId=' + encodeURIComponent(NETWORK_ID)
          + '&apiKey='   + encodeURIComponent(API_KEY);

let zoomBehavior;

fetch(url)
  .then(r => r.json())
  .then(data => {
    const nodes = data.nodes || [];
    const links = data.links || [];
    netPill.textContent = data.networkName || ('Network: ' + NETWORK_ID.slice(0,12));

    // Inject a virtual PC node for access-mode ports
    if (PORT_MODE === 'access' && HIGHLIGHT_SERIAL) {
      // Ensure the highlighted switch exists in the node list — the topology API
      // may not include it (e.g. Catalyst switches not in CDP/LLDP data).
      if (!nodes.find(n => n.id === HIGHLIGHT_SERIAL)) {
        nodes.push({ id: HIGHLIGHT_SERIAL, name: HIGHLIGHT_NAME || HIGHLIGHT_SERIAL,
                     type: 'switch' });
      }
      const pcLabel = HIGHLIGHT_HOSTNAME || HIGHLIGHT_MAC || 'Device';
      nodes.push({ id: '__pc__', name: pcLabel, type: 'pc', isPc: true,
                   mac: HIGHLIGHT_MAC, hostname: HIGHLIGHT_HOSTNAME });
      links.push({ source: '__pc__', target: HIGHLIGHT_SERIAL, isPcLink: true, port: HIGHLIGHT_PORT });
      document.getElementById('pcLegend').style.display = '';
    } else if (HIGHLIGHT_SERIAL && !nodes.find(n => n.id === HIGHLIGHT_SERIAL)) {
      // Non-access mode: still ensure the target switch is present in the graph
      nodes.push({ id: HIGHLIGHT_SERIAL, name: HIGHLIGHT_NAME || HIGHLIGHT_SERIAL,
                   type: 'switch' });
    }

    renderGraph(nodes, links);
  })
  .catch(err => {
    netPill.textContent = 'Error loading topology';
    console.error(err);
  });

function nodeClass(d) {
  if (d.isPc) return 'pc';
  const t = (d.type || '').toLowerCase();
  if (t.includes('switch')) return 'switch';
  if (t.includes('wireless') || t.includes('ap') || t.includes('mr')) return 'wireless';
  if (t.includes('appliance') || t.includes('mx') || t.includes('vpn')) return 'appliance';
  if (t.includes('camera') || t.includes('mv')) return 'camera';
  return 'other';
}

function renderGraph(nodes, links) {
  const W = document.getElementById('canvas').clientWidth;
  const H = document.getElementById('canvas').clientHeight;

  zoomBehavior = d3.zoom().scaleExtent([0.1, 4]).on('zoom', e => root.attr('transform', e.transform));
  svg.call(zoomBehavior);

  // Links
  const linkSel = root.append('g').attr('class','links').selectAll('line')
    .data(links).join('line')
    .attr('class', d => {
      if (d.isPcLink) return 'link pc-link';
      const src = typeof d.source === 'object' ? d.source.id : d.source;
      const tgt = typeof d.target === 'object' ? d.target.id : d.target;
      return 'link' + (src === HIGHLIGHT_SERIAL || tgt === HIGHLIGHT_SERIAL ? ' highlighted' : '');
    });

  // Port labels on PC-link (shown near the switch)
  const portLabelSel = root.append('g').attr('class','port-labels').selectAll('text')
    .data(links.filter(l => l.isPcLink && l.port)).join('text')
    .attr('class','port-label')
    .text(d => 'port ' + d.port);

  // Nodes
  const nodeSel = root.append('g').attr('class','nodes').selectAll('g')
    .data(nodes).join('g')
    .attr('class', d => 'node ' + nodeClass(d) + (d.id === HIGHLIGHT_SERIAL ? ' highlighted' : ''))
    .call(d3.drag()
      .on('start', (e,d) => { if (!e.active) sim.alphaTarget(0.3).restart(); d.fx=d.x; d.fy=d.y; })
      .on('drag',  (e,d) => { d.fx=e.x; d.fy=e.y; })
      .on('end',   (e,d) => { if (!e.active) sim.alphaTarget(0); d.fx=null; d.fy=null; }))
    .on('mousemove', (e,d) => {
      tooltip.style.opacity = 1;
      tooltip.style.left = (e.clientX+14)+'px';
      tooltip.style.top  = (e.clientY-10)+'px';
      if (d.isPc) {
        tooltip.innerHTML = ` + "`" + `<div class="tt-title">🖥 Found Device</div>
          <div class="tt-row hi">MAC: <span>${d.mac||'—'}</span></div>` + "`" + ` +
          (d.hostname ? ` + "`" + `<div class="tt-row hi">Host: <span>${d.hostname}</span></div>` + "`" + ` : '') +
          (HIGHLIGHT_PORT ? ` + "`" + `<div class="tt-row hi">Port: <span>${HIGHLIGHT_PORT}</span></div>` + "`" + ` : '') +
          ` + "`" + `<div class="tt-row">Mode: <span>Access</span></div>` + "`" + `;
      } else {
        tooltip.innerHTML = ` + "`" + `<div class="tt-title">${d.name||d.id}</div>
          <div class="tt-row">Type: <span>${d.type||'—'}</span></div>
          <div class="tt-row">Serial: <span>${d.id||'—'}</span></div>
          <div class="tt-row">Model: <span>${d.model||'—'}</span></div>` + "`" + ` +
          (d.id === HIGHLIGHT_SERIAL && HIGHLIGHT_PORT ? ` + "`" + `<div class="tt-row hi">Port ${HIGHLIGHT_PORT}: <span>Device connected</span></div>` + "`" + ` : '');
      }
    })
    .on('mouseleave', () => { tooltip.style.opacity = 0; });

  // PC nodes: monitor icon as SVG rects
  const pcSel = nodeSel.filter(d => d.isPc);
  pcSel.append('rect').attr('class','pc-screen').attr('x',-14).attr('y',-14).attr('width',28).attr('height',18).attr('rx',2);
  pcSel.append('rect').attr('class','pc-display').attr('x',-11).attr('y',-11).attr('width',22).attr('height',12).attr('rx',1).style('fill','#0f172a');
  pcSel.append('rect').attr('class','pc-stand').attr('x',-2).attr('y',4).attr('width',4).attr('height',7);
  pcSel.append('rect').attr('class','pc-base').attr('x',-7).attr('y',10).attr('width',14).attr('height',3);

  // Regular nodes: circles
  nodeSel.filter(d => !d.isPc)
    .append('circle').attr('r', d => d.id === HIGHLIGHT_SERIAL ? 18 : 12);

  // Labels
  nodeSel.append('text')
    .attr('dy', d => d.isPc ? 30 : (d.id === HIGHLIGHT_SERIAL ? 32 : 26))
    .text(d => (d.name||d.id||'').slice(0,22));

  const sim = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id(d=>d.id).distance(d => d.isPcLink ? 80 : 120))
    .force('charge', d3.forceManyBody().strength(-400))
    .force('center', d3.forceCenter(W/2, H/2))
    .force('collision', d3.forceCollide(32))
    .on('tick', () => {
      linkSel.attr('x1',d=>d.source.x).attr('y1',d=>d.source.y)
             .attr('x2',d=>d.target.x).attr('y2',d=>d.target.y);
      // Port label: 80% towards the switch (target) from PC (source)
      portLabelSel
        .attr('x', d => d.source.x + (d.target.x - d.source.x) * 0.78)
        .attr('y', d => d.source.y + (d.target.y - d.source.y) * 0.78 - 6);
      nodeSel.attr('transform', d => ` + "`" + `translate(${d.x},${d.y})` + "`" + `);
    });

  // After settling, center on the switch (or PC if no serial given)
  const centerTarget = HIGHLIGHT_SERIAL || '__pc__';
  setTimeout(() => {
    const hn = nodes.find(n => n.id === centerTarget);
    if (hn && hn.x != null) {
      const scale = 1.5;
      const tx = W/2 - scale*hn.x;
      const ty = H/2 - scale*hn.y;
      svg.transition().duration(800)
        .call(zoomBehavior.transform, d3.zoomIdentity.translate(tx,ty).scale(scale));
    }
  }, 2200);
}

document.getElementById('resetBtn').addEventListener('click', () => {
  svg.transition().duration(400).call(zoomBehavior.transform, d3.zoomIdentity);
});
</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	networkID = firstNonEmpty(networkID, "unknown")
	orgID = firstNonEmpty(orgID, "")
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
