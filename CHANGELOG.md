# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.1.0]: https://github.com/bci/Find-Meraki-Ports-With-MAC/releases/tag/v1.1.0
[1.0.0]: https://github.com/bci/Find-Meraki-Ports-With-MAC/releases/tag/v1.0.0</content>
<parameter name="filePath">c:\Users\kent.behrends\Documents\GitHub\Find-Meraki-Ports-With-MAC\Find-Meraki-Ports-With-MAC\CHANGELOG.md