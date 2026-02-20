# Interactive Web Interface Mode for Find-Meraki-Ports-With-MAC

## Overview
Add an interactive web interface mode to the Find-Meraki-Ports-With-MAC CLI tool that provides a user-friendly web UI for Meraki network device discovery and troubleshooting.

## Command Line Interface
Add new command line flags:
- `-i, --interactive`: Launch web interface mode
- `--web-port`: Port for web server (default: 8080)
- `--web-host`: Host for web server (default: localhost)

## Web Interface Features

### 1. Initial API Key Input Page
- **Password Field**: Secure input for Meraki API key
- **Read API Key Button**: Validates API key and loads organization/network data
- **Error Display**: Shows validation errors or connection issues
- **Loading States**: Visual feedback during API calls

### 2. Main Resolution Interface
After API key validation, display:

#### Organization Dropdown
- **Source**: GET /organizations API call
- **Display**: Organization name
- **Default**: First organization or from .env file
- **Validation**: Required selection before proceeding

#### Network Dropdown
- **Source**: GET /organizations/{orgId}/networks API call
- **Display**: Network name
- **Default**: First network or from .env file
- **Filter**: Only show networks accessible to the API key
- **Validation**: Required selection before proceeding

#### Device Resolution Section
- **MAC Address Input**: Text field with validation
  - Accepts formats: `00:11:22:33:44:55`, `001122334455`, `00-11-22-33-44-55`, `0011122.334455`, `00.11.22.33.44.55`
  - Real-time format validation
  - Wildcard support: `00:11:22:*`, `00:11:22:33:44:*`
- **IP Address Input**: Text field for IP resolution
  - IPv4 format validation
  - Optional field (can resolve IP or MAC independently)
- **Resolve Button**: Triggers the resolution process
- **Topology Button**: Opens network topology visualization in new window
- **Clear Button**: Resets all inputs and results

#### Results Display
- **Table Format**: Shows switch, port, MAC, IP, hostname, last seen
- **CSV Export**: Download results as CSV file
- **JSON Export**: Download results as JSON
- **Copy to Clipboard**: Copy results in various formats
- **Real-time Updates**: Results update as they are found

#### Manufacturer Information Section
- **Automatic Lookup**: When MAC address is resolved, display manufacturer info
- **OUI Database**: Use IEEE OUI database to identify device manufacturer
- **Info Display**: Show manufacturer name, company info, and logo if available
- **Fallback**: Show "Unknown Manufacturer" for unrecognized OUIs
- **Caching**: Cache OUI lookups for performance
- **Source**: First 3 bytes (24 bits) of MAC address for OUI lookup

#### Log Display Section
- **Log Level Dropdown**: DEBUG, INFO, WARNING, ERROR
  - Dynamically changes logging verbosity
  - Persists across page refreshes
- **Log Box**: Real-time scrolling log display
  - Color-coded by log level
  - Timestamped entries
  - Auto-scroll to bottom
  - Maximum lines limit (configurable)
- **Clear Logs Button**: Clears the log display
- **Export Logs Button**: Download logs as text file

#### Network Topology Visualization
- **Topology Button**: Opens interactive network map in new browser window
- **Auto-Discovery**: Automatically maps all switches, access points, and cameras
- **Visual Layout**: Hierarchical display with core, distribution, and access layers
- **Interactive Elements**: 
  - Hover for device details (name, model, IP, status)
  - Click for full device information and port status
  - Color-coded ports (green=up, red=down, yellow=warning)
- **Real-time Updates**: Live status indicators and link utilization
- **Search & Filter**: Find devices by name, IP, MAC, or filter by type/model
- **Export Options**: Save topology as PNG/SVG for documentation
- **Troubleshooting Integration**: Highlight paths when resolving MAC addresses

#### Network Alerts & Notifications
- **Alerts Table**: Displays all network alerts from accessible organizations/networks
- **Real-time Updates**: Live alert feed with WebSocket updates
- **Alert Categories**: Filter by severity (critical, warning, info), device type, time range
- **Alert Details**: Click alerts for full description, affected devices, timestamps
- **Email Notifications**: Configure email alerts for critical issues
- **Webhook Integration**: Send alerts to external systems (Slack, Teams, monitoring tools)
- **Alert History**: Searchable alert history with export capabilities
- **Alert Acknowledgment**: Mark alerts as acknowledged/read

## Technical Implementation

### Backend (Go)
- **Web Framework**: Use `net/http` with `gorilla/mux` for routing
- **Templates**: HTML templates with Go template engine
- **WebSocket/SSE**: Real-time log updates and progress
- **OUI Database**: IEEE OUI lookup for manufacturer identification
- **Caching**: In-memory cache for OUI lookups
- **CORS**: Handle cross-origin requests if needed
- **Security**: API key never exposed in client-side code

### Frontend (HTML/CSS/JavaScript)
- **Vanilla JS**: No heavy frameworks for simplicity
- **Topology Visualization**: D3.js or similar for interactive network graphs
- **Responsive Design**: Works on desktop and mobile
- **Progressive Enhancement**: Works without JavaScript (basic form)
- **Accessibility**: ARIA labels, keyboard navigation

### API Endpoints
```
GET  /                           - Initial API key input page
POST /api/validate-key          - Validate API key and return orgs
GET  /api/networks?org={id}     - Get networks for organization
POST /api/resolve               - Resolve MAC/IP to switch port
GET  /api/manufacturer?mac={mac} - Get manufacturer info for MAC address
GET  /topology                  - Network topology visualization page
GET  /api/topology?org={id}&networks={ids} - Get topology data (devices, connections)
GET  /api/device-details?serial={serial} - Get detailed device information
GET  /api/alerts?org={id}&networks={ids} - Get network alerts
POST /api/alerts/acknowledge    - Acknowledge alerts
POST /api/notifications/setup   - Configure email/webhook notifications
GET  /api/logs                  - WebSocket/SSE for real-time logs
POST /api/log-level             - Change log level
GET  /api/export?format={csv|json} - Export results
```

### Data Flow
1. User enters API key → POST /api/validate-key
2. Backend validates key and fetches organizations
3. User selects org → GET /api/networks?org={id}
4. Backend fetches networks for selected org
5. User enters MAC/IP and clicks resolve → POST /api/resolve
6. Backend performs resolution, streams logs via WebSocket
7. Results displayed in table, manufacturer info fetched automatically
8. Manufacturer details shown in info section alongside results
9. User clicks Topology button → Opens /topology in new window
10. Topology page loads device/connection data via /api/topology
11. Alerts table loads and updates via /api/alerts with WebSocket

### Configuration
- **Environment Variables**:
  - `WEB_PORT`: Default web server port
  - `WEB_HOST`: Default web server host
  - `WEB_LOG_LEVEL`: Default log level for web interface
  - `ALERT_EMAIL`: Email address for alert notifications
  - `ALERT_WEBHOOK_URL`: Webhook URL for external notifications
  - `ALERT_SEVERITY_LEVEL`: Minimum severity level for notifications
- **Command Line Overrides**: All web settings can be overridden via CLI

### Security Considerations
- **API Key Handling**: Never store in browser, only in server session
- **HTTPS**: Recommend HTTPS for production use
- **Input Validation**: Sanitize all user inputs
- **Rate Limiting**: Prevent API abuse
- **Session Management**: Temporary sessions for API key storage

### Error Handling
- **Network Errors**: Graceful handling of API failures
- **Invalid API Key**: Clear error messages
- **No Results Found**: Helpful suggestions
- **Timeout Handling**: Progress indicators for long operations

### Testing
- **Unit Tests**: Web handlers, template rendering, input validation, OUI lookup, alert processing
- **Integration Tests**: Full resolution workflow with manufacturer lookup and alert monitoring
- **E2E Tests**: Browser automation for UI testing, notification delivery
- **Performance**: Handle multiple concurrent users and alert streams

### Documentation Updates
- **README.md**: Add interactive mode section with screenshots
- **CHANGELOG.md**: Document new feature in next version
- **Usage Examples**: Web interface usage alongside CLI examples

### Future Enhancements
- **Saved Queries**: Store and replay common searches
- **Bulk Resolution**: Upload CSV files for batch processing
- **Advanced Topology Features**: Custom layouts, historical views, comparison mode
- **Advanced Alert Features**: Custom alert rules, escalation policies, integration with ITSM tools
- **Multi-tenancy**: Support multiple API keys simultaneously

## Implementation Priority
1. **Core Web Server**: Basic HTTP server with templates
2. **API Key Validation**: Secure key input and org/network loading
3. **Basic Resolution**: MAC/IP input and results display
4. **Network Topology**: Interactive visualization in new window
5. **Network Alerts**: Alert monitoring table with email/webhook notifications
6. **Real-time Logs**: WebSocket/SSE for log streaming
7. **Export Features**: CSV/JSON download capabilities
8. **Polish**: Responsive design, error handling, accessibility

## Dependencies
- **Existing**: Reuse all current Meraki API client code
- **New**: `gorilla/mux` for routing, `gorilla/websocket` for real-time logs
- **Frontend**: D3.js for topology visualization
- **Notifications**: Email library (e.g., `gopkg.in/gomail.v2`) and HTTP client for webhooks
- **OUI Database**: IEEE OUI lookup library or embedded database
- **Optional**: `github.com/gorilla/sessions` for session management

This interactive mode will make the tool much more accessible to non-technical users while maintaining all the power and flexibility of the CLI version.</content>
<parameter name="filePath">c:\Users\kent.behrends\Documents\GitHub\Find-Meraki-Ports-With-MAC\Find-Meraki-Ports-With-MAC\prompt-interactive.md