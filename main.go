// Package main provides a command-line tool for finding MAC addresses on Meraki switches.
// It supports both native Meraki MS switches and Cisco Catalyst switches managed by Meraki.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"Find-Meraki-Ports-With-MAC/pkg/filters"
	"Find-Meraki-Ports-With-MAC/pkg/logger"
	"Find-Meraki-Ports-With-MAC/pkg/macaddr"
	"Find-Meraki-Ports-With-MAC/pkg/meraki"
	"Find-Meraki-Ports-With-MAC/pkg/output"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

// Config holds all configuration options from environment variables and command-line flags.
type Config struct {
	APIKey       string // Meraki Dashboard API key
	OrgName      string // Organization name filter
	OrgID        string // Organization ID (used by web path for direct lookup)
	NetworkName  string // Network name filter or "ALL"
	OutputFormat string // Output format: csv, text, or html
	BaseURL      string // Meraki API base URL
	MaxRetries   int    // Maximum number of API request retries on 429
	MacTablePoll int    // MAC table lookup poll attempts (2s each)
	LogFile      string // Path to log file
	LogLevel     string // Log level: DEBUG, INFO, WARNING, ERROR
	Verbose      bool   // Enable verbose output
	SwitchFilter string // Switch name filter
	PortFilter   string // Port filter
	TestFull     bool   // Display complete MAC forwarding table
	IPAddress    string // IP address to resolve
}

// Version information injected at build time via ldflags.
// Build with: go build -ldflags "-X main.Version=1.0.0 -X main.Commit=<git-sha> -X main.BuildTime=<timestamp>"
const (
	RepositoryURL = "https://github.com/bci/Find-Meraki-Ports-With-MAC"
)

var (
	Version    = "dev"     // Version set at build time
	Commit     = "unknown" // Git commit SHA set at build time
	BuildTime  = "unknown" // Build timestamp set at build time
	GoVersion  = "go1.21"  // Go version (can be updated at build time)
	webAPIKey  string      // API key pre-loaded from .env for the web interface
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
	ch := make(chan string, 128)
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
		default: // drop if consumer is slow
		}
	}
}

// wsWriter is an io.Writer that sends each line to the hub.
type wsWriter struct{}

func (wsWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg != "" {
		wsLogHub.broadcast(msg)
	}
	return len(p), nil
}

// newWebLogger returns a logger that writes to both stderr and the WS hub.
func newWebLogger() *logger.Logger {
	return logger.NewWriter(io.MultiWriter(os.Stderr, wsWriter{}), logger.LevelDebug)
}

func main() {
	_ = godotenv.Load()

	macFlag := flag.String("mac", "", "MAC address or pattern")
	ipFlag := flag.String("ip", "", "IP address to resolve to MAC")
	networkFlag := flag.String("network", "", "Network name or ALL")
	orgFlag := flag.String("org", "", "Organization name")
	outputFlag := flag.String("output-format", "", "Output format: csv, text, html")
	listOrgsFlag := flag.Bool("list-orgs", false, "List organizations the API key can access and exit")
	listNetworksFlag := flag.Bool("list-networks", false, "List networks per organization and exit")
	testAPIFlag := flag.Bool("test-api", false, "Validate API key and exit")
	testFullTableFlag := flag.Bool("test-full-table", false, "Display all MAC addresses in forwarding table (filtered by --switch/--port)")
	verboseFlag := flag.Bool("verbose", false, "Send DEBUG logs to console (overrides --log-level and --log-file)")
	switchFlag := flag.String("switch", "", "Filter by switch name (case-insensitive substring match)")
	portFlag := flag.String("port", "", "Filter by port name/number")
	logFileFlag := flag.String("log-file", "", "Log file path")
	logLevelFlag := flag.String("log-level", "", "Log level: DEBUG, INFO, WARNING, ERROR")
	versionFlag := flag.Bool("version", false, "Show version and exit")
	helpFlag := flag.Bool("help", false, "Show help")
	interactiveFlag := flag.Bool("interactive", false, "Launch web interface mode")
	retryFlag := flag.Int("retry", 0, "Maximum API retry attempts on rate limit (default: 6)")
	macPollFlag := flag.Int("mac-table-poll", 0, "MAC table lookup poll attempts, 2s each (default: 15)")
	webPortFlag := flag.String("web-port", "", "Port for web server (default: 8080)")
	webHostFlag := flag.String("web-host", "", "Host for web server (default: localhost)")
	flag.Usage = func() {
		printUsage(os.Stdout)
	}
	flag.Parse()

	cfg := Config{
		APIKey:       strings.TrimSpace(os.Getenv("MERAKI_API_KEY")),
		OrgName:      strings.TrimSpace(firstNonEmpty(*orgFlag, os.Getenv("MERAKI_ORG"))),
		NetworkName:  strings.TrimSpace(firstNonEmpty(*networkFlag, os.Getenv("MERAKI_NETWORK"))),
		OutputFormat: strings.TrimSpace(firstNonEmpty(*outputFlag, os.Getenv("OUTPUT_FORMAT"))),
		BaseURL:      strings.TrimSpace(firstNonEmpty(os.Getenv("MERAKI_BASE_URL"), "https://api.meraki.com/api/v1")),
		MaxRetries:   firstNonZeroInt(*retryFlag, parseIntEnv("MERAKI_RETRIES"), 6),
		MacTablePoll: firstNonZeroInt(*macPollFlag, parseIntEnv("MERAKI_MAC_POLL"), 15),
		LogFile:      strings.TrimSpace(firstNonEmpty(*logFileFlag, os.Getenv("LOG_FILE"), "Find-Meraki-Ports-With-MAC.log")),
		LogLevel:     strings.TrimSpace(firstNonEmpty(*logLevelFlag, os.Getenv("LOG_LEVEL"), "DEBUG")),
		Verbose:      *verboseFlag,
		SwitchFilter: strings.TrimSpace(*switchFlag),
		PortFilter:   strings.TrimSpace(*portFlag),
		TestFull:     *testFullTableFlag,
		IPAddress:    strings.TrimSpace(*ipFlag),
	}

	// If verbose flag is set, override log level to DEBUG and send logs to console
	if *verboseFlag {
		cfg.LogLevel = "DEBUG"
		cfg.LogFile = "" // Empty log file sends logs to console only
		fmt.Printf("DEBUG: Verbose flag set, LogLevel=%s, LogFile='%s'\n", cfg.LogLevel, cfg.LogFile)
	}

	if *helpFlag {
		printUsage(os.Stdout)
		return
	}

	if *versionFlag {
		printVersion(os.Stdout)
		return
	}

	// Handle interactive mode
	if *interactiveFlag {
		webPort := firstNonEmpty(*webPortFlag, os.Getenv("WEB_PORT"), "8080")
		webHost := firstNonEmpty(*webHostFlag, os.Getenv("WEB_HOST"), "localhost")
		startWebServer(cfg, webHost, webPort)
		return
	}

	log := logger.New(cfg.LogFile, logger.ParseLogLevel(cfg.LogLevel))

	if cfg.APIKey == "" {
		exitWithError(log, "MERAKI_API_KEY is required in .env or environment")
	}
	if cfg.NetworkName == "" {
		cfg.NetworkName = "ALL"
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "csv"
	}

	cfg.OutputFormat = strings.ToLower(cfg.OutputFormat)
	if cfg.OutputFormat != "csv" && cfg.OutputFormat != "text" && cfg.OutputFormat != "html" {
		exitWithError(log, "--output-format must be one of: csv, text, html")
	}

	client := meraki.NewClient(cfg.APIKey, cfg.BaseURL, cfg.MaxRetries)
	ctx := context.Background()

	if *testAPIFlag {
		orgs, err := client.GetOrganizations(ctx)
		if err != nil {
			exitWithError(log, err.Error())
		}
		fmt.Fprintf(os.Stdout, "API OK: %d organizations found\n", len(orgs))
		return
	}

	if *listOrgsFlag {
		orgs, err := client.GetOrganizations(ctx)
		if err != nil {
			exitWithError(log, err.Error())
		}
		writeOrganizations(os.Stdout, orgs)
		return
	}

	if *listNetworksFlag {
		orgs, err := client.GetOrganizations(ctx)
		if err != nil {
			exitWithError(log, err.Error())
		}
		if cfg.OrgName != "" {
			org, err := selectOrganization(cfg.OrgName, orgs)
			if err != nil {
				exitWithError(log, err.Error())
			}
			networks, err := client.GetNetworks(ctx, org.ID)
			if err != nil {
				exitWithError(log, err.Error())
			}
			writeNetworksForOrg(os.Stdout, org, networks)
			return
		}
		for _, org := range orgs {
			networks, err := client.GetNetworks(ctx, org.ID)
			if err != nil {
				exitWithError(log, err.Error())
			}
			writeNetworksForOrg(os.Stdout, org, networks)
		}
		return
	}

	if cfg.TestFull {
		log.Debugf("Test full table mode enabled")
	}

	// Validate mutual exclusivity of --ip and --mac
	if cfg.IPAddress != "" && strings.TrimSpace(*macFlag) != "" {
		exitWithError(log, "--ip and --mac are mutually exclusive")
	}

	if cfg.IPAddress == "" && strings.TrimSpace(*macFlag) == "" {
		if !cfg.TestFull {
			exitWithError(log, "--ip or --mac is required")
		}
	}

	// Get organizations first
	orgs, err := client.GetOrganizations(ctx)
	if err != nil {
		exitWithError(log, err.Error())
	}

	// Handle single organization auto-selection.
	// When the API key is scoped to exactly one org, use it unconditionally.
	// If an org name was specified but doesn't match, log a warning and continue.
	if len(orgs) == 1 {
		if cfg.OrgName != "" && cfg.OrgName != orgs[0].Name {
			log.Debugf("Org name %q not matched; auto-selecting only available organization: %s", cfg.OrgName, orgs[0].Name)
		}
		cfg.OrgName = orgs[0].Name
		log.Debugf("Auto-selected single organization: %s", cfg.OrgName)
	}

	org, err := selectOrganization(cfg.OrgName, orgs)
	if err != nil {
		exitWithError(log, err.Error())
	}
	log.Debugf("Organization: %s", org.Name)

	networks, err := client.GetNetworks(ctx, org.ID)
	if err != nil {
		exitWithError(log, err.Error())
	}

	selectedNetworks, err := selectNetworks(cfg.NetworkName, networks)
	if err != nil {
		exitWithError(log, err.Error())
	}

	matcher := func(string) bool { return true }
	var resolvedHostname string

	if cfg.IPAddress != "" {
		// IP resolution mode
		log.Debugf("Resolving IP: %s", cfg.IPAddress)

		// Resolve IP to MAC
		resolvedMAC, _, resolvedHostname, err := client.ResolveIPToMAC(ctx, org.ID, selectedNetworks, cfg.IPAddress)
		if err != nil {
			exitWithError(log, fmt.Sprintf("Failed to resolve IP %s: %v", cfg.IPAddress, err))
		}

		log.Debugf("Resolved IP %s to MAC %s (hostname: %s)", cfg.IPAddress, resolvedMAC, resolvedHostname)

		// Create matcher for the resolved MAC
		matcher, _, _, err = macaddr.BuildMacMatcher(resolvedMAC)
		if err != nil {
			exitWithError(log, err.Error())
		}

	} else if strings.TrimSpace(*macFlag) != "" {
		// MAC mode (existing logic)
		var normalized string
		var isWildcard bool
		var err error
		matcher, normalized, isWildcard, err = macaddr.BuildMacMatcher(*macFlag)
		if err != nil {
			exitWithError(log, err.Error())
		}
		if isWildcard {
			log.Debugf("MAC pattern: %s", strings.TrimSpace(*macFlag))
		} else {
			log.Debugf("MAC: %s", normalized)
		}
	}

	var results []output.ResultRow
	resultsIndex := make(map[string]struct{})
	for _, net := range selectedNetworks {
		log.Debugf("Network: %s", net.Name)

		// Get all devices for this network
		devices, err := client.GetDevices(ctx, net.ID)
		if err != nil {
			exitWithError(log, err.Error())
		}

		// Build device lookup map
		deviceBySerial := make(map[string]meraki.Device)
		for _, dev := range devices {
			deviceBySerial[dev.Serial] = dev
		}

		// Filter to switches only
		switches := filters.FilterSwitches(devices)
		switches = filters.FilterSwitchesByName(switches, cfg.SwitchFilter)

		// Query network-level clients
		networkClients, err := client.GetNetworkClients(ctx, net.ID)
		if err != nil {
			exitWithError(log, err.Error())
		}
		log.Debugf("Network clients API returned %d clients", len(networkClients))

		// Build MAC→IP map for enriching results from live table / device clients.
		macToIP := make(map[string]string, len(networkClients))
		for _, nc := range networkClients {
			if nc.IP == "" {
				continue
			}
			if norm, err2 := macaddr.NormalizeExactMac(nc.MAC); err2 == nil {
				macToIP[norm] = nc.IP
			}
		}

		// ipAndHostname returns the IP (and reverse-DNS hostname in MAC mode) for a
		// normalized MAC. If not found in macToIP, performs a live ARP table lookup
		// on the switch (serial) where the MAC was found, caching results per switch.
		// In IP mode the hostname is already in resolvedHostname.
		serialArpCache := make(map[string]map[string]string)
		ipAndHostname := func(normMAC, knownIP, serial string) (string, string) {
			ip := knownIP
			if ip == "" {
				ip = macToIP[normMAC]
			}
			// Fallback: live ARP table lookup on the specific switch
			if ip == "" && serial != "" {
				if _, cached := serialArpCache[serial]; !cached {
					serialArpCache[serial] = client.FetchArpMap(ctx, serial, cfg.MacTablePoll)
				}
				ip = serialArpCache[serial][normMAC]
			}
			hn := resolvedHostname // pre-set in IP mode
			if hn == "" && ip != "" {
				hn, _ = meraki.ResolveHostname(ip)
			}
			return ip, hn
		}

		for _, c := range networkClients {
			normMAC, err := macaddr.NormalizeExactMac(c.MAC)
			if err != nil {
				continue
			}
			if matcher(normMAC) {
				serial := strings.TrimSpace(c.RecentDeviceSerial)
				if serial == "" {
					continue
				}

				dev := deviceBySerial[serial]
				switchName := firstNonEmpty(dev.Name, c.RecentDeviceName, serial)

				if !filters.MatchesSwitchFilter(switchName, cfg.SwitchFilter) {
					if cfg.Verbose {
						log.Debugf("Network client %s filtered out by switch filter (switch=%s, filter=%s)",
							macaddr.FormatMacColon(normMAC), switchName, cfg.SwitchFilter)
					}
					continue
				}

				port := firstNonEmpty(c.SwitchportName, c.Switchport, c.Port, "unknown")
				if !filters.MatchesPortFilter(port, cfg.PortFilter) {
					continue
				}

				if cfg.Verbose {
					log.Debugf("Adding network client %s on %s port %s", macaddr.FormatMacColon(normMAC), switchName, port)
				}

				vlan, portMode := enrichPortInfo(ctx, client, serial, port, 0, "")

				ip, hn := ipAndHostname(normMAC, c.IP, serial)
				addResult(resultsIndex, &results, output.ResultRow{
					OrgName:      org.Name,
					NetworkName:  net.Name,
					SwitchName:   switchName,
					SwitchSerial: serial,
					Port:         port,
					MAC:          macaddr.FormatMacColon(normMAC),
					IP:           ip,
					Hostname:     hn,
					LastSeen:     c.LastSeen,
					VLAN:         vlan,
					PortMode:     portMode,
				})
			}
		}

		// Query device-level clients for each switch
		for _, dev := range switches {
			log.Debugf("Querying switch: %s (%s)", firstNonEmpty(dev.Name, dev.Serial), dev.Serial)

			// Try live tools MAC table lookup first (works for all switches including Catalyst)
			macTableID, err := client.CreateMacTableLookup(ctx, dev.Serial)
			if err == nil && macTableID != "" {
				if cfg.Verbose {
					log.Debugf("Created MAC table lookup job %s for %s", macTableID, dev.Serial)
				}

				// Poll for results (max 30 seconds)
				var macEntries []map[string]interface{}
				var status string
				for attempt := 0; attempt < cfg.MacTablePoll; attempt++ {
					time.Sleep(2 * time.Second)
					macEntries, status, err = client.GetMacTableLookup(ctx, dev.Serial, macTableID)
					if err != nil {
						if cfg.Verbose {
							log.Debugf("Error getting MAC table lookup for %s (%s) in network %s: %v",
								firstNonEmpty(dev.Name, dev.Serial), dev.Serial, net.Name, err)
						}
						break
					}
					if status == "complete" {
						break
					}
					if cfg.Verbose {
						log.Debugf("MAC table lookup status for %s (%s) in network %s: %s (attempt %d/%d)",
							firstNonEmpty(dev.Name, dev.Serial), dev.Serial, net.Name, status, attempt+1, cfg.MacTablePoll)
					}
				}

				if status == "complete" && len(macEntries) > 0 {
					log.Debugf("Live MAC table returned %d entries for %s", len(macEntries), firstNonEmpty(dev.Name, dev.Serial))

					foundInTable := false
					for _, entry := range macEntries {
						macStr, _ := entry["mac"].(string)
						if macStr == "" {
							continue
						}

						normMAC, err := macaddr.NormalizeExactMac(macStr)
						if err != nil {
							continue
						}

						if matcher(normMAC) {
							// Try different field names for port
							portID, _ := entry["portId"].(string)
							if portID == "" {
								portID, _ = entry["port"].(string)
							}
							if portID == "" {
								portID, _ = entry["interface"].(string)
							}
							vlan, _ := entry["vlan"].(float64)
							portMode, _ := entry["type"].(string) // "access" or "trunk"

							if cfg.Verbose && portID == "" {
								log.Debugf("MAC entry fields: %+v", entry)
							}

							port := firstNonEmpty(portID, "unknown")
							if !filters.MatchesPortFilter(port, cfg.PortFilter) {
								continue
							}

							// Enrich with switch port API (authoritative VLAN + mode)
							richVLAN, richMode := enrichPortInfo(ctx, client, dev.Serial, port, int(vlan), portMode)

							if cfg.Verbose {
								log.Debugf("Found MAC %s on %s port %s (VLAN %d, mode=%s) via live lookup",
									macaddr.FormatMacColon(normMAC), firstNonEmpty(dev.Name, dev.Serial), port, richVLAN, richMode)
							}

							ip, hn := ipAndHostname(normMAC, "", dev.Serial)
							addResult(resultsIndex, &results, output.ResultRow{
								OrgName:      org.Name,
								NetworkName:  net.Name,
								SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
								SwitchSerial: dev.Serial,
								Port:         port,
								MAC:          macaddr.FormatMacColon(normMAC),
								IP:           ip,
								Hostname:     hn,
								LastSeen:     "",
								VLAN:         richVLAN,
								PortMode:     richMode,
							})
							foundInTable = true
						}
					}
					// Only skip device-clients fallback if the target MAC was actually found in the table.
					// If the table had entries but our MAC wasn't present (device temporarily inactive),
					// fall through so device clients history can still surface the result.
					if foundInTable {
						continue // Skip device clients API
					}
				}
			}

			// Fallback to device clients API
			clients, err := client.GetDeviceClients(ctx, dev.Serial)
			if err != nil {
				if cfg.Verbose {
					log.Warnf("Failed to get device clients for %s: %v", dev.Serial, err)
				}
				continue
			}

			log.Debugf("Device clients API returned %d clients for %s", len(clients), firstNonEmpty(dev.Name, dev.Serial))

			for _, c := range clients {
				normMAC, err := macaddr.NormalizeExactMac(c.MAC)
				if err != nil {
					continue
				}
				if matcher(normMAC) {
					port := firstNonEmpty(c.SwitchportName, c.Switchport, c.Port, "unknown")
					if !filters.MatchesPortFilter(port, cfg.PortFilter) {
						continue
					}
					vlan, portMode := enrichPortInfo(ctx, client, dev.Serial, port, 0, "")
					ip, hn := ipAndHostname(normMAC, "", dev.Serial)
					addResult(resultsIndex, &results, output.ResultRow{
						OrgName:      org.Name,
						NetworkName:  net.Name,
						SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
						SwitchSerial: dev.Serial,
						Port:         port,
						MAC:          macaddr.FormatMacColon(normMAC),
						IP:           ip,
						Hostname:     hn,
						LastSeen:     c.LastSeen,
						VLAN:         vlan,
						PortMode:     portMode,
					})
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].NetworkName == results[j].NetworkName {
			if results[i].SwitchName == results[j].SwitchName {
				return results[i].Port < results[j].Port
			}
			return results[i].SwitchName < results[j].SwitchName
		}
		return results[i].NetworkName < results[j].NetworkName
	})

	switch cfg.OutputFormat {
	case "csv":
		output.WriteCSV(os.Stdout, results)
	case "text":
		output.WriteText(os.Stdout, results)
	case "html":
		output.WriteHTML(os.Stdout, results)
	}
}

func startWebServer(cfg Config, host, port string) {
	webAPIKey = cfg.APIKey // expose to handlers
	log := newWebLogger()
	log.Infof("Starting web server on %s:%s", host, port)

	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// API routes
	r.HandleFunc("/", handleHome).Methods("GET")
	r.HandleFunc("/api/validate-key", handleValidateKey).Methods("POST")
	r.HandleFunc("/api/config", handleGetConfig).Methods("GET")
	r.HandleFunc("/api/networks", handleGetNetworks).Methods("GET")
	r.HandleFunc("/api/resolve", handleResolve).Methods("POST")
	r.HandleFunc("/api/manufacturer", handleGetManufacturer).Methods("GET")
	r.HandleFunc("/topology", handleTopology).Methods("GET")
	r.HandleFunc("/api/topology", handleGetTopology).Methods("GET")
	r.HandleFunc("/api/alerts", handleGetAlerts).Methods("GET")
	r.HandleFunc("/api/logs", handleLogs).Methods("GET")

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

// Web handler functions
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
		"apiKey": webAPIKey,
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
			APIKey:      req.APIKey,
			OrgID:       req.OrgID,
			NetworkName: netID,
			LogLevel:    "INFO",
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
			"mac":          result.MAC,
			"ip":           result.IP,
			"hostname":     result.Hostname,
			"lastSeen":     result.LastSeen,
			"manufacturer": getManufacturer(result.MAC),
			"vlan":         result.VLAN,
			"portMode":     result.PortMode,
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWebSocketLogs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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

// enrichPortInfo calls the switch port API to get authoritative VLAN and port mode.
// Falls back to the provided defaults if the call fails or port is unsupported.
func enrichPortInfo(ctx context.Context, client *meraki.MerakiClient, serial, portID string, defaultVLAN int, defaultMode string) (vlan int, portMode string) {
	vlan, portMode = defaultVLAN, defaultMode
	if serial == "" || portID == "" || portID == "unknown" {
		return
	}
	sp, err := client.GetSwitchPort(ctx, serial, portID)
	if err != nil {
		return
	}
	if sp.Type != "" {
		portMode = sp.Type
	}
	if sp.Vlan > 0 {
		vlan = sp.Vlan
	}
	return
}

func resolveDevices(cfg Config, macAddr, ipAddr string) ([]output.ResultRow, error) {
	log := newWebLogger()

	client := meraki.NewClient(cfg.APIKey, cfg.BaseURL, cfg.MaxRetries)
	ctx := context.Background()

	var targetOrg *meraki.Organization
	var targetNetwork *meraki.Network

	if cfg.OrgID != "" {
		// Fast path: org ID known, only fetch networks for that org
		networks, err := client.GetNetworks(ctx, cfg.OrgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get networks: %v", err)
		}
		for i, net := range networks {
			if net.ID == cfg.NetworkName {
				targetOrg = &meraki.Organization{ID: cfg.OrgID}
				targetNetwork = &networks[i]
				break
			}
		}
		if targetNetwork == nil {
			return nil, fmt.Errorf("network %s not found in org %s", cfg.NetworkName, cfg.OrgID)
		}
		// Fetch org name for display
		if orgs, err := client.GetOrganizations(ctx); err == nil {
			for _, o := range orgs {
				if o.ID == cfg.OrgID {
					targetOrg.Name = o.Name
					break
				}
			}
		}
	} else {
		// Slow path: search all orgs for the network (fallback when orgId not provided)
		orgs, err := client.GetOrganizations(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get organizations: %v", err)
		}
		if len(orgs) == 0 {
			return nil, fmt.Errorf("no organizations found")
		}
		for _, org := range orgs {
			networks, err := client.GetNetworks(ctx, org.ID)
			if err != nil {
				continue
			}
			for i, net := range networks {
				if net.ID == cfg.NetworkName {
					o := org
					targetOrg = &o
					targetNetwork = &networks[i]
					break
				}
			}
			if targetOrg != nil {
				break
			}
		}
		if targetOrg == nil || targetNetwork == nil {
			return nil, fmt.Errorf("network not found")
		}
	}

	log.Infof("Resolving in organization: %s, network: %s", targetOrg.Name, targetNetwork.Name)

	// Build MAC matcher
	var matcher func(string) bool
	var resolvedHostname string

	if ipAddr != "" {
		// IP resolution mode
		log.Debugf("Resolving IP: %s", ipAddr)

		resolvedMAC, _, hostname, err := client.ResolveIPToMAC(ctx, targetOrg.ID, []meraki.Network{*targetNetwork}, ipAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve IP %s: %v", ipAddr, err)
		}

		log.Debugf("Resolved IP %s to MAC %s (hostname: %s)", ipAddr, resolvedMAC, hostname)
		resolvedHostname = hostname

		matcher, _, _, err = macaddr.BuildMacMatcher(resolvedMAC)
		if err != nil {
			return nil, err
		}
	} else {
		// MAC mode
		var err error
		matcher, _, _, err = macaddr.BuildMacMatcher(macAddr)
		if err != nil {
			return nil, err
		}
	}

	// Get devices and process results (simplified version)
	devices, err := client.GetDevices(ctx, targetNetwork.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %v", err)
	}

	switches := filters.FilterSwitches(devices)
	results, err := processSwitchesForResolution(ctx, client, targetOrg, targetNetwork, switches, matcher, resolvedHostname, cfg.MacTablePoll, log)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func processSwitchesForResolution(ctx context.Context, client *meraki.MerakiClient, org *meraki.Organization, network *meraki.Network, switches []meraki.Device, matcher func(string) bool, hostname string, macTablePoll int, log *logger.Logger) ([]output.ResultRow, error) {
	var results []output.ResultRow
	resultsIndex := make(map[string]struct{})

	// Get network clients
	networkClients, err := client.GetNetworkClients(ctx, network.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network clients: %v", err)
	}

	log.Debugf("Network clients API returned %d clients", len(networkClients))

	// Build device lookup map
	deviceBySerial := make(map[string]meraki.Device)
	for _, dev := range switches {
		deviceBySerial[dev.Serial] = dev
	}

	// Process network clients
	for _, c := range networkClients {
		normMAC, err := macaddr.NormalizeExactMac(c.MAC)
		if err != nil {
			continue
		}
		if matcher(normMAC) {
			serial := strings.TrimSpace(c.RecentDeviceSerial)
			if serial == "" {
				continue
			}

			dev := deviceBySerial[serial]
			switchName := firstNonEmpty(dev.Name, c.RecentDeviceName, serial)

			port := firstNonEmpty(c.SwitchportName, c.Switchport, c.Port, "unknown")
			vlan, portMode := enrichPortInfo(ctx, client, serial, port, 0, "")

			addResult(resultsIndex, &results, output.ResultRow{
				OrgName:      org.Name,
				NetworkName:  network.Name,
				SwitchName:   switchName,
				SwitchSerial: serial,
				Port:         port,
				MAC:          macaddr.FormatMacColon(normMAC),
				IP:           c.IP,
				Hostname:     hostname,
				LastSeen:     c.LastSeen,
				VLAN:         vlan,
				PortMode:     portMode,
			})
		}
	}

	// Process device-level clients for each switch
	for _, dev := range switches {
		log.Debugf("Querying switch: %s (%s)", firstNonEmpty(dev.Name, dev.Serial), dev.Serial)

		// Try live MAC table lookup with up to 15 retries (30 seconds)
		macTableID, err := client.CreateMacTableLookup(ctx, dev.Serial)
		if err != nil {
			log.Debugf("MAC table lookup not available for %s: %v", dev.Serial, err)
			goto fallbackClients
		}

		{
			var macEntries []map[string]interface{}
			var status string
			for attempt := 0; attempt < macTablePoll; attempt++ {
				time.Sleep(2 * time.Second)
				macEntries, status, err = client.GetMacTableLookup(ctx, dev.Serial, macTableID)
				if err != nil {
					break
				}
				if status == "complete" {
					break
				}
				log.Debugf("MAC table status for %s: %s (attempt %d/%d)", firstNonEmpty(dev.Name, dev.Serial), status, attempt+1, macTablePoll)
			}

			if status == "complete" && len(macEntries) > 0 {
				foundInTable := false
				for _, entry := range macEntries {
					macStr, _ := entry["mac"].(string)
					if macStr == "" {
						continue
					}
					normMAC, err := macaddr.NormalizeExactMac(macStr)
					if err != nil {
						continue
					}
					if !matcher(normMAC) {
						continue
					}
					portID, _ := entry["portId"].(string)
					if portID == "" {
						portID, _ = entry["port"].(string)
					}
					if portID == "" {
						portID, _ = entry["interface"].(string)
					}
					vlan, _ := entry["vlan"].(float64)
					portMode, _ := entry["type"].(string)
					richVLAN, richMode := enrichPortInfo(ctx, client, dev.Serial, portID, int(vlan), portMode)
					addResult(resultsIndex, &results, output.ResultRow{
						OrgName:      org.Name,
						NetworkName:  network.Name,
						SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
						SwitchSerial: dev.Serial,
						Port:         firstNonEmpty(portID, "unknown"),
						MAC:          macaddr.FormatMacColon(normMAC),
						IP:           "",
						Hostname:     hostname,
						LastSeen:     "",
						VLAN:         richVLAN,
						PortMode:     richMode,
					})
					foundInTable = true
				}
				// Only skip device-clients fallback if the target MAC was actually
				// found in the table. If the table had entries but our MAC wasn't
				// present (device temporarily inactive), fall through so device
				// clients history can still surface the result.
				if foundInTable {
					continue // skip fallback
				}
			}
		}

	fallbackClients:
		// Fallback to device clients API
		clients, err := client.GetDeviceClients(ctx, dev.Serial)
		if err != nil {
			log.Debugf("Failed to get device clients for %s: %v", dev.Serial, err)
			continue
		}
		for _, c := range clients {
			normMAC, err := macaddr.NormalizeExactMac(c.MAC)
			if err != nil || !matcher(normMAC) {
				continue
			}
			port := firstNonEmpty(c.SwitchportName, c.Switchport, c.Port, "unknown")
			vlan, portMode := enrichPortInfo(ctx, client, dev.Serial, port, 0, "")
			addResult(resultsIndex, &results, output.ResultRow{
				OrgName:      org.Name,
				NetworkName:  network.Name,
				SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
				SwitchSerial: dev.Serial,
				Port:         port,
				MAC:          macaddr.FormatMacColon(normMAC),
				IP:           "",
				Hostname:     hostname,
				LastSeen:     c.LastSeen,
				VLAN:         vlan,
				PortMode:     portMode,
			})
		}
	}

	return results, nil
}

// ouiCache stores OUI prefix → vendor name to avoid duplicate API calls.
var ouiCache sync.Map

// lookupOUI queries api.macvendors.com for the vendor of a MAC address.
// The first three octets (OUI) are used as the cache key.
// Returns empty string if the lookup fails or the vendor is unknown.
func lookupOUI(mac string) string {
	if mac == "" {
		return ""
	}
	// Normalise separators and extract the OUI prefix (first 8 chars: XX:XX:XX)
	norm := strings.ToUpper(strings.NewReplacer("-", ":", ".", ":").Replace(mac))
	parts := strings.Split(norm, ":")
	if len(parts) < 3 {
		return ""
	}
	oui := strings.Join(parts[:3], ":")

	if cached, ok := ouiCache.Load(oui); ok {
		return cached.(string)
	}

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://api.macvendors.com/" + oui)
	if err != nil {
		ouiCache.Store(oui, "")
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		ouiCache.Store(oui, "")
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		ouiCache.Store(oui, "")
		return ""
	}
	vendor := strings.TrimSpace(string(body))
	ouiCache.Store(oui, vendor)
	return vendor
}

func getManufacturer(mac string) string {
	return lookupOUI(mac)
}

// firstNonEmpty returns the first non-empty string from the provided values.
// Returns empty string if all values are empty or contain only whitespace.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// firstNonZeroInt returns the first non-zero int from the provided values.
func firstNonZeroInt(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

// parseIntEnv reads an environment variable and returns its integer value, or 0 if unset/invalid.
func parseIntEnv(key string) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// exitWithError logs an error message and exits the program with status code 1.
// If log is nil, the error is written to stderr instead.
func exitWithError(log *logger.Logger, msg string) {
	if log != nil {
		log.Errorf(msg)
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", msg)
	}
	os.Exit(1)
}

// selectOrganization finds an organization by name.
// If name is empty and only one organization exists, returns that organization.
// Returns an error if name is empty with multiple organizations or if the name is not found.
func selectOrganization(name string, orgs []meraki.Organization) (meraki.Organization, error) {
	if name == "" {
		if len(orgs) == 1 {
			return orgs[0], nil
		}
		return meraki.Organization{}, errors.New("multiple organizations found, please specify --org")
	}
	for _, org := range orgs {
		if strings.EqualFold(org.Name, name) {
			return org, nil
		}
	}
	return meraki.Organization{}, fmt.Errorf("organization %q not found", name)
}

// selectNetworks filters networks by name.
// If name is "ALL" (case-insensitive), returns all networks.
// Otherwise returns a single network matching the name, or an error if not found.
func selectNetworks(name string, networks []meraki.Network) ([]meraki.Network, error) {
	if strings.ToUpper(name) == "ALL" {
		return networks, nil
	}
	for _, net := range networks {
		if strings.EqualFold(net.Name, name) {
			return []meraki.Network{net}, nil
		}
	}
	return nil, fmt.Errorf("network %q not found", name)
}

// addResult adds a result row to the results slice if it's not a duplicate.
// Deduplication is based on switch serial, port, MAC address, and last seen timestamp.
func addResult(index map[string]struct{}, rows *[]output.ResultRow, row output.ResultRow) {
	// Key on serial+port+MAC only (not LastSeen) so network-clients and MAC-table
	// results for the same port don't both appear as separate rows.
	key := fmt.Sprintf("%s|%s|%s", row.SwitchSerial, row.Port, row.MAC)
	if _, exists := index[key]; exists {
		return
	}
	index[key] = struct{}{}
	*rows = append(*rows, row)
}

// printUsage writes comprehensive help text to the specified file.
// Includes all command-line flags, environment variables, and usage examples.
func printUsage(w *os.File) {
	fmt.Fprintln(w, "Find-Meraki-Ports-With-MAC - Meraki MAC lookup")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL --org \"My Org\" --output-format csv")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --ip <address>              IP address to resolve to MAC (mutually exclusive with --mac)")
	fmt.Fprintln(w, "  --mac <mac|pattern>         MAC address or wildcard pattern (required unless using list/test flags)")
	fmt.Fprintln(w, "  --network <name|ALL>        Network name or ALL (default from .env)")
	fmt.Fprintln(w, "  --org <name>                Organization name (optional if only one org accessible)")
	fmt.Fprintln(w, "  --output-format <csv|text|html>  Output format (default from .env)")
	fmt.Fprintln(w, "  --list-orgs                 List organizations and exit")
	fmt.Fprintln(w, "  --list-networks             List networks per organization and exit")
	fmt.Fprintln(w, "  --test-api                  Validate API key and exit")
	fmt.Fprintln(w, "  --test-full-table           Display all MACs in forwarding table (filters apply)")
	fmt.Fprintln(w, "  --switch <name>             Filter by switch name (case-insensitive substring)")
	fmt.Fprintln(w, "  --port <number>             Filter by port name/number")
	fmt.Fprintln(w, "  --verbose                   Send DEBUG logs to console (overrides --log-level and --log-file)")
	fmt.Fprintln(w, "  --log-file <filename>        Log file path (default from .env)")
	fmt.Fprintln(w, "  --log-level <DEBUG|INFO|WARNING|ERROR>  Log level (default from .env)")
	fmt.Fprintln(w, "  --retry <n>                 Max API retry attempts on rate limit (default: 6)")
	fmt.Fprintln(w, "  --mac-table-poll <n>        MAC table lookup poll attempts, 2s each (default: 15)")
	fmt.Fprintln(w, "  --version                   Show version and exit")
	fmt.Fprintln(w, "  --help                      Show this help")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Environment:")
	fmt.Fprintln(w, "  MERAKI_API_KEY     Meraki Dashboard API key (required)")
	fmt.Fprintln(w, "  MERAKI_ORG         Default org name")
	fmt.Fprintln(w, "  MERAKI_NETWORK     Default network name or ALL")
	fmt.Fprintln(w, "  OUTPUT_FORMAT      csv | text | html")
	fmt.Fprintln(w, "  MERAKI_BASE_URL    API base URL (default https://api.meraki.com/api/v1)")
	fmt.Fprintln(w, "  MERAKI_RETRIES     Max API retry attempts on rate limit (default 6)")
	fmt.Fprintln(w, "  MERAKI_MAC_POLL    MAC table lookup poll attempts, 2s each (default 15)")
	fmt.Fprintln(w, "  LOG_FILE           Log file path (default Find-Meraki-Ports-With-MAC.log)")
	fmt.Fprintln(w, "  LOG_LEVEL          DEBUG | INFO | WARNING | ERROR")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --ip 192.168.1.100 --network ALL")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --mac 08:f1:b3:6f:9c:* --output-format text")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --test-full-table --network City --switch ccc9300xa")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --test-full-table --network City --switch ccc9300xa --port 3")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --list-orgs")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --list-networks --org \"My Org\"")
	fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --test-api")
}

// writeOrganizations writes a formatted list of organizations to the specified file.
func writeOrganizations(w *os.File, orgs []meraki.Organization) {
	fmt.Fprintln(w, "Organizations:")
	for _, org := range orgs {
		fmt.Fprintf(w, "- %s (%s)\n", org.Name, org.ID)
	}
}

// writeNetworksForOrg writes a formatted list of networks for an organization to the specified file.
func writeNetworksForOrg(w *os.File, org meraki.Organization, networks []meraki.Network) {
	fmt.Fprintf(w, "Organization: %s (%s)\n", org.Name, org.ID)
	if len(networks) == 0 {
		fmt.Fprintln(w, "  (no networks)")
		return
	}
	for _, n := range networks {
		fmt.Fprintf(w, "  - %s (%s)\n", n.Name, n.ID)
	}
}

// printVersion writes version and build information to the specified file.
func printVersion(w *os.File) {
	fmt.Fprintf(w, "Find-Meraki-Ports-With-MAC version %s\n", Version)
	fmt.Fprintf(w, "  Commit:     %s\n", Commit)
	fmt.Fprintf(w, "  Build Time: %s\n", BuildTime)
	fmt.Fprintf(w, "  Go Version: %s\n", GoVersion)
	fmt.Fprintf(w, "  Repository: %s\n", RepositoryURL)
}
