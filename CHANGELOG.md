# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-02-19

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

For more information, see the [README](README.md) and [repository](https://github.com/bci/Find-Meraki-Ports-With-MAC).</content>
<parameter name="filePath">c:\Users\kent.behrends\Documents\GitHub\Find-Meraki-Ports-With-MAC\Find-Meraki-Ports-With-MAC\CHANGELOG.md