# Find-Meraki-Ports-With-MAC

[![Tests](https://github.com/bci/Find-Meraki-Ports-With-MAC/workflows/Tests/badge.svg)](https://github.com/bci/Find-Meraki-Ports-With-MAC/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/bci/Find-Meraki-Ports-With-MAC)](https://goreportcard.com/report/github.com/bci/Find-Meraki-Ports-With-MAC)
[![codecov](https://codecov.io/gh/bci/Find-Meraki-Ports-With-MAC/branch/main/graph/badge.svg)](https://codecov.io/gh/bci/Find-Meraki-Ports-With-MAC)
[![License: Unlicense](https://img.shields.io/badge/license-Unlicense-blue.svg)](http://unlicense.org/)
[![GitHub release](https://img.shields.io/github/release/bci/Find-Meraki-Ports-With-MAC.svg)](https://github.com/bci/Find-Meraki-Ports-With-MAC/releases)

A Go CLI tool that queries the Meraki Dashboard API to find which switch and port a MAC address is associated with. Supports both Meraki MS switches and Cisco Catalyst switches managed by Meraki.

## Use Cases

This tool helps network administrators quickly locate devices and troubleshoot network issues:

**Security & Compliance:**
- **Rogue DHCP Server Detection** - Locate unauthorized devices offering DHCP addresses on the network
- **Rogue Access Point Detection** - Find unauthorized wireless access points connected to switch ports
- **Security Incident Response** - Track down devices involved in security events or policy violations
- **MAC Address Spoofing Investigation** - Verify physical location of devices claiming specific MAC addresses
- **Compliance Audits** - Verify device locations and port assignments match documentation

**Network Troubleshooting:**
- **Duplicate IP Address Resolution** - Locate devices with conflicting IP addresses by their MAC
- **MAC Flapping Detection** - Find devices appearing on multiple ports (potential network loops or misconfigurations)
- **Broadcast/Multicast Storm Sources** - Identify the switch port where problematic traffic originates
- **Network Loop Detection** - Locate ports involved in spanning tree issues or physical loops
- **Connectivity Troubleshooting** - Verify which port a specific device is connected to

**Operations & Inventory:**
- **VLAN Membership Verification** - Confirm devices are on the correct port and VLAN
- **Port Security Violations** - Locate devices triggering port security alerts
- **Device Inventory** - Build accurate physical topology maps of connected devices
- **Change Management** - Verify devices are moved to correct ports after infrastructure changes

## Switch Families Tested

- **Meraki MS Series**: MS120, MS125, MS210, MS220, MS225, MS250, MS350, MS355, MS390, MS410, MS425, MS450
- **Cisco Catalyst (Meraki-managed)**: C9300 series

## APIs Used

This tool utilizes the following Meraki Dashboard API v1 endpoints:

- `GET /organizations` - List organizations accessible by the API key
- `GET /organizations/{organizationId}/networks` - List networks in an organization
- `GET /networks/{networkId}/devices` - List all devices in a network
- `GET /networks/{networkId}/clients` - Get network-level client information
- `GET /devices/{serial}/clients` - Get device-level client information (fallback)
- `POST /devices/{serial}/liveTools/macTable` - Initiate live MAC table lookup (critical for Catalyst switches)
- `GET /devices/{serial}/liveTools/macTable/{macTableId}` - Poll for MAC table lookup results

The live MAC table lookup is essential for Cisco Catalyst switches managed by Meraki, as standard client endpoints may have limited visibility.

## Usage

```
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL --org "My Org" --output-format csv
```

Filter by switch/port:

```
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network "City" --switch ccc9300xa --port 3
```

Dump full forwarding table (filtered by switch/port if provided):

```
Find-Meraki-Ports-With-MAC.exe --test-full-table --network "City" --switch ccc9300xa
```

Troubleshooting:

```
Find-Meraki-Ports-With-MAC.exe --list-orgs
Find-Meraki-Ports-With-MAC.exe --list-networks --org "My Org"
Find-Meraki-Ports-With-MAC.exe --test-api
```

Help:

```
Find-Meraki-Ports-With-MAC.exe --help
```

Version:

```
Find-Meraki-Ports-With-MAC.exe --version
```

Output:
```
Find-Meraki-Ports-With-MAC version 1.0.0
  Commit:     a1b2c3d
  Build Time: 2024-01-15T10:30:00Z
  Go Version: go1.21
  Repository: https://github.com/bci/Find-Meraki-Ports-With-MAC
```

### MAC formats

Accepted formats:

- 00:11:22:33:44:55
- 08f1b36f9c25
- 08.f1.b3.6f.9c.25
- 08f1.b36f.9c25

Wildcard patterns:

- 08:f1:b3:6f:9c:*
- 08:f1:b3:6f:9c:[1-4][0-f]

## Environment (.env)

Create a `.env` file (see `.env.example`).

- MERAKI_API_KEY: required
- MERAKI_ORG: default org name if --org is not provided
- MERAKI_NETWORK: default network name or ALL
- OUTPUT_FORMAT: csv | text | html
- MERAKI_BASE_URL: optional (defaults to https://api.meraki.com/api/v1)
- LOG_FILE: log file path (default Find-Meraki-Ports-With-MAC.log)
- LOG_LEVEL: DEBUG | INFO | WARNING | ERROR

## Flags

**Required:**
- --mac: MAC address or wildcard pattern (required unless using list/test flags)

**Filtering:**
- --org: organization name (default from .env)
- --network: network name or ALL (default from .env)
- --switch: filter by switch name (case-insensitive substring)
- --port: filter by port name/number

**Output:**
- --output-format: csv | text | html (default from .env)

**Troubleshooting & Testing:**
- --list-orgs: list organizations the API key can access
- --list-networks: list networks per organization
- --test-api: validate the API key
- --test-full-table: display all MACs in forwarding table (filters apply)
- --verbose: show search progress (org, network, switch)

**Logging:**
- --log-file: log file path (default from .env)
- --log-level: DEBUG | INFO | WARNING | ERROR

**Information:**
- --version: show version, commit, build time, and repository URL
- --help: show usage, flags, env vars, and examples

## Output formats

- csv (default)
- text
- html

## Notes

- This tool uses the Meraki Dashboard API and enumerates switch devices in the selected network(s).
- Client MAC visibility depends on the Meraki API data available for the switches.

## Installation

### From source

```bash
go install github.com/BEHRConsulting/Find-Meraki-Ports-With-MAC@latest
```

### Build from source

```bash
git clone https://github.com/BEHRConsulting/Find-Meraki-Ports-With-MAC.git
cd Find-Meraki-Ports-With-MAC
go build -o Find-Meraki-Ports-With-MAC.exe .
```

### Build for multiple platforms

Use the provided build script to create binaries for Windows, macOS, and Linux:

**PowerShell (Windows):**
```powershell
.\build.ps1
```

**Bash (macOS/Linux):**
```bash
chmod +x build.sh
./build.sh
```

**Make (macOS/Linux):**
```bash
make build-all
# Or build for specific platforms:
make build-darwin   # macOS only
make build-linux    # Linux only
make build-windows  # Windows only (requires Go cross-compilation)
```

Built binaries will be placed in the `bin/` directory.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This is free and unencumbered software released into the public domain. See the [UNLICENSE](UNLICENSE) file for details.

## Authors

- Kent Behrends
- GitHub Copilot Agents

## Repository

https://github.com/BEHRConsulting/Find-Meraki-Ports-With-MAC
