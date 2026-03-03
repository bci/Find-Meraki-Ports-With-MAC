# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0] - 2026-03-02

### Added
- **Demo Mode (`--test-data`)**: New flag launches the web interface with sanitised demo data — no API key required. Useful for screenshots, training, and evaluations. Auto-populates the MAC field and triggers an automatic resolve on load.
- **Live Demo Log Streaming**: In `--test-data` mode the resolve handler streams realistic per-switch log messages over WebSocket (in a goroutine) to simulate a live scan, including IP resolution, per-network progress, hit/miss per switch, and a final summary.
- **IP Back-fill**: After a MAC-based resolve, the resolved IP address is now written back into the IP input field, mirroring the existing MAC back-fill behaviour when searching by IP.

### Fixed
- **Uplink Detection**: Replaced the LLDP/CDP-based uplink detection with the `/devices/{serial}/switch/ports/statuses` API `isUplink` field. The previous approach missed ports whose neighbours send no LLDP/CDP frames (e.g. ISP fibre routers). The new approach mirrors exactly what the Meraki Dashboard shows.
- **Third-party Switch Uplinks**: Removed the `meraki.com` URL gate that prevented third-party upstream switches (e.g. GTrans core) from being recognised as uplinks.
- **AP False-positives on Uplink Detection**: After removing the URL gate, APs advertising `"Router, Switch"` in CDP were incorrectly flagged as switch uplinks. Fixed by requiring CDP `"Switch"` without `"Router"`, and checking LLDP `systemCapabilities` first (S-VLAN/Bridge = switch; Two-port MAC Relay = AP).
- **Mode Column Sorting**: The mode column sort now correctly distinguishes `uplink` from `trunk` rows. Previously `_colValue('mode')` returned the raw `portMode` value for uplink rows, so uplink and trunk sorted identically.
- **Demo Manufacturer Lookup**: The demo MAC `a4:c3:f0:85:1d:3e` has an Intel OUI in the IEEE registry. In `--test-data` mode the `/api/manufacturer` endpoint now returns `Apple` to match the manufacturer shown in the results table.

### Changed
- **Demo Data Model**: Demo results now show a single MAC address found across one primary network (HQ Campus, access + uplink hops) plus two additional networks where the MAC is seen on uplink/border ports only — a physically valid topology.
- **Demo Constants Consolidated**: `demoMAC`, `demoIP`, `demoHostname`, `demoOrg`, `demoMfr`, `demoOUI` extracted as package-level constants shared by all demo handlers, eliminating duplicated string literals.
- **Uplink Note**: Removed the list of switch names from the uplink advisory callout in the results table.



### Added
- **Interactive Web UI**: New `--interactive` flag launches a local HTTP server with a single-page web interface for Meraki port/device lookup
- **Web Presets**: CLI flags `--mac`, `--ip`, `--org`, `--network` pre-fill the web UI and trigger automatic resolution when all required context is available
- **Web Server Flags**: `--web-port` (default `8080`) and `--web-host` (default `localhost`) control the server binding
- **Real-Time Logs**: WebSocket endpoint (`/ws/logs`) streams live log events to the browser log console
- **Uplink/Trunk Advisory**: Results table shows a callout when the MAC is seen on trunk ports, with an extra warning when all results are trunk ports
- **Port Mode Column**: Results include `portMode` (`access`/`trunk`) in all output formats
- **DNS & Override Flags**: `--dns-servers` and `HOST_OVERRIDES` env var for PTR lookups against internal DNS

### Fixed
- **MacTablePoll=0 in web path**: `handleResolve` was building a `Config` with `MacTablePoll: 0`, causing live MAC table polls to never run and returning empty results
- **Browser caching**: Static files (`app.js`, `style.css`) now served with `Cache-Control: no-store` so code changes take effect immediately
- **Stale saved org/network**: Saved localStorage org/network IDs are now validated against the current API key's org/network list before use

## [1.1.0] - 2026-02-19

### Added
- **IP Address Resolution**: New `--ip` flag to resolve IP addresses to MAC addresses via Meraki API
- **Hostname Resolution**: Automatic reverse DNS lookup with 5-second timeout for resolved IPs
- **Enhanced Output**: IP and Hostname columns added to all output formats (CSV, text, HTML)
- **Logger Bug Fix**: Fixed critical bug preventing DEBUG messages with `--verbose` flag
- **Mutual Exclusivity**: `--ip` and `--mac` flags are mutually exclusive for clear operation
- **Comprehensive Testing**: Added unit tests for IP resolution and hostname lookup functionality

### Technical Improvements
- **API Integration**: Extended Meraki client with `ResolveIPToMAC()` and `ResolveHostname()` methods
- **Error Handling**: Robust timeout handling for DNS resolution
- **Logging Enhancement**: Fixed log level comparison logic for proper DEBUG/INFO filtering
- **Output Writers**: Updated all formatters to include IP and hostname data
- **Test Coverage**: Added `TestResolveHostname()` function for DNS resolution testing

### Examples
```bash
# Resolve IP to MAC and find switch port
Find-Meraki-Ports-With-MAC --ip 172.17.4.1 --verbose

# Output includes IP and hostname columns
Org,Network,Switch,Serial,Port,MAC,IP,Hostname,LastSeen
MyOrg,Network1,Switch1,XXXX-XXXX-XXXX,24,40:a6:b7:a5:3b:e0,172.17.4.1,itconsole.ci.gardena.ca.us,
```

## [1.0.0] - 2026-02-13

### Added
- **Initial Release**: Complete CLI tool for finding Meraki switch ports by MAC address
- **Multi-switch Support**: Works with Meraki MS series and Cisco Catalyst switches managed by Meraki
- **MAC Address Resolution**: Find which switch and port a device is connected to using its MAC address
- **Flexible MAC Input**: Support for multiple MAC address formats:
  - Colon-separated: `00:11:22:33:44:55`
  - Hexadecimal: `08f1b36f9c25`
  - Dot-separated: `08.f1.b3.6f.9c.25`
  - Mixed notation: `08f1.b36f.9c25`
- **Wildcard Patterns**: Support for MAC address patterns like `08:f1:b3:6f:9c:*` and bracket notation
- **Multiple Output Formats**:
  - CSV (comma-separated values)
  - Text (human-readable table)
  - HTML (web-viewable table)
- **Advanced Filtering Options**:
  - Organization filtering (`--org`)
  - Network filtering (`--network`)
  - Switch name filtering (`--switch`, case-insensitive substring match)
  - Port number/name filtering (`--port`)
- **API Integration**: Uses Meraki Dashboard API v1 endpoints:
  - Organizations, networks, and devices enumeration
  - Network and device-level client information
  - Live MAC table lookups for Catalyst switches
- **Troubleshooting Features**:
  - `--list-orgs`: List accessible organizations
  - `--list-networks`: List networks per organization
  - `--test-api`: Validate API key connectivity
  - `--test-full-table`: Display complete forwarding tables
- **Comprehensive Logging**:
  - Configurable log levels (DEBUG, INFO, WARNING, ERROR)
  - File and console output options
  - Verbose mode for search progress tracking
- **Configuration Management**: Environment file support (`.env`) for API keys and defaults
- **Cross-Platform Builds**: Automated build scripts for Windows, macOS, and Linux
- **Version Information**: `--version` flag showing version, commit, build time, and Go version
- **Production-Ready**: Static binaries with no runtime dependencies

### Technical Features
- **Go 1.21**: Modern Go implementation with standard library usage
- **godotenv**: Configuration management for environment variables
- **Modular Architecture**: Separate packages for API client, output formatting, MAC utilities, and logging
- **Comprehensive Testing**: Unit tests for all major components
- **Build Optimization**: Stripped binaries for minimal size
- **Error Handling**: Robust error handling with informative messages

### Use Cases Supported
- **Security & Compliance**: Rogue device detection, security incident response, compliance audits
- **Network Troubleshooting**: Duplicate IP resolution, MAC flapping detection, connectivity issues
- **Operations & Inventory**: VLAN verification, port security monitoring, device inventory management

### Documentation
- Complete README with usage examples and API documentation
- Environment configuration guide
- Build instructions for multiple platforms
- Sample output and troubleshooting examples

### Authors
- Kent Behrends
- GitHub Copilot Agents

---

## Development Notes

This version represents the initial stable release of the Find-Meraki-Ports-With-MAC tool. Future versions will include additional features such as IP address resolution, enhanced filtering options, and performance improvements.

For more information, see the [README](README.md) and [repository](https://github.com/bci/Find-Meraki-Ports-With-MAC).

[1.3.0]: https://github.com/bci/Find-Meraki-Ports-With-MAC/releases/tag/v1.3.0
[1.2.0]: https://github.com/bci/Find-Meraki-Ports-With-MAC/releases/tag/v1.2.0
[1.1.0]: https://github.com/bci/Find-Meraki-Ports-With-MAC/releases/tag/v1.1.0
[1.0.0]: https://github.com/bci/Find-Meraki-Ports-With-MAC/releases/tag/v1.0.0</content>
<parameter name="filePath">c:\Users\kent.behrends\Documents\GitHub\Find-Meraki-Ports-With-MAC\Find-Meraki-Ports-With-MAC\CHANGELOG.md