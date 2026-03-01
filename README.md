# Find-Meraki-Ports-With-MAC

[![Tests](https://github.com/bci/Find-Meraki-Ports-With-MAC/workflows/Tests/badge.svg)](https://github.com/bci/Find-Meraki-Ports-With-MAC/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/bci/Find-Meraki-Ports-With-MAC)](https://goreportcard.com/report/github.com/bci/Find-Meraki-Ports-With-MAC)
[![codecov](https://codecov.io/gh/bci/Find-Meraki-Ports-With-MAC/branch/main/graph/badge.svg)](https://codecov.io/gh/bci/Find-Meraki-Ports-With-MAC)
[![License: Unlicense](https://img.shields.io/badge/license-Unlicense-blue.svg)](http://unlicense.org/)
[![GitHub release](https://img.shields.io/github/release/bci/Find-Meraki-Ports-With-MAC.svg)](https://github.com/bci/Find-Meraki-Ports-With-MAC/releases)

A Go CLI tool that queries the Meraki Dashboard API to find which switch and port a MAC address or IP address is associated with. Supports both Meraki MS switches and Cisco Catalyst switches managed by Meraki.

## Use Cases

This tool helps network administrators quickly locate devices and troubleshoot network issues:

**Security & Compliance:**
- **Rogue DHCP Server Detection** - Locate unauthorized devices offering DHCP addresses on the network
- **Rogue Access Point Detection** - Find unauthorized wireless access points connected to switch ports
- **Security Incident Response** - Track down devices involved in security events or policy violations
- **MAC Address Spoofing Investigation** - Verify physical location of devices claiming specific MAC addresses
- **Compliance Audits** - Verify device locations and port assignments match documentation

**Network Troubleshooting:**
- **Duplicate IP Address Resolution** - Locate devices with conflicting IP addresses by their IP
- **IP Address Investigation** - Find switch ports for devices by IP address instead of MAC
- **MAC Flapping Detection** - Find devices appearing on multiple ports (potential network loops or misconfigurations)
- **Broadcast/Multicast Storm Sources** - Identify the switch port where problematic traffic originates
- **Network Loop Detection** - Locate ports involved in spanning tree issues or physical loops
- **Connectivity Troubleshooting** - Verify which port a specific device is connected to by IP or MAC

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
- `GET /networks/{networkId}/clients` - Get network-level client information (includes IP-to-MAC mappings)
- `GET /devices/{serial}/clients` - Get device-level client information (fallback)
- `POST /devices/{serial}/liveTools/macTable` - Initiate live MAC table lookup (critical for Catalyst switches)
- `GET /devices/{serial}/liveTools/macTable/{macTableId}` - Poll for MAC table lookup results

The live MAC table lookup is essential for Cisco Catalyst switches managed by Meraki, as standard client endpoints may have limited visibility. The clients API provides IP-to-MAC resolution for IP-based lookups.

## Usage

```
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL --org "My Org" --output-format csv
```

Find by IP address:

```
Find-Meraki-Ports-With-MAC.exe --ip 192.168.1.100 --network ALL --org "My Org" --output-format csv
```

Filter by switch/port:

```
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network "City" --switch ccc9300xa --port 3
```

Dump full forwarding table (filtered by switch/port if provided):

```
Find-Meraki-Ports-With-MAC.exe --test-full-table --network "City" --switch ccc9300xa
```

Verbose logging to console:

```
Find-Meraki-Ports-With-MAC.exe --ip 192.168.1.100 --verbose
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
Find-Meraki-Ports-With-MAC version 1.2.0
  Commit:     a1b2c3d
  Build Time: 2024-01-15T10:30:00Z
  Go Version: go1.21
  Repository: https://github.com/bci/Find-Meraki-Ports-With-MAC
```

## Interactive Web Interface

Launch a user-friendly web interface for device resolution with real-time features:

```
Find-Meraki-Ports-With-MAC.exe --interactive
```

The web interface provides:

- **API Key Management**: Secure input and validation of Meraki API keys
- **Organization & Network Selection**: Dropdown menus for easy navigation
- **Device Resolution**: Search by MAC address or IP address with instant results
- **Network Topology**: Interactive visualization of switch connections (D3.js)
- **Real-time Logs**: Live logging with WebSocket streaming
- **Manufacturer Lookup**: IEEE OUI database integration for device identification
- **Alert Monitoring**: Real-time notifications for network events

### Web Interface Features

- **Responsive Design**: Works on desktop and mobile devices
- **Persistent Sessions**: Remembers your selections between visits
- **RESTful API**: Backend provides JSON APIs for integration
- **WebSocket Support**: Real-time updates for logs and alerts
- **Topology Visualization**: Interactive network maps with D3.js

### Web Server Configuration

```
Find-Meraki-Ports-With-MAC.exe --interactive --web-port 8080 --web-host localhost
```

**Web Server Flags:**
- --web-port: Port for web server (default: 8080)
- --web-host: Host for web server (default: localhost)

**Environment Variables:**
- WEB_PORT: Default web server port
- WEB_HOST: Default web server host

The web interface is available at `http://localhost:8080` (or configured host/port).

### Sample Output

Running with verbose logging shows the search process across networks and switches:

```
$ Find-Meraki-Ports-With-MAC.exe --ip 192.168.1.100 --org "My Organization" --verbose
2026-02-19T12:20:00-08:00 [DEBUG] Resolving IP: 192.168.1.100
2026-02-19T12:20:05-08:00 [DEBUG] Resolved IP 192.168.1.100 to MAC 40:a6:b7:a5:3b:e0 (hostname: ITCONSOLE.ci.gardena.ca.us)
2026-02-19T12:20:05-08:00 [DEBUG] Organization: My Organization
2026-02-19T12:20:05-08:00 [DEBUG] Network: Network1
2026-02-19T12:20:10-08:00 [DEBUG] Network clients API returned 1000 clients
2026-02-19T12:20:10-08:00 [DEBUG] Querying switch: switch1 (XXXX-XXXX-XXXX)
2026-02-19T12:20:45-08:00 [DEBUG] Device clients API returned 0 clients for switch1
2026-02-19T12:20:45-08:00 [DEBUG] Querying switch: switch2 (XXXX-XXXX-XXXX)
2026-02-19T12:20:48-08:00 [DEBUG] Live MAC table returned 147 entries for switch2
...
Org,Network,Switch,Serial,Port,MAC,IP,Hostname,LastSeen
My Organization,Network1,switch6,XXXX-XXXX-XXXX,51,40:a6:b7:a5:3b:e0,192.168.1.100,ITCONSOLE.ci.gardena.ca.us,2026-02-19T15:24:38Z
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
- WEB_PORT: default web server port (default: 8080)
- WEB_HOST: default web server host (default: localhost)

## Flags

**Required (one of):**
- --mac: MAC address or wildcard pattern
- --ip: IP address to resolve to MAC (mutually exclusive with --mac)

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
- --verbose: send DEBUG logs to console (overrides --log-level and --log-file)

**Logging:**
- --log-file: log file path (default from .env)
- --log-level: DEBUG | INFO | WARNING | ERROR

**Information:**
- --version: show version, commit, build time, and repository URL
- --help: show usage, flags, env vars, and examples

**Interactive Web Interface:**
- --interactive: launch web interface mode
- --web-port: port for web server (default: 8080)
- --web-host: host for web server (default: localhost)

## Output formats

All output formats include the following columns:
- Org: Organization name
- Network: Network name  
- Switch: Switch name
- Serial: Switch serial number
- Port: Port name/number
- MAC: MAC address
- IP: IP address (when resolved from --ip flag)
- Hostname: Resolved hostname (when available)
- Last Seen: Last seen timestamp

- csv (default)
- text
- html

## Notes

- This tool uses the Meraki Dashboard API and enumerates switch devices in the selected network(s).
- Client MAC visibility depends on the Meraki API data available for the switches.
- IP resolution uses the Meraki clients API to find IP-to-MAC mappings from recent network activity.
- Hostname resolution performs reverse DNS lookups and may not be available for all IPs.
- The --ip and --mac flags are mutually exclusive - use one or the other.

## Installation

### From source

```bash
go install github.com/bci/Find-Meraki-Ports-With-MAC@latest
```

### Build from source

```bash
git clone https://github.com/bci/Find-Meraki-Ports-With-MAC.git
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

https://github.com/bci/Find-Meraki-Ports-With-MAC
