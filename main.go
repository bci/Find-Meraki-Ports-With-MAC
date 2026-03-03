// Package main provides a command-line tool for finding MAC addresses on Meraki switches.
// It supports both native Meraki MS switches and Cisco Catalyst switches managed by Meraki.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"Find-Meraki-Ports-With-MAC/pkg/filters"
	"Find-Meraki-Ports-With-MAC/pkg/logger"
	"Find-Meraki-Ports-With-MAC/pkg/macaddr"
	"Find-Meraki-Ports-With-MAC/pkg/meraki"
	"Find-Meraki-Ports-With-MAC/pkg/output"

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
	DNSServers   string // Comma-separated alternate DNS servers for PTR lookups
	LogFile      string // Path to log file
	LogLevel     string // Log level: DEBUG, INFO, WARNING, ERROR
	Verbose      bool   // Enable verbose output
	SwitchFilter string // Switch name filter
	PortFilter   string // Port filter
	TestFull     bool   // Display complete MAC forwarding table
	IPAddress    string // IP address to resolve
	MACAddress   string // MAC address or pattern to look up
}

// Version information injected at build time via ldflags.
// Build with: go build -ldflags "-X main.Version=1.0.0 -X main.Commit=<git-sha> -X main.BuildTime=<timestamp>"
const (
	RepositoryURL = "https://github.com/bci/Find-Meraki-Ports-With-MAC"
)

var (
	Version          = "dev"     // Version set at build time
	Commit           = "unknown" // Git commit SHA set at build time
	BuildTime        = "unknown" // Build timestamp set at build time
	GoVersion        = "go1.21"  // Go version (can be updated at build time)
	webAPIKey        string      // API key pre-loaded from .env for the web interface
	webPresetMAC     string      // pre-filled MAC from CLI --mac
	webPresetIP      string      // pre-filled IP from CLI --ip
	webPresetOrgName string      // pre-selected org name from CLI --org
	webPresetNetwork string      // pre-selected network name from CLI --network
	webTestDataMode  bool        // --test-data: serve sanitised demo data, no API calls
)

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
	dnsServersFlag := flag.String("dns-servers", "", "Comma-separated DNS servers for PTR lookups (e.g. 192.168.1.1,192.168.1.2)")
	webPortFlag := flag.String("web-port", "", "Port for web server (default: 8080)")
	webHostFlag := flag.String("web-host", "", "Host for web server (default: localhost)")
	testDataFlag := flag.Bool("test-data", false, "Launch web interface with sanitised demo data (no API key required)")
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
		DNSServers:   strings.TrimSpace(firstNonEmpty(*dnsServersFlag, os.Getenv("DNS_SERVERS"))),
		LogFile:      strings.TrimSpace(firstNonEmpty(*logFileFlag, os.Getenv("LOG_FILE"), "Find-Meraki-Ports-With-MAC.log")),
		LogLevel:     strings.TrimSpace(firstNonEmpty(*logLevelFlag, os.Getenv("LOG_LEVEL"), "DEBUG")),
		Verbose:      *verboseFlag,
		SwitchFilter: strings.TrimSpace(*switchFlag),
		PortFilter:   strings.TrimSpace(*portFlag),
		TestFull:     *testFullTableFlag,
		IPAddress:    strings.TrimSpace(*ipFlag),
		MACAddress:   strings.TrimSpace(*macFlag),
	}

	// If verbose flag is set, override log level to DEBUG and send logs to console
	if *verboseFlag {
		cfg.LogLevel = "DEBUG"
		cfg.LogFile = "" // Empty log file sends logs to console only
		fmt.Printf("DEBUG: Verbose flag set, LogLevel=%s, LogFile='%s'\n", cfg.LogLevel, cfg.LogFile)
	}

	// Configure alternate DNS servers for PTR hostname lookups.
	if cfg.DNSServers != "" {
		meraki.SetDNSServers(strings.Split(cfg.DNSServers, ","))
	}

	// Configure static IP→hostname overrides (for when internal DNS is unreachable).
	if v := strings.TrimSpace(os.Getenv("HOST_OVERRIDES")); v != "" {
		meraki.SetHostOverrides(v)
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
	if *interactiveFlag || *testDataFlag {
		webTestDataMode = *testDataFlag
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
		_, _ = fmt.Fprintf(os.Stdout, "API OK: %d organizations found\n", len(orgs))
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
			exitWithError(log, "--ip or --mac is required (or use --interactive to launch the web interface)")
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
	var cliAggrCache map[string]map[string][]string
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

		// Fetch topology to identify true uplink ports; failure is non-fatal.
		// Pre-populate AGGR cache from network-level link aggregations API (reliable source for AGGR/N membership).
		cliAggrCache = client.GetNetworkLinkAggregations(ctx, net.ID)
		// Build uplink set using LLDP/CDP per switch (topology API lacks port IDs on this firmware).
		cliUplinkPortCache := make(map[string]map[string]struct{})
		cliGetUplinkPorts := func(serial string) map[string]struct{} {
			if _, ok := cliUplinkPortCache[serial]; !ok {
				cliUplinkPortCache[serial] = client.GetDeviceUplinkPorts(ctx, serial)
			}
			return cliUplinkPortCache[serial]
		}

		// Query network-level clients
		networkClients, err := client.GetNetworkClients(ctx, net.ID)
		if err != nil {
			exitWithError(log, err.Error())
		}
		log.Debugf("Network clients API returned %d clients", len(networkClients))

		// Build MAC→IP/hostname/lastSeen maps for enriching results from live table / device clients.
		macToIP := make(map[string]string, len(networkClients))
		macToLastSeen := make(map[string]string, len(networkClients))
		macToHostname := make(map[string]string, len(networkClients))
		for _, nc := range networkClients {
			norm, err2 := macaddr.NormalizeExactMac(nc.MAC)
			if err2 != nil {
				continue
			}
			if nc.IP != "" {
				macToIP[norm] = nc.IP
			}
			if nc.LastSeen != "" {
				if existing := macToLastSeen[norm]; existing == "" || nc.LastSeen > existing {
					macToLastSeen[norm] = nc.LastSeen
				}
			}
			if hn := meraki.ClientHostname(nc); hn != "" {
				macToHostname[norm] = hn
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
			if hn == "" {
				hn = macToHostname[normMAC]
			}
			if hn == "" && ip != "" {
				if hn = meraki.LookupHostOverride(ip, org.Name, net.Name); hn == "" {
					hn, _ = meraki.ResolveHostname(ip)
				}
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

				aggrMembers := resolveAggrPorts(ctx, client, serial, port, cliAggrCache)
				vlan, portMode := enrichPortInfoWithMembers(ctx, client, serial, port, aggrMembers, 0, "")

				ip, hn := ipAndHostname(normMAC, c.IP, serial)
				addResult(resultsIndex, &results, output.ResultRow{
					OrgName:      org.Name,
					NetworkName:  net.Name,
					SwitchName:   switchName,
					SwitchSerial: serial,
					Port:         port,
					AggrPorts:    aggrMembers,
					MAC:          macaddr.FormatMacColon(normMAC),
					IP:           ip,
					Hostname:     hn,
					LastSeen:     firstNonEmpty(c.LastSeen, macToLastSeen[normMAC]),
					VLAN:         vlan,
					PortMode:     portMode,
					IsUplink:     isPortUplink(port, aggrMembers, cliGetUplinkPorts(serial)),
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

							// Normalize AGGR raw strings (e.g. "AGGR/0=serial/49,...") to clean ID
							cleanPortID, aggrMembers := parseAggrPort(firstNonEmpty(portID, "unknown"))
							port := cleanPortID
							if !filters.MatchesPortFilter(port, cfg.PortFilter) {
								continue
							}

							// If not already parsed from the raw string, try API/cache lookup
							if aggrMembers == nil {
								aggrMembers = resolveAggrPorts(ctx, client, dev.Serial, port, cliAggrCache)
							}

							// Enrich with switch port API (authoritative VLAN + mode); for AGGR use first member
							richVLAN, richMode := enrichPortInfoWithMembers(ctx, client, dev.Serial, port, aggrMembers, int(vlan), portMode)

							if cfg.Verbose {
								log.Debugf("Found MAC %s on %s port %s (VLAN %d, mode=%s) via live lookup",
									macaddr.FormatMacColon(normMAC), firstNonEmpty(dev.Name, dev.Serial), port, richVLAN, richMode)
							}

							ip, hn := ipAndHostname(normMAC, "", dev.Serial)
							_, isUplink := cliGetUplinkPorts(dev.Serial)[port]
							addResult(resultsIndex, &results, output.ResultRow{
								OrgName:      org.Name,
								NetworkName:  net.Name,
								SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
								SwitchSerial: dev.Serial,
								Port:         port,
								AggrPorts:    aggrMembers,
								MAC:          macaddr.FormatMacColon(normMAC),
								IP:           ip,
								Hostname:     hn,
								LastSeen:     macToLastSeen[normMAC],
								VLAN:         richVLAN,
								PortMode:     richMode,
								IsUplink:     isUplink,
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
					aggrMembers2 := resolveAggrPorts(ctx, client, dev.Serial, port, cliAggrCache)
					vlan, portMode := enrichPortInfoWithMembers(ctx, client, dev.Serial, port, aggrMembers2, 0, "")
					ip, hn := ipAndHostname(normMAC, "", dev.Serial)
					addResult(resultsIndex, &results, output.ResultRow{
						OrgName:      org.Name,
						NetworkName:  net.Name,
						SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
						SwitchSerial: dev.Serial,
						Port:         port,
						AggrPorts:    aggrMembers2,
						MAC:          macaddr.FormatMacColon(normMAC),
						IP:           ip,
						Hostname:     hn,
						LastSeen:     c.LastSeen,
						VLAN:         vlan,
						PortMode:     portMode,
						IsUplink:     isPortUplink(port, aggrMembers2, cliGetUplinkPorts(dev.Serial)),
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

// ── Utility helpers ───────────────────────────────────────────────────────────

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

// ── CLI output helpers ────────────────────────────────────────────────────────

// printUsage writes comprehensive help text to the specified file.
// Includes all command-line flags, environment variables, and usage examples.
func printUsage(w *os.File) {
	_, _ = fmt.Fprintln(w, "Find-Meraki-Ports-With-MAC - Meraki MAC lookup")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL --org \"My Org\" --output-format csv")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Flags:")
	_, _ = fmt.Fprintln(w, "  --ip <address>              IP address to resolve to MAC (mutually exclusive with --mac)")
	_, _ = fmt.Fprintln(w, "  --mac <mac|pattern>         MAC address or wildcard pattern (required unless using list/test flags)")
	_, _ = fmt.Fprintln(w, "  --network <name|ALL>        Network name or ALL (default from .env)")
	_, _ = fmt.Fprintln(w, "  --org <name>                Organization name (optional if only one org accessible)")
	_, _ = fmt.Fprintln(w, "  --output-format <csv|text|html>  Output format (default from .env)")
	_, _ = fmt.Fprintln(w, "  --list-orgs                 List organizations and exit")
	_, _ = fmt.Fprintln(w, "  --list-networks             List networks per organization and exit")
	_, _ = fmt.Fprintln(w, "  --test-api                  Validate API key and exit")
	_, _ = fmt.Fprintln(w, "  --test-full-table           Display all MACs in forwarding table (filters apply)")
	_, _ = fmt.Fprintln(w, "  --switch <name>             Filter by switch name (case-insensitive substring)")
	_, _ = fmt.Fprintln(w, "  --port <number>             Filter by port name/number")
	_, _ = fmt.Fprintln(w, "  --verbose                   Send DEBUG logs to console (overrides --log-level and --log-file)")
	_, _ = fmt.Fprintln(w, "  --log-file <filename>        Log file path (default from .env)")
	_, _ = fmt.Fprintln(w, "  --log-level <DEBUG|INFO|WARNING|ERROR>  Log level (default from .env)")
	_, _ = fmt.Fprintln(w, "  --retry <n>                 Max API retry attempts on rate limit (default: 6)")
	_, _ = fmt.Fprintln(w, "  --mac-table-poll <n>        MAC table lookup poll attempts, 2s each (default: 15)")
	_, _ = fmt.Fprintln(w, "  --dns-servers <addr,...>    Comma-separated DNS servers for PTR lookups")
	_, _ = fmt.Fprintln(w, "  --interactive               Launch interactive web interface")
	_, _ = fmt.Fprintln(w, "  --web-port <port>           Web server port (default: 8080)")
	_, _ = fmt.Fprintln(w, "  --web-host <host>           Web server host (default: localhost)")
	_, _ = fmt.Fprintln(w, "  --version                   Show version and exit")
	_, _ = fmt.Fprintln(w, "  --help                      Show this help")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Environment:")
	_, _ = fmt.Fprintln(w, "  MERAKI_API_KEY     Meraki Dashboard API key (required)")
	_, _ = fmt.Fprintln(w, "  MERAKI_ORG         Default org name")
	_, _ = fmt.Fprintln(w, "  MERAKI_NETWORK     Default network name or ALL")
	_, _ = fmt.Fprintln(w, "  OUTPUT_FORMAT      csv | text | html")
	_, _ = fmt.Fprintln(w, "  MERAKI_BASE_URL    API base URL (default https://api.meraki.com/api/v1)")
	_, _ = fmt.Fprintln(w, "  MERAKI_RETRIES     Max API retry attempts on rate limit (default 6)")
	_, _ = fmt.Fprintln(w, "  MERAKI_MAC_POLL    MAC table lookup poll attempts, 2s each (default 15)")
	_, _ = fmt.Fprintln(w, "  DNS_SERVERS        Comma-separated DNS servers for PTR lookups")
	_, _ = fmt.Fprintln(w, "  LOG_FILE           Log file path (default Find-Meraki-Ports-With-MAC.log)")
	_, _ = fmt.Fprintln(w, "  LOG_LEVEL          DEBUG | INFO | WARNING | ERROR")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Examples:")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --ip 192.168.1.100 --network ALL")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --mac 08:f1:b3:6f:9c:* --output-format text")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --test-full-table --network City --switch ccc9300xa")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --test-full-table --network City --switch ccc9300xa --port 3")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --list-orgs")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --list-networks --org \"My Org\"")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --test-api")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --interactive")
	_, _ = fmt.Fprintln(w, "  Find-Meraki-Ports-With-MAC.exe --interactive --web-port 9090")
}

// writeOrganizations writes a formatted list of organizations to the specified file.
func writeOrganizations(w *os.File, orgs []meraki.Organization) {
	_, _ = fmt.Fprintln(w, "Organizations:")
	for _, org := range orgs {
		_, _ = fmt.Fprintf(w, "- %s (%s)\n", org.Name, org.ID)
	}
}

// writeNetworksForOrg writes a formatted list of networks for an organization to the specified file.
func writeNetworksForOrg(w *os.File, org meraki.Organization, networks []meraki.Network) {
	_, _ = fmt.Fprintf(w, "Organization: %s (%s)\n", org.Name, org.ID)
	if len(networks) == 0 {
		_, _ = fmt.Fprintln(w, "  (no networks)")
		return
	}
	for _, n := range networks {
		_, _ = fmt.Fprintf(w, "  - %s (%s)\n", n.Name, n.ID)
	}
}

// printVersion writes version and build information to the specified file.
func printVersion(w *os.File) {
	_, _ = fmt.Fprintf(w, "Find-Meraki-Ports-With-MAC version %s\n", Version)
	_, _ = fmt.Fprintf(w, "  Commit:     %s\n", Commit)
	_, _ = fmt.Fprintf(w, "  Build Time: %s\n", BuildTime)
	_, _ = fmt.Fprintf(w, "  Go Version: %s\n", GoVersion)
	_, _ = fmt.Fprintf(w, "  Repository: %s\n", RepositoryURL)
}
