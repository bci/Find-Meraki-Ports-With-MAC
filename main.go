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
	NetworkName  string // Network name filter or "ALL"
	OutputFormat string // Output format: csv, text, or html
	BaseURL      string // Meraki API base URL
	LogFile      string // Path to log file
	LogLevel     string // Log level: DEBUG, INFO, WARNING, ERROR
	Verbose      bool   // Enable verbose output
	SwitchFilter string // Switch name filter
	PortFilter   string // Port filter
	TestFull     bool   // Display complete MAC forwarding table
}

// Version information injected at build time via ldflags.
// Build with: go build -ldflags "-X main.Version=1.0.0 -X main.Commit=<git-sha> -X main.BuildTime=<timestamp>"
const (
	RepositoryURL = "https://github.com/bci/Find-Meraki-Ports-With-MAC"
)

var (
	Version   = "dev"     // Version set at build time
	Commit    = "unknown" // Git commit SHA set at build time
	BuildTime = "unknown" // Build timestamp set at build time
	GoVersion = "go1.21"  // Go version (can be updated at build time)
)

func main() {
	_ = godotenv.Load()

	macFlag := flag.String("mac", "", "MAC address or pattern")
	networkFlag := flag.String("network", "", "Network name or ALL")
	orgFlag := flag.String("org", "", "Organization name")
	outputFlag := flag.String("output-format", "", "Output format: csv, text, html")
	listOrgsFlag := flag.Bool("list-orgs", false, "List organizations the API key can access and exit")
	listNetworksFlag := flag.Bool("list-networks", false, "List networks per organization and exit")
	testAPIFlag := flag.Bool("test-api", false, "Validate API key and exit")
	testFullTableFlag := flag.Bool("test-full-table", false, "Display all MAC addresses in forwarding table (filtered by --switch/--port)")
	verboseFlag := flag.Bool("verbose", false, "Show search progress")
	switchFlag := flag.String("switch", "", "Filter by switch name (case-insensitive substring match)")
	portFlag := flag.String("port", "", "Filter by port name/number")
	logFileFlag := flag.String("log-file", "", "Log file path")
	logLevelFlag := flag.String("log-level", "", "Log level: DEBUG, INFO, WARNING, ERROR")
	versionFlag := flag.Bool("version", false, "Show version and exit")
	helpFlag := flag.Bool("help", false, "Show help")
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
		LogFile:      strings.TrimSpace(firstNonEmpty(*logFileFlag, os.Getenv("LOG_FILE"), "Find-Meraki-Ports-With-MAC.log")),
		LogLevel:     strings.TrimSpace(firstNonEmpty(*logLevelFlag, os.Getenv("LOG_LEVEL"), "DEBUG")),
		Verbose:      *verboseFlag,
		SwitchFilter: strings.TrimSpace(*switchFlag),
		PortFilter:   strings.TrimSpace(*portFlag),
		TestFull:     *testFullTableFlag,
	}

	if *helpFlag {
		printUsage(os.Stdout)
		return
	}

	if *versionFlag {
		printVersion(os.Stdout)
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

	client := meraki.NewClient(cfg.APIKey, cfg.BaseURL)
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
		if cfg.Verbose {
			log.Infof("Test full table mode enabled")
		}
	}

	if strings.TrimSpace(*macFlag) == "" {
		if !cfg.TestFull {
			exitWithError(log, "--mac is required")
		}
	}

	matcher := func(string) bool { return true }
	if strings.TrimSpace(*macFlag) != "" {
		var normalized string
		var isWildcard bool
		var err error
		matcher, normalized, isWildcard, err = macaddr.BuildMacMatcher(*macFlag)
		if err != nil {
			exitWithError(log, err.Error())
		}
		if cfg.Verbose {
			if isWildcard {
				log.Infof("MAC pattern: %s", strings.TrimSpace(*macFlag))
			} else {
				log.Infof("MAC: %s", normalized)
			}
		}
	}

	orgs, err := client.GetOrganizations(ctx)
	if err != nil {
		exitWithError(log, err.Error())
	}

	org, err := selectOrganization(cfg.OrgName, orgs)
	if err != nil {
		exitWithError(log, err.Error())
	}
	if cfg.Verbose {
		log.Infof("Organization: %s", org.Name)
	}

	networks, err := client.GetNetworks(ctx, org.ID)
	if err != nil {
		exitWithError(log, err.Error())
	}

	selectedNetworks, err := selectNetworks(cfg.NetworkName, networks)
	if err != nil {
		exitWithError(log, err.Error())
	}

	var results []output.ResultRow
	resultsIndex := make(map[string]struct{})
	for _, net := range selectedNetworks {
		if cfg.Verbose {
			log.Infof("Network: %s", net.Name)
		}

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
		if cfg.Verbose {
			log.Infof("Network clients API returned %d clients", len(networkClients))
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

				addResult(resultsIndex, &results, output.ResultRow{
					OrgName:      org.Name,
					NetworkName:  net.Name,
					SwitchName:   switchName,
					SwitchSerial: serial,
					Port:         port,
					MAC:          macaddr.FormatMacColon(normMAC),
					LastSeen:     c.LastSeen,
				})
			}
		}

		// Query device-level clients for each switch
		for _, dev := range switches {
			if cfg.Verbose {
				log.Infof("Querying switch: %s (%s)", firstNonEmpty(dev.Name, dev.Serial), dev.Serial)
			}

			// Try live tools MAC table lookup first (works for all switches including Catalyst)
			macTableID, err := client.CreateMacTableLookup(ctx, dev.Serial)
			if err == nil && macTableID != "" {
				if cfg.Verbose {
					log.Debugf("Created MAC table lookup job %s for %s", macTableID, dev.Serial)
				}

				// Poll for results (max 30 seconds)
				var macEntries []map[string]interface{}
				var status string
				for attempt := 0; attempt < 15; attempt++ {
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
						log.Debugf("MAC table lookup status for %s (%s) in network %s: %s (attempt %d/15)",
							firstNonEmpty(dev.Name, dev.Serial), dev.Serial, net.Name, status, attempt+1)
					}
				}

				if status == "complete" && len(macEntries) > 0 {
					if cfg.Verbose {
						log.Infof("Live MAC table returned %d entries for %s", len(macEntries), firstNonEmpty(dev.Name, dev.Serial))
					}

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

							if cfg.Verbose && portID == "" {
								log.Debugf("MAC entry fields: %+v", entry)
							}

							port := firstNonEmpty(portID, "unknown")
							if !filters.MatchesPortFilter(port, cfg.PortFilter) {
								continue
							}

							if cfg.Verbose {
								log.Debugf("Found MAC %s on %s port %s (VLAN %d) via live lookup",
									macaddr.FormatMacColon(normMAC), firstNonEmpty(dev.Name, dev.Serial), port, int(vlan))
							}

							addResult(resultsIndex, &results, output.ResultRow{
								OrgName:      org.Name,
								NetworkName:  net.Name,
								SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
								SwitchSerial: dev.Serial,
								Port:         port,
								MAC:          macaddr.FormatMacColon(normMAC),
								LastSeen:     "",
							})
						}
					}
					continue // Skip device clients API if live lookup succeeded
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

			if cfg.Verbose {
				log.Infof("Device clients API returned %d clients for %s", len(clients), firstNonEmpty(dev.Name, dev.Serial))
			}

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
					addResult(resultsIndex, &results, output.ResultRow{
						OrgName:      org.Name,
						NetworkName:  net.Name,
						SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
						SwitchSerial: dev.Serial,
						Port:         port,
						MAC:          macaddr.FormatMacColon(normMAC),
						LastSeen:     c.LastSeen,
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
		if org.Name == name {
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
		if net.Name == name {
			return []meraki.Network{net}, nil
		}
	}
	return nil, fmt.Errorf("network %q not found", name)
}

// addResult adds a result row to the results slice if it's not a duplicate.
// Deduplication is based on switch serial, port, MAC address, and last seen timestamp.
func addResult(index map[string]struct{}, rows *[]output.ResultRow, row output.ResultRow) {
	key := fmt.Sprintf("%s|%s|%s|%s", row.SwitchSerial, row.Port, row.MAC, row.LastSeen)
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
	fmt.Fprintln(w, "  --mac <mac|pattern>         MAC address or wildcard pattern (required unless using list/test flags)")
	fmt.Fprintln(w, "  --network <name|ALL>        Network name or ALL (default from .env)")
	fmt.Fprintln(w, "  --org <name>                Organization name (default from .env)")
	fmt.Fprintln(w, "  --output-format <csv|text|html>  Output format (default from .env)")
	fmt.Fprintln(w, "  --list-orgs                 List organizations and exit")
	fmt.Fprintln(w, "  --list-networks             List networks per organization and exit")
	fmt.Fprintln(w, "  --test-api                  Validate API key and exit")
	fmt.Fprintln(w, "  --test-full-table           Display all MACs in forwarding table (filters apply)")
	fmt.Fprintln(w, "  --switch <name>             Filter by switch name (case-insensitive substring)")
	fmt.Fprintln(w, "  --port <number>             Filter by port name/number")
	fmt.Fprintln(w, "  --verbose                   Show search progress")
	fmt.Fprintln(w, "  --log-file <filename>        Log file path (default from .env)")
	fmt.Fprintln(w, "  --log-level <DEBUG|INFO|WARNING|ERROR>  Log level (default from .env)")
	fmt.Fprintln(w, "  --version                   Show version and exit")
	fmt.Fprintln(w, "  --help                      Show this help")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Environment:")
	fmt.Fprintln(w, "  MERAKI_API_KEY     Meraki Dashboard API key (required)")
	fmt.Fprintln(w, "  MERAKI_ORG         Default org name")
	fmt.Fprintln(w, "  MERAKI_NETWORK     Default network name or ALL")
	fmt.Fprintln(w, "  OUTPUT_FORMAT      csv | text | html")
	fmt.Fprintln(w, "  MERAKI_BASE_URL    API base URL (default https://api.meraki.com/api/v1)")
	fmt.Fprintln(w, "  LOG_FILE           Log file path (default Find-Meraki-Ports-With-MAC.log)")
	fmt.Fprintln(w, "  LOG_LEVEL          DEBUG | INFO | WARNING | ERROR")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
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
