# Prompt: Find-Meraki-Ports-With-MAC (Meraki)

## Goal
Create a cross-platform Go CLI application named `Find-Meraki-Ports-With-MAC` that queries the Meraki Dashboard API to locate which switch and port a MAC address is associated with. The application must support both native Meraki MS switches and Cisco Catalyst switches managed by Meraki, handle multiple MAC input formats with wildcard patterns, read configuration from `.env` files, and output results in multiple formats.

## Technical Requirements

### Language & Dependencies
- **Go Version**: 1.21 or higher
- **Dependencies**:
  - `github.com/joho/godotenv` v1.5.1 - for `.env` file loading
- **Standard Library**: Use Go standard library for HTTP, JSON, CSV, HTML, logging, and regex

### Supported Switch Families
- **Meraki MS Series**: MS120, MS125, MS210, MS220, MS225, MS250, MS350, MS355, MS390, MS410, MS425, MS450
- **Cisco Catalyst (Meraki-managed)**: C9300 series (requires live MAC table lookup)

## CLI Interface

### Binary Name
- Windows: `Find-Meraki-Ports-With-MAC.exe`
- macOS/Linux: `Find-Meraki-Ports-With-MAC`

### Command-Line Flags

**Required (unless using list/test flags):**
- `--mac <address>` - MAC address or wildcard pattern to search for

**Optional Configuration:**
- `--output-format <format>` - Output format: `csv`, `text`, or `html` (default from .env)

**Filtering:**
- `--org <name>` - Filter by organization name (default from .env)
- `--network <name>` - Filter by network name or `ALL` (default from .env)
- `--switch <name>` - Filter by switch name (case-insensitive substring match)
- `--port <port>` - Filter by port name or number

**Troubleshooting:**
- `--list-orgs` - List all accessible organizations and exit
- `--list-networks` - List all networks for the organization and exit
- `--test-api` - Validate API key and connectivity
- `--test-full-table` - Display complete MAC forwarding table (respects --switch and --port filters)
- `--verbose` - Show detailed search progress (organizations, networks, switches)

**Logging:**
- `--log-file <path>` - Log file path (default from .env)
- `--log-level <level>` - Logging level: `DEBUG`, `INFO`, `WARNING`, `ERROR` (default from .env)

**Information:**
- `--version` - Display version, commit, build time, and repository URL, then exit
- `--help` - Display comprehensive help with all flags, environment variables, and usage examples

### Usage Examples

```bash
# Basic MAC lookup
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network "City" --org "My Org"

# Search all networks
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL

# Filter by switch and port
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network "City" --switch ccc9300xa --port 3

# Wildcard search
Find-Meraki-Ports-With-MAC.exe --mac "08:f1:b3:6f:9c:*" --network ALL

# Dump complete MAC table from specific switch
Find-Meraki-Ports-With-MAC.exe --test-full-table --network "City" --switch ccc9300xa

# HTML output format
Find-Meraki-Ports-With-MAC.exe --mac 00:11:22:33:44:55 --network ALL --output-format html

# Troubleshooting
Find-Meraki-Ports-With-MAC.exe --list-orgs
Find-Meraki-Ports-With-MAC.exe --list-networks --org "My Org"
Find-Meraki-Ports-With-MAC.exe --test-api
```

## Environment Configuration (.env)

### Environment Variables
- `MERAKI_API_KEY` (required) - Meraki Dashboard API key
- `MERAKI_ORG` (optional) - Default organization name
- `MERAKI_NETWORK` (optional) - Default network name or `ALL`
- `OUTPUT_FORMAT` (optional) - Default output format: `csv`, `text`, or `html`
- `MERAKI_BASE_URL` (optional) - API base URL (default: `https://api.meraki.com/api/v1`)
- `LOG_FILE` (optional) - Log file path (default: `Find-Meraki-Ports-With-MAC.log`)
- `LOG_LEVEL` (optional) - Default log level: `DEBUG`, `INFO`, `WARNING`, `ERROR`

### .env.example Template
Provide a `.env.example` file with sample values and comments explaining each variable.

## MAC Address Handling

### Supported Input Formats
- Colon-separated: `00:11:22:33:44:55`
- No separators: `08f1b36f9c25`
- Dot-separated: `08.f1.b3.6f.9c.25`
- Mixed dot notation: `08f1.b36f.9c25`

### Wildcard Patterns
- `*` - Matches exactly one byte (two hex digits)
   - Example: `00:11:22:33:44:*` matches `00:11:22:33:44:00` through `00:11:22:33:44:ff`
- `[ranges]` - Bracket patterns for hex nibbles
  - Example: `08:f1:b3:6f:9c:[1-4][0-f]` matches specific ranges per nibble

### Normalization
- Remove separators (colons, dots) for pattern matching
- Preserve brackets for pattern parsing
- Output MACs in colon-separated format: `00:11:22:33:44:55`

## Meraki Dashboard API Integration

### Base URL
`https://api.meraki.com/api/v1`

### Required API Endpoints

1. **List Organizations**
   - `GET /organizations`
   - Returns all organizations accessible by the API key

2. **List Networks**
   - `GET /organizations/{organizationId}/networks`
   - Returns all networks for a specific organization

3. **List Devices**
   - `GET /networks/{networkId}/devices`
   - Returns all devices in a network

4. **Network-Level Clients**
   - `GET /networks/{networkId}/clients?perPage=1000&timespan=2592000`
   - Returns clients seen across the entire network (30-day timespan)
   - Primary method for MAC lookup

5. **Device-Level Clients** (Fallback)
   - `GET /devices/{serial}/clients?perPage=1000&timespan=2592000`
   - Returns clients seen on a specific device
   - Use as fallback if network-level lookup yields no results

6. **Create Live MAC Table Lookup** (CRITICAL for Catalyst switches)
   - `POST /devices/{serial}/liveTools/macTable`
   - Initiates real-time MAC table dump
   - Returns `macTableId` for polling
   - Required because Catalyst switches may not appear in standard client endpoints

7. **Get Live MAC Table Results**
   - `GET /devices/{serial}/liveTools/macTable/{macTableId}`
   - Polls for MAC table lookup status and results
   - Status values: `pending`, `complete`, `failed`
   - Returns array of MAC entries when complete

### Live MAC Table Lookup Implementation (CRITICAL)

**Why it's needed:**
Cisco Catalyst switches managed by Meraki have limited visibility through standard client API endpoints. The live tools MAC table endpoint provides real-time access to the switch's forwarding database.

**Implementation:**
1. POST to `/devices/{serial}/liveTools/macTable` to create a lookup job
2. Extract `macTableId` from the response
3. Poll GET `/devices/{serial}/liveTools/macTable/{macTableId}` up to 15 times
4. Wait 2 seconds between poll attempts (30-second total timeout)
5. Check response `status` field:
   - `pending` - Continue polling
   - `complete` - Parse entries from response
   - `failed` - Log error and skip switch
6. Parse MAC entries from the `entries` array
7. Extract port from `interface` field (NOT `portId` or `port`)

**Live MAC Entry Structure:**
```json
{
   "mac": "00:11:22:33:44:55",
  "interface": "3",
  "vlan": "100"
}
```

**Fallback Strategy:**
If live lookup fails, attempt device-level client API as fallback.

### Switch Detection
Filter devices where:
- `productType == "switch"` OR
- `model` starts with `MS` (Meraki switches) OR
- `model` starts with `C9` (Catalyst 9000 series)

### Pagination
- Meraki uses `Link` header with `rel="next"` for pagination
- Parse `Link` header to get next page URL
- Aggregate results from all pages
- Use `perPage=1000` to minimize requests

### Rate Limiting & Retry
- Handle HTTP 429 (Too Many Requests)
- Check for `Retry-After` header (in seconds)
- Implement exponential backoff if `Retry-After` not provided
- Maximum 3 retry attempts per request

### Error Handling
- Handle HTTP 404 (resource not found) gracefully
- Handle HTTP 400 (bad request) with clear error messages
- Handle network timeouts with retries
- Log all API errors with full context (network, switch, endpoint)

## MAC Lookup Strategy

### Dual-Strategy Approach
1. **Primary**: Query network-level clients API
   - Faster, covers all devices in network
   - Works well for Meraki MS switches
   
2. **Secondary**: Live MAC table lookup per switch
   - Essential for Cisco Catalyst switches
   - Provides real-time forwarding table data
   - Polls with 2-second intervals (max 15 attempts, 30s timeout)
   
3. **Fallback**: Device-level clients API
   - Used if live lookup fails
   - Less reliable for Catalyst but provides some data

### Search Flow
1. Load configuration from .env and flags
2. Validate API key with test request
3. Get organization by name (or use only available org)
4. Get network(s) by name or ALL
5. For each network:
   - Get network-level clients and match MACs
   - Get all switch devices
   - Filter switches by productType/model and optional --switch filter
   - For each switch:
     - Attempt live MAC table lookup (POST + polling GET)
     - Parse entries and match MACs
     - If live lookup fails, try device clients API
     - Extract port from `interface` > `portId` > `port` (in priority order)
6. Apply --port filter if specified
7. Sort results by network, switch, port
8. Output in requested format

## Output

### Result Columns
- Organization Name
- Network Name
- Switch Name
- Switch Serial
- Port (prefer: `interface` > `portId` > `switchportName` > `switchport` > `port`)
- MAC Address (colon-separated format)
- Last Seen (timestamp)

### Output Formats

**CSV (default):**
```csv
Org,Network,Switch,Serial,Port,MAC,LastSeen
My Org,City,ccc9300xa,QXXX-XXXX-XXXX,3,00:11:22:33:44:55,2026-02-13T10:30:00Z
```

**Text:**
```
Organization: My Org
Network: City
Switch: ccc9300xa
Serial: QXXX-XXXX-XXXX
Port: 3
MAC: 00:11:22:33:44:55
Last Seen: 2026-02-13T10:30:00Z
```

**HTML:**
```html
<table>
  <thead>
    <tr><th>Org</th><th>Network</th><th>Switch</th><th>Serial</th><th>Port</th><th>MAC</th><th>Last Seen</th></tr>
  </thead>
  <tbody>
   <tr><td>My Org</td><td>City</td><td>ccc9300xa</td><td>QXXX-XXXX-XXXX</td><td>3</td><td>00:11:22:33:44:55</td><td>2026-02-13T10:30:00Z</td></tr>
  </tbody>
</table>
```

### Sorting
Sort results by:
1. Network name (ascending)
2. Switch name (ascending)
3. Port (ascending)

## Logging System

### Log Levels
- `DEBUG` - Detailed trace of operations (API requests, polling attempts, response parsing)
- `INFO` - General operational information (networks found, switches scanned, MACs matched)
- `WARNING` - Non-fatal issues (retries, timeouts, partial failures)
- `ERROR` - Errors requiring attention (API failures, authentication issues)

### Log Output
- Write to file specified by `--log-file` or `LOG_FILE` env var
- Default: `Find-Meraki-Ports-With-MAC.log`
- Include timestamp, level, and message
- For MAC table lookups, include: network name, switch serial, switch name, status, attempt number

### Example Log Entries
```
2026-02-13 10:30:15 DEBUG: Initiating live MAC table lookup for switch ccc9300xa (QXXX-XXXX-XXXX) in network City
2026-02-13 10:30:17 DEBUG: MAC table lookup status for ccc9300xa (QXXX-XXXX-XXXX) in network City: pending (attempt 1/15)
2026-02-13 10:30:19 DEBUG: MAC table lookup status for ccc9300xa (QXXX-XXXX-XXXX) in network City: complete (attempt 2/15)
2026-02-13 10:30:19 INFO: Retrieved 1097 MAC entries from switch ccc9300xa
2026-02-13 10:30:19 INFO: Found MAC 00:11:22:33:44:55 on switch ccc9300xa port 3
```

### Error Logging
Always include context:
- Network name and ID
- Switch name and serial
- Endpoint being called
- HTTP status code
- Response body (for non-200 responses)

## Project Structure

### Modular Architecture

The project uses a modular architecture with reusable packages:

```
Find-Meraki-Ports-With-MAC/
├── main.go                      # CLI entry point and orchestration
├── main_test.go                 # Tests for main.go helper functions
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── .env.example                 # Environment variable template
├── .env                         # User's environment (git-ignored)
├── .gitignore                   # Git ignore rules
├── README.md                    # User documentation
├── PROMPT.md                    # This specification
├── build.ps1                    # PowerShell build script (Windows)
├── build.sh                     # Bash build script (Unix)
├── Makefile                     # Make build targets
├── bin/                         # Build output directory (git-ignored)
├── .github/
│   ├── workflows/
│   │   └── test.yml             # CI/CD workflow for automated testing
│   └── copilot-instructions.md  # GitHub Copilot workspace instructions
└── pkg/                         # Reusable packages
    ├── macaddr/                 # MAC address utilities
    │   ├── macaddr.go           # Normalization, formatting, pattern matching
    │   └── macaddr_test.go      # Unit tests for MAC utilities
    ├── meraki/                  # Meraki Dashboard API client
    │   └── client.go            # API methods with pagination and retry logic
    ├── logger/                  # Structured logging
    │   └── logger.go            # Level-based logging with file output
    ├── output/                  # Output formatters
    │   ├── writers.go           # CSV, Text, HTML writers
    │   └── writers_test.go      # Tests for output formats
    └── filters/                 # Device and port filtering
        ├── filters.go           # Switch detection and filtering
        └── filters_test.go      # Tests for filtering logic
```

### Package Responsibilities

**pkg/macaddr**: MAC address utilities for any network tool
- `NormalizeExactMac`: Accept multiple formats (colon, dot, no separators)
- `FormatMacColon`: Output formatting with colon separators
- `BuildMacMatcher`: Pattern matching with wildcards and bracket patterns
- `BuildMacRegex`: Convert patterns to regular expressions
- Comprehensive test coverage for all input formats and patterns

**pkg/meraki**: Meraki Dashboard API v1 client
- `NewClient`: Initialize client with API key and base URL
- `GetOrganizations`, `GetNetworks`, `GetDevices`: Query infrastructure
- `GetNetworkClients`, `GetDeviceClients`: Query MAC data
- `CreateMacTableLookup`, `GetMacTableLookup`: Live MAC table for Catalyst switches
- Automatic pagination via Link headers
- Retry logic with exponential backoff for rate limiting (HTTP 429)
- 60-second timeout on requests

**pkg/logger**: Simple structured logging
- `ParseLogLevel`: Convert string to log level enum
- `New`: Create logger with file and stderr output
- `Debugf`, `Infof`, `Warnf`, `Errorf`: Level-based logging methods
- RFC3339 timestamps

**pkg/output**: Multiple output format support
- `ResultRow`: Struct representing one result row
- `WriteCSV`: CSV output with headers
- `WriteText`: Plain text table with aligned columns
- `WriteHTML`: HTML table with proper escaping
- Comprehensive tests for all formats

**pkg/filters**: Device and port filtering utilities
- `FilterSwitches`: Filter devices by productType and model (MS, C9)
- `FilterSwitchesByName`: Case-insensitive substring filtering
- `MatchesSwitchFilter`: Switch name matching logic
- `MatchesPortFilter`: Port filtering logic
- Uses pkg/meraki types

### main.go Structure

The main.go file has been refactored to ~530 lines using the modular packages:

```go
package main

// Imports: Standard library + pkg/* + godotenv

// Config struct - all configuration from flags and .env
type Config struct {
    APIKey       string
    OrgName      string
    NetworkName  string
    OutputFormat string
    LogFile      string
    LogLevel     string
    Verbose      bool
    SwitchFilter string
    PortFilter   string
    TestFull     bool
}

// Main flow:
// 1. Load .env
// 2. Parse flags
// 3. Initialize logger using pkg/logger
// 4. Build MAC matcher using pkg/macaddr
// 5. Create Meraki client using pkg/meraki
// 6. Handle --list-orgs, --list-networks, --test-api flags
// 7. Get organization and networks
// 8. For each network:
//    - Get network clients (primary lookup)
//    - Get devices, filter switches using pkg/filters
//    - For each switch:
//      - Try live MAC table lookup via pkg/meraki
//      - Parse entries from "interface" field
//      - If failed, fallback to device clients
// 9. Filter by --switch, --port using pkg/filters
// 10. Sort results
// 11. Output using pkg/output

// Helper functions:
// - firstNonEmpty: Returns first non-empty string
// - exitWithError: Logs error and exits
// - selectOrganization: Finds org by name
// - selectNetworks: Filters networks by name or ALL
// - addResult: Deduplicates and adds results
// - printUsage: Displays help text
// - writeOrganizations: Formats org list
// - writeNetworksForOrg: Formats network list
```

## Documentation Standards

### Godoc Compliance

All .go files must follow godoc conventions:

**Package Documentation:**
- Every package must have a package comment describing its purpose
- Package comment appears before the package declaration
- Example: `// Package macaddr provides utilities for working with MAC addresses.`

**Exported Functions/Types:**
- All exported functions, types, constants, and variables must have godoc comments
- Comment starts with the name of the item being documented
- Example: `// NormalizeExactMac normalizes a MAC address to lowercase hex without separators.`

**Function Documentation:**
- Describe what the function does, not how
- Document parameters and return values when not obvious
- Document special behaviors, errors, and edge cases
- Use complete sentences

**Example Documentation:**
```go
// BuildMacMatcher creates a MAC matching function from an input pattern.
// Returns:
//   - matcher function that tests if a MAC matches the pattern
//   - normalized pattern string
//   - isPattern flag (true for wildcards, false for exact match)
//   - error if the pattern is invalid
//
// Supports:
//   - Exact MAC: "00:11:22:33:44:55"
//   - Wildcards: "08:f1:b3:6f:9c:*" where * matches one byte
//   - Bracket patterns: "08:f1:b3:6f:9c:[1-4][0-f]" for hex ranges
func BuildMacMatcher(input string) (func(string) bool, string, bool, error)
```

**Unexported Functions:**
- Document unexported functions when they perform non-trivial operations
- Helps maintainability and code understanding
- Example: `// isHexDigit checks if a byte is a valid hexadecimal digit (0-9, A-F, a-f).`

## Testing Requirements

### Test Coverage

All packages must have comprehensive unit tests:

**pkg/macaddr/macaddr_test.go:**
- TestNormalizeExactMac: 7 test cases for input formats and validation
- TestFormatMacColon: 2 test cases for formatting
- TestBuildMacRegex: Pattern matching with wildcards and brackets
- TestBuildMacMatcher: End-to-end pattern matching
- Benchmarks for performance-critical functions

**pkg/output/writers_test.go:**
- TestWriteCSV: Verify CSV format and headers
- TestWriteText: Verify text table formatting
- TestWriteHTML: Verify HTML output and escaping
- Test edge cases (empty results, special characters)

**pkg/filters/filters_test.go:**
- TestFilterSwitches: Device type filtering (MS, C9, productType)
- TestFilterSwitchesByName: Case-insensitive substring matching
- TestMatchesSwitchFilter: Switch name matching logic
- TestMatchesPortFilter: Port filtering logic

**main_test.go:**
- TestFirstNonEmpty: Multiple scenarios
- TestSelectOrganization: Exact match, not found, auto-select
- TestSelectNetworks: Single network, ALL, not found
- TestAddResult: Deduplication logic

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...

# Run specific package
go test ./pkg/macaddr

# Verbose output
go test -v ./...
```

### CI/CD Testing

GitHub Actions workflow (`.github/workflows/test.yml`) runs:
- **Test Matrix**: 3 operating systems × 3 Go versions
  - OS: Ubuntu, Windows, macOS
  - Go: 1.21, 1.22, 1.23
- **Test Job**: `go test -race -coverprofile=coverage.txt ./...`
- **Lint Job**: `golangci-lint run`
- **Build Job**: Cross-platform builds for all 6 targets
- **Coverage**: Upload to Codecov for tracking

### go.mod

```
module Find-Meraki-Ports-With-MAC

go 1.21

require github.com/joho/godotenv v1.5.1
```

## Build & Distribution

### Multi-Platform Build Scripts

All build scripts create binaries for 6 platforms in the `bin/` directory:

**Supported Platforms:**
- Windows: amd64, arm64
- macOS (Darwin): amd64, arm64
- Linux: amd64, arm64

**PowerShell (build.ps1) - Windows:**
```powershell
# Builds for all 6 platforms
# Usage: .\build.ps1
# Output: bin/Find-Meraki-Ports-With-MAC-{os}-{arch}{.exe}

# Sets GOOS and GOARCH environment variables for each build
# Disables CGO for cross-platform builds
```

**Bash (build.sh) - macOS/Linux:**
```bash
# Builds for all 6 platforms
# Usage: chmod +x build.sh && ./build.sh
# Output: bin/Find-Meraki-Ports-With-MAC-{os}-{arch}

# Sets GOOS and GOARCH environment variables for each build
# Disables CGO for cross-platform builds
```

**Makefile (build):**
```makefile
# Targets:
#   make build-all        # Build all 6 platforms
#   make build-windows    # Build Windows (amd64, arm64)
#   make build-darwin     # Build macOS (amd64, arm64)
#   make build-linux      # Build Linux (amd64, arm64)
#   make clean            # Remove bin/ directory

# Usage: make build-all
```

### Build Artifacts

All build scripts produce:
```
bin/
├── Find-Meraki-Ports-With-MAC-windows-amd64.exe
├── Find-Meraki-Ports-With-MAC-windows-arm64.exe
├── Find-Meraki-Ports-With-MAC-darwin-amd64
├── Find-Meraki-Ports-With-MAC-darwin-arm64
├── Find-Meraki-Ports-With-MAC-linux-amd64
└── Find-Meraki-Ports-With-MAC-linux-arm64
```

### Cross-Platform Considerations

- **Windows**: `.exe` extension added automatically
- **macOS/Linux**: No extension, marked executable
- **CGO disabled**: `CGO_ENABLED=0` for static binaries
- **GOOS/GOARCH**: Environment variables control target platform
- **No external dependencies**: Standard library + godotenv (pure Go)

### Single Platform Build
```bash
# Windows
go build -o Find-Meraki-Ports-With-MAC.exe .

# macOS/Linux
go build -o Find-Meraki-Ports-With-MAC .
```

## Environment Configuration & Security

### .env File Security

**CRITICAL: Never commit .env to version control**

The `.env` file contains the Meraki API key and must be protected:

```bash
# .gitignore includes .env to prevent accidental commits
# Only .env.example should be in version control
```

### API Key Security Best Practices

**For Development:**
1. Create `.env` file from `.env.example`:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` with your actual Meraki API key (ONLY locally):
   ```
   MERAKI_API_KEY=your_actual_api_key_here
   ```

3. Never share `.env` file
4. Add `.env` to `.gitignore` (already done)

**For CI/CD (GitHub Actions):**
1. Store API key as GitHub Secret: `Settings > Secrets > Actions`
2. Reference in workflow:
   ```yaml
   env:
     MERAKI_API_KEY: ${{ secrets.MERAKI_API_KEY }}
   ```

3. Never print or log the API key
4. Secrets are masked in logs automatically

**For Production Deployment:**
1. Use environment variables (not .env files)
2. Set via deployment platform (Docker, Kubernetes, systemd, etc.)
3. Use temporary credentials with limited scope when possible
4. Rotate API keys regularly
5. Monitor API key usage for unusual activity

### Environment Variables

```bash
# MERAKI_API_KEY (required)
# The Meraki Dashboard API key for authentication
# Source: Meraki Dashboard > Settings > API > API Keys
MERAKI_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxx

# MERAKI_ORG (optional)
# Default organization name to avoid --org flag every time
MERAKI_ORG=My Org

# MERAKI_NETWORK (optional)
# Default network name or "ALL" to avoid --network flag
MERAKI_NETWORK=ALL

# OUTPUT_FORMAT (optional)
# Default output format: csv, text, or html
OUTPUT_FORMAT=csv

# MERAKI_BASE_URL (optional)
# Meraki API base URL (almost never needs to change)
MERAKI_BASE_URL=https://api.meraki.com/api/v1

# LOG_FILE (optional)
# Path to log file for verbose output
LOG_FILE=Find-Meraki-Ports-With-MAC.log

# LOG_LEVEL (optional)
# Logging verbosity: DEBUG, INFO, WARNING, ERROR
LOG_LEVEL=DEBUG
```

### .env.example Template

```bash
# Meraki Dashboard API Configuration
# Get your API key from: https://dashboard.meraki.com/api/v1/docs
MERAKI_API_KEY=your_api_key_here

# Optional: Default organization name
MERAKI_ORG=

# Optional: Default network name or "ALL" for all networks
MERAKI_NETWORK=ALL

# Optional: Default output format (csv, text, html)
OUTPUT_FORMAT=csv

# Optional: Meraki API base URL (usually not needed)
MERAKI_BASE_URL=https://api.meraki.com/api/v1

# Optional: Log file path
LOG_FILE=Find-Meraki-Ports-With-MAC.log

# Optional: Log level (DEBUG, INFO, WARNING, ERROR)
LOG_LEVEL=DEBUG
```

## .gitignore

```
# Environment files
.env

# Binaries
Find-Meraki-Ports-With-MAC
Find-Meraki-Ports-With-MAC.exe
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/

# Test binaries
*.test
*.out
*.prof

# Dependencies
vendor/

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Log files
*.log
Find-Meraki-Ports-With-MAC.log
```

## Testing & Validation

### Manual Testing Checklist
1. **API Connectivity**:
   - `--test-api` validates API key
   - `--list-orgs` returns organizations
   - `--list-networks` returns networks

2. **MAC Lookup**:
   - Test exact MAC match on Meraki MS switch
   - Test exact MAC match on Catalyst C9300 switch
   - Test wildcard patterns
   - Test --network ALL vs specific network

3. **Live MAC Table Lookup**:
   - Verify POST creates job successfully
   - Verify polling completes within 30 seconds
   - Verify entries parsed from "interface" field
   - Verify debug logs show network/switch/status context

4. **Filtering**:
   - Test --switch filter (case-insensitive substring)
   - Test --port filter (exact match)
   - Test --test-full-table with and without filters

5. **Output Formats**:
   - CSV format (default)
   - Text format
   - HTML format

6. **Logging**:
   - Verify log file creation
   - Verify DEBUG level shows API details
   - Verify INFO level shows progress
   - Verify ERROR level captures failures

### Known Limitations
- Cisco Catalyst switches may show limited data in standard client endpoints
- Live MAC table lookup requires API permissions for live tools
- 30-day timespan limit on client data
- Pagination required for networks/switches with >1000 entries

## Documentation (README.md)

### Required Sections
1. **Title & Badges**: Go Report Card, License (Unlicense), GitHub Release
2. **Description**: Brief overview of functionality
3. **Switch Families Tested**: List of MS and C9 models
4. **APIs Used**: All 7 endpoints with descriptions
5. **Installation**: `go install`, build from source
6. **Usage**: Examples with all major flags
7. **MAC Formats**: Supported formats and wildcards
8. **Environment Variables**: Table of .env options
9. **Flags**: Complete flag reference
10. **Output Formats**: CSV, text, HTML examples
11. **Build Scripts**: PowerShell, Bash, Make instructions
12. **Contributing**: Contribution guidelines
13. **License**: Unlicense (public domain)
14. **Authors**: Kent Behrends and GitHub Copilot Agents
15. **Repository**: GitHub repository URL

## Key Implementation Details

### Critical Discoveries
1. **Catalyst Switch Support**: Standard client endpoints don't work reliably for C9300 switches. Must use live tools MAC table endpoint.
2. **Live Lookup Polling**: Poll every 2 seconds for up to 15 attempts (30s timeout) waiting for `status: "complete"`.
3. **Port Field Mapping**: Live lookup returns port in `interface` field, not `portId`. Priority: `interface` > `portId` > `switchportName` > `switchport` > `port`.
4. **Network-Level Clients**: Always query network-level clients first as it's faster and covers more devices in a single request.

### Performance Optimizations
- Use network-level client API before per-switch lookups
- Implement pagination to handle large datasets
- Use Go concurrency for parallel switch queries (optional enhancement)
- Cache organization/network lookups to avoid redundant API calls

### Error Resilience
- Never panic; always return errors
- Log all failures with full context
- Provide fallback strategies (live lookup → device clients)
- Handle partial failures gracefully (continue to next switch if one fails)

## License & Attribution

**License**: The Unlicense (public domain)

**Authors**:
- Kent Behrends
- GitHub Copilot Agents

**Repository**: https://github.com/BEHRConsulting/find-meraki-switch-for-mac

## Success Criteria

The application is complete when:
1. ✅ All flags work as specified
2. ✅ MAC address found on both Meraki MS and Catalyst C9300 switches
3. ✅ Live MAC table lookup successfully retrieves 1000+ entries from Catalyst switches
4. ✅ Wildcard patterns match correctly
5. ✅ All three output formats (CSV, text, HTML) render properly
6. ✅ Logging captures all operations with appropriate levels
7. ✅ Build scripts generate binaries for all 6 platforms
8. ✅ README.md is comprehensive and accurate
9. ✅ .env.example provides clear guidance
10. ✅ --help output is clear and complete
