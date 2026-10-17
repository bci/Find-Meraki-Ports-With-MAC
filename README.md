# Find-Meraki-Ports-With-MAC

[![Tests](https://github.com/bci/Find-Meraki-Ports-With-MAC/workflows/Tests/badge.svg)](https://github.com/bci/Find-Meraki-Ports-With-MAC/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/bci/Find-Meraki-Ports-With-MAC)](https://goreportcard.com/report/github.com/bci/Find-Meraki-Ports-With-MAC)
[![codecov](https://codecov.io/gh/bci/Find-Meraki-Ports-With-MAC/branch/main/graph/badge.svg)](https://codecov.io/gh/bci/Find-Meraki-Ports-With-MAC)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
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
- `GET /devices/{serial}/switch/ports/statuses` - Determine uplink ports (matches what Meraki Dashboard shows)
- `GET /networks/{networkId}/switch/linkAggregations` - Map `AGGR/n` link-aggregation ports to their member ports
- `GET /devices/{serial}/switch/ports` and `GET /devices/{serial}/switch/ports/{portId}` - Port mode (access/trunk), VLAN and AGGR membership fallback
- `POST /devices/{serial}/liveTools/arpTable` and `GET /devices/{serial}/liveTools/arpTable/{arpTableId}` - Fill in a missing IP for a found MAC
- `GET /networks/{networkId}/topology/linkLayer` - Topology view in the web interface (falls back to the device list)

All calls are read-only apart from the two `POST .../liveTools/...` calls, which start a lookup job on the switch.

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

Launch with sanitised demo data (no API key required — useful for screenshots and demos):

```
Find-Meraki-Ports-With-MAC.exe --interactive --test-data
```

![Find Meraki Ports — Interactive Web UI](docs/findmac-interactive.png)

The web interface provides:

- **API Key Management**: Input and validation of Meraki API keys (or the key from your `.env`, used automatically)
- **Organization & Network Selection**: Dropdown menus for easy navigation, including "All Networks"
- **Device Resolution**: Search by MAC address or IP address; results can be sorted and exported as CSV or JSON
- **Network Topology**: Interactive visualization of switch connections (D3.js, loaded from d3js.org)
- **Real-time Logs**: Live logging with WebSocket streaming
- **Manufacturer Lookup**: Vendor name from the MAC's OUI, looked up online at `api.macvendors.com` (only the first three octets are sent)

`/api/alerts` and `/api/logs` exist but are stubs that return empty lists; there is no alert monitoring yet.

**Run it from the repository root.** The page's JavaScript and CSS are served from `./static/`
relative to the current working directory, so a binary started elsewhere (for example one
installed with `go install`) serves a page with no styling or behaviour.

**Treat it as a local, single-user tool.** The server has no authentication: `/api/config`
returns the API key from your `.env`, and the topology page embeds it. Keep the default
`--web-host localhost` unless you understand that anyone who can reach the port can use
your key.

### Web Interface Features

- **Responsive Design**: Works on desktop and mobile devices
- **Persistent Sessions**: Remembers your org, network and log-level selections between visits (browser localStorage)
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
Org,Network,Switch,Serial,Port,AggrPorts,MAC,IP,Hostname,LastSeen,Uplink
My Organization,Network1,switch6,XXXX-XXXX-XXXX,51,,40:a6:b7:a5:3b:e0,192.168.1.100,ITCONSOLE.ci.gardena.ca.us,2026-02-19T15:24:38Z,
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

## Configuration (.env file)

The app reads configuration from a `.env` file. By default it looks for
**`~/.env.find-mac`** in your home directory — this file is created automatically
with commented-out stubs the first time you run the app.

| Platform | Default location |
|----------|------------------|
| macOS / Linux | `~/.env.find-mac` (`/home/<user>/.env.find-mac`) |
| Windows | `%USERPROFILE%\.env.find-mac` (e.g. `C:\Users\kent\.env.find-mac`) |

You can override the path with `--env <filepath>`:

```bash
# Use a shared config on a network drive
findmac --env /Volumes/share/meraki.env --mac 00:11:22:33:44:55

# Use a project-local .env
findmac --env .env --mac 00:11:22:33:44:55
```

### Variables

- `MERAKI_API_KEY` — **required** — Meraki Dashboard API key
- `MERAKI_ORG` — default org name (used if `--org` is not provided)
- `MERAKI_NETWORK` — default network name or `ALL`
- `OUTPUT_FORMAT` — `csv` | `text` | `html`
- `MERAKI_BASE_URL` — optional (defaults to `https://api.meraki.com/api/v1`)
- `MERAKI_RETRIES` — max API retry attempts on rate limit (default `6`)
- `MERAKI_MAC_POLL` — MAC table poll attempts, 2 s each (default `15`)
- `DNS_SERVERS` — comma-separated DNS servers for PTR lookups
- `LOG_FILE` — log file path (default `Find-Meraki-Ports-With-MAC.log`)
- `LOG_LEVEL` — `DEBUG` | `INFO` | `WARNING` | `ERROR`
- `WEB_PORT` — web server port (default `8080`)
- `WEB_HOST` — web server host (default `localhost`)
- `HOST_OVERRIDES` — JSON array of static IP→hostname mappings

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

**Tuning:**
- --retry: max API retry attempts on HTTP 429 rate limiting (default 6, or `MERAKI_RETRIES`)
- --mac-table-poll: live MAC/ARP table poll attempts, 2 s apart (default 15, or `MERAKI_MAC_POLL`)
- --dns-servers: comma-separated DNS servers for reverse (PTR) hostname lookups (or `DNS_SERVERS`)

**Troubleshooting & Testing:**
- --list-orgs: list organizations the API key can access
- --list-networks: list networks per organization
- --test-api: validate the API key
- --test-full-table: display all MACs in forwarding table (filters apply)
- --verbose: send DEBUG logs to console (overrides --log-level and --log-file)

**Logging:**
- --log-file: log file path (default from .env)
- --log-level: DEBUG | INFO | WARNING | ERROR

**Configuration:**
- --env: path to `.env` config file (default: `~/.env.find-mac`; created automatically if absent)

**Information:**
- --version: show version, commit, build time, and repository URL
- --help: show usage, flags, env vars, and examples

**Interactive Web Interface:**
- --interactive: launch web interface mode
- --test-data: launch web interface with sanitised demo data (no API key required)
- --web-port: port for web server (default: 8080)
- --web-host: host for web server (default: localhost)

## Output formats

All output formats include the following columns:
- Org: Organization name
- Network: Network name  
- Switch: Switch name
- Serial: Switch serial number
- Port: Port name/number (`AGGR/n` for a link-aggregation port)
- AggrPorts: Member ports when Port is an `AGGR/n` port
- MAC: MAC address
- IP: IP address (from the --ip lookup, the Meraki clients list, or the switch's ARP table)
- Hostname: Resolved hostname (when available)
- Last Seen: Last seen timestamp
- Uplink: `yes` when the switch reports the port as an uplink — the device is further upstream, not on this port

- csv (default)
- text
- html

## Notes

- This tool uses the Meraki Dashboard API and enumerates switch devices in the selected network(s).
- Client MAC visibility depends on the Meraki API data available for the switches.
- IP resolution uses the Meraki clients API to find IP-to-MAC mappings from recent network activity.
- Hostname resolution performs reverse DNS lookups and may not be available for all IPs.
- The --ip and --mac flags are mutually exclusive - use one or the other.
- `--port` matches substrings, so `--port 3` also matches ports 13, 23, 33, ...
- With `--verbose`, a `DEBUG:` line is printed to stdout before the results, so redirect carefully when capturing CSV.

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

### Test and lint

```bash
go build ./...
go vet ./...
go test -race ./...        # what CI runs (plus coverage)
make test                  # go test -v ./...
make lint                  # go vet + golangci-lint
```

The tests make no Meraki API calls. `TestResolveHostname` performs real reverse-DNS
lookups but tolerates failures. `go.mod` declares Go 1.21; CI tests 1.21–1.23.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This program is free software: you can redistribute it and/or modify it under the terms of the
[GNU General Public License v3.0](LICENSE) as published by the Free Software Foundation.
See the [LICENSE](LICENSE) file for details.

## Authors

- Kent Behrends
- GitHub Copilot Agents

## Repository

https://github.com/bci/Find-Meraki-Ports-With-MAC
