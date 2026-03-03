package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"Find-Meraki-Ports-With-MAC/pkg/logger"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// ── Log broadcast hub ─────────────────────────────────────────────────────────
// wsLogHub collects all log lines produced during web requests and fans them out
// to every connected WebSocket /ws/logs client.

type logHub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

var wsLogHub = &logHub{clients: make(map[chan string]struct{})}

func (h *logHub) subscribe() chan string {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *logHub) unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *logHub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default: // drop if subscriber is slow
		}
	}
}

// wsWriter satisfies io.Writer so logger output can be forwarded to WebSocket clients.
type wsWriter struct{}

func (wsWriter) Write(p []byte) (int, error) {
	wsLogHub.broadcast(string(p))
	return len(p), nil
}

// newWebLogger returns a logger that writes to both stderr and the WebSocket broadcast hub.
func newWebLogger() *logger.Logger {
	return logger.NewWriter(io.MultiWriter(os.Stderr, wsWriter{}), logger.LevelDebug)
}

func startWebServer(cfg Config, host, port string) {
	webAPIKey = cfg.APIKey
	webPresetMAC = cfg.MACAddress
	webPresetIP = cfg.IPAddress
	webPresetOrgName = cfg.OrgName
	webPresetNetwork = cfg.NetworkName
	log := newWebLogger()
	log.Infof("Starting web server on %s:%s", host, port)

	r := mux.NewRouter()

	// Static files — served with no-cache so the browser always fetches the
	// latest JS/CSS after a binary rebuild (development tool on localhost).
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.PathPrefix("/static/").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		staticHandler.ServeHTTP(w, req)
	})

	// API routes
	r.HandleFunc("/", handleHome).Methods("GET")
	if webTestDataMode {
		r.HandleFunc("/api/validate-key", handleTestValidateKey).Methods("POST")
		r.HandleFunc("/api/config", handleTestGetConfig).Methods("GET")
		r.HandleFunc("/api/networks", handleTestGetNetworks).Methods("GET")
		r.HandleFunc("/api/resolve", handleTestResolve).Methods("POST")
		r.HandleFunc("/api/manufacturer", handleTestGetManufacturer).Methods("GET")
	} else {
		r.HandleFunc("/api/validate-key", handleValidateKey).Methods("POST")
		r.HandleFunc("/api/config", handleGetConfig).Methods("GET")
		r.HandleFunc("/api/networks", handleGetNetworks).Methods("GET")
		r.HandleFunc("/api/resolve", handleResolve).Methods("POST")
		r.HandleFunc("/api/manufacturer", handleGetManufacturer).Methods("GET")
	}
	r.HandleFunc("/topology", handleTopology).Methods("GET")
	r.HandleFunc("/api/topology", handleGetTopology).Methods("GET")
	r.HandleFunc("/api/alerts", handleGetAlerts).Methods("GET")
	r.HandleFunc("/api/logs", handleLogs).Methods("GET")
	r.HandleFunc("/api/debug/network", handleDebugNetwork).Methods("GET")

	// WebSocket for real-time updates
	r.HandleFunc("/ws/logs", handleWebSocketLogs)

	addr := fmt.Sprintf("%s:%s", host, port)
	url := fmt.Sprintf("http://%s", addr)
	log.Infof("Web interface available at %s", url)
	log.Infof("Press Ctrl+C to stop the server")

	// Open browser after a short delay to allow the server to start
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Errorf("Web server error: %v", err)
		os.Exit(1)
	}
}

// openBrowser opens the given URL in the default system browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux and others
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// handleHome serves the main web application HTML page.
func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Find Meraki Ports</title>
<link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
<div id="app">

  <div class="topbar">
    <h1>&#128269; Find Meraki Ports</h1>
    <span class="context-badge" id="contextBadge"></span>
    <div class="spacer"></div>
    <div class="status-row hidden" id="statusRow">
      <span class="dot dot-idle" id="statusDot"></span>
      <span id="statusLabel"></span>
    </div>
    <span class="version">v` + Version + `</span>
  </div>

  <div class="body-wrap">

    <!-- ── Sidebar ───────────────────────────────────────── -->
    <div class="sidebar">

      <!-- API Key (hidden when pre-loaded from .env) -->
      <div class="card hidden" id="keySection">
        <div class="card-header"><h3>API Key</h3></div>
        <div class="card-body">
          <div class="field">
            <label for="apiKey">Meraki API Key</label>
            <input type="password" id="apiKey" placeholder="Enter Meraki Dashboard API key">
          </div>
          <button class="btn btn-primary btn-wide" id="validateKeyBtn">Validate Key</button>
        </div>
      </div>

      <!-- Org / Network -->
      <div class="card">
        <div class="card-header"><h3>Scope</h3></div>
        <div class="card-body">
          <div class="field hidden" id="orgRow">
            <label for="orgSelect">Organization</label>
            <select id="orgSelect"><option value="">— Select organization —</option></select>
          </div>
          <div class="field hidden" id="networkRow">
            <label for="networkSelect">Network</label>
            <select id="networkSelect"><option value="">— Select network —</option></select>
          </div>
          <div class="field" id="scopeHint" style="color:#9ca3af;font-size:.82rem;padding:4px 0;">
            Loading organizations…
          </div>
        </div>
      </div>

      <!-- Resolve inputs -->
      <div class="card hidden" id="resolveSection">
        <div class="card-header"><h3>Device Lookup</h3></div>
        <div class="card-body">
          <div class="field">
            <label for="macInput">MAC Address</label>
            <input type="text" id="macInput" placeholder="00:11:22:33:44:55">
            <div class="hint">Formats: colon, dash, dotted, bare, wildcard *</div>
          </div>
          <div class="field">
            <label for="ipInput">IP Address <span style="font-weight:400;text-transform:none">(optional)</span></label>
            <input type="text" id="ipInput" placeholder="192.168.1.100">
          </div>
          <div class="btn-group" style="margin-top:4px">
            <button class="btn btn-primary" id="resolveBtn">Resolve</button>
            <button class="btn btn-secondary has-tip" id="topologyBtn"
              data-tip="Click a row in the results table to choose which switch to highlight">Topology</button>
            <button class="btn btn-ghost btn-sm" id="clearBtn">Clear</button>
          </div>

          <!-- Manufacturer badge -->
          <div class="hidden" id="mfrRow" style="margin-top:12px;padding-top:12px;border-top:1px solid #e5e7eb">
            <div style="font-size:.76rem;font-weight:600;text-transform:uppercase;letter-spacing:.04em;color:#4b5563;margin-bottom:4px">Manufacturer</div>
            <span class="mfr-badge" id="mfrBadge"></span>
          </div>
        </div>
      </div>

    </div><!-- /sidebar -->

    <!-- ── Main ──────────────────────────────────────────── -->
    <div class="main">

      <!-- Results -->
      <div class="card" id="resultsWrap">
        <div class="card-header">
          <h3>Results</h3>
          <div class="btn-group hidden" id="exportBtns">
            <button class="btn btn-secondary btn-sm" id="exportCsvBtn">&#8595; CSV</button>
            <button class="btn btn-secondary btn-sm" id="exportJsonBtn">&#8595; JSON</button>
          </div>
        </div>
        <div class="card-body">
          <div class="results-toolbar">
            <span class="results-count" id="resultsCount"></span>
          </div>
          <div id="uplinkNote" class="hidden uplink-note"></div>
          <div class="table-wrap">
            <table id="resultsTable">
              <thead>
                <tr>
                  <th data-col="device" class="sortable">Switch / Device</th>
                  <th data-col="network" class="sortable">Network</th>
                  <th data-col="mac" class="sortable">MAC</th>
                  <th data-col="ip" class="sortable">IP</th>
                  <th data-col="port" class="sortable">Port</th>
                  <th data-col="vlan" class="sortable">VLAN</th>
                  <th data-col="hostname" class="sortable">Hostname</th>
                  <th data-col="manufacturer" class="sortable">Manufacturer</th>
                  <th data-col="mode" class="sortable">Mode</th>
                </tr>
              </thead>
              <tbody id="resultsTbody">
                <tr><td colspan="9" class="no-results">Select a network and enter a MAC or IP address to begin.</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Log console -->
      <div class="card" id="logSection">
        <div class="card-header" id="logToggle">
          <h3>Logs <i class="chevron">&#9660;</i></h3>
          <div class="log-toolbar">
            <select id="logLevelSel">
              <option value="DEBUG">DEBUG</option>
              <option value="INFO">INFO</option>
              <option value="WARNING">WARNING</option>
              <option value="ERROR">ERROR</option>
            </select>
            <button class="btn btn-ghost btn-sm" id="clearLogsBtn">Clear</button>
            <button class="btn btn-ghost btn-sm" id="exportLogsBtn">&#8595; Export</button>
          </div>
        </div>
        <div class="collapsible">
          <div class="log-console" id="logConsole"></div>
        </div>
      </div>

    </div><!-- /main -->
  </div><!-- /body-wrap -->
</div><!-- /app -->

<div id="toastContainer"></div>
<script src="/static/js/app.js"></script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(tmpl))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWebSocketLogs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ch := wsLogHub.subscribe()
	defer wsLogHub.unsubscribe(ch)

	// Drain incoming messages from browser (pings / close frames)
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for msg := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return
		}
	}
}
