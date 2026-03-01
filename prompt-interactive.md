# Interactive Web UI — As-Built Specification

This document is the authoritative as-built reference for the interactive web interface added to
`Find-Meraki-Ports-With-MAC`. It is written in enough detail that an AI (or developer) can
rebuild the feature from scratch without reading any source files first.

Last reflected commit: `52a0387`

---

## 1. Purpose & Invocation

The `--interactive` flag replaces the one-shot CLI execution with a local HTTP server that serves
a single-page web UI for Meraki port/device lookup.

```
Find-Meraki-Ports-With-MAC \
  --interactive \
  [--web-port 8080]       # default 8080
  [--web-host 0.0.0.0]    # default 0.0.0.0
  [--mac 00:11:22:33:44:55]   # pre-fill MAC input + auto-resolve
  [--ip  192.168.1.100]       # pre-fill IP input  + auto-resolve
  [--org "My Org Name"]       # auto-select org by name
  [--network "My Network"]    # auto-select network by name
```

When `--mac` or `--ip` is supplied together with `--interactive`, the UI will:
1. Auto-validate the API key (if it comes from `--key` or `MERAKI_API_KEY`).
2. Auto-select the matching org and network.
3. Fill the MAC/IP inputs.
4. Automatically click Resolve — the user sees results without any interaction.

---

## 2. Backend — `main.go`

### 2.1 Global preset vars

Declared at package level so route handlers can read them without threading `cfg` everywhere:

```go
var (
    webAPIKey        string
    webPresetMAC     string
    webPresetIP      string
    webPresetOrgName string
    webPresetNetwork string
)
```

### 2.2 `Config` struct additions

```go
type Config struct {
    // ...existing fields...
    MACAddress string  // MAC address or pattern to look up
}
```

### 2.3 `startWebServer(cfg Config, host, port string)`

1. Stores all preset globals from `cfg`:
   ```go
   webAPIKey        = cfg.APIKey
   webPresetMAC     = cfg.MACAddress
   webPresetIP      = cfg.IPAddress
   webPresetOrgName = cfg.OrgName
   webPresetNetwork = cfg.NetworkName
   ```
2. Builds a `gorilla/mux` router with the routes listed in §2.4.
3. Wraps `/static/` with a middleware that sets `Cache-Control: no-store` on every response
   **before** calling `http.FileServer`. This is essential during development — without it the
   browser caches `app.js` and new code never runs.
4. Logs the URL and opens the default system browser after a 500 ms delay (so the server is
   listening by the time the browser connects).
5. Calls `http.ListenAndServe(addr, r)`.

### 2.4 Routes

| Method | Path | Handler | Notes |
|--------|------|---------|-------|
| GET | `/` | `handleHome` | Serves the full HTML template (inline string literal) |
| GET | `/static/` | `http.FileServer` wrapped with no-cache middleware | Serves `static/` directory |
| POST | `/api/validate-key` | `handleValidateKey` | Body `{apiKey}` → `{organizations:[...]}` |
| GET | `/api/config` | `handleGetConfig` | Returns `{apiKey, presetMAC, presetIP, presetOrg, presetNetwork}` |
| GET | `/api/networks` | `handleGetNetworks` | Query `?orgId=&apiKey=` → `{networks:[...]}` |
| POST | `/api/resolve` | `handleResolve` | Body `{mac,ip,apiKey,orgId,networkId,networkIds[]}` → `{results:[...]}` |
| GET | `/api/manufacturer` | `handleGetManufacturer` | Query `?mac=` → `{manufacturer}` |
| GET | `/topology` | `handleTopology` | D3 force-graph HTML page (separate full-page UI) |
| GET | `/api/topology` | `handleGetTopology` | Returns topology JSON |
| GET | `/api/alerts` | `handleGetAlerts` | Returns alerts JSON |
| GET | `/api/logs` | `handleLogs` | Returns recent logs |
| GET | `/ws/logs` | `handleWebSocketLogs` | WebSocket; streams log events in real time |

### 2.5 `handleGetConfig`

```go
func handleGetConfig(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{
        "apiKey":        webAPIKey,
        "presetMAC":     webPresetMAC,
        "presetIP":      webPresetIP,
        "presetOrg":     webPresetOrgName,
        "presetNetwork": webPresetNetwork,
    })
}
```

### 2.6 `handleResolve` — critical details

Request body:
```json
{"mac":"", "ip":"", "apiKey":"", "orgId":"", "networkId":"", "networkIds":[]}
```

Normalisation:
- If `networkIds` is empty and `networkId` is non-empty, treat as single-item list.
- At least one network ID must be present after normalisation.

Per-network resolution loop:
```go
cfg := Config{
    APIKey:       req.APIKey,
    OrgID:        req.OrgID,
    NetworkName:  netID,
    LogLevel:     "INFO",
    MacTablePoll: firstNonZeroInt(parseIntEnv("MERAKI_MAC_POLL"), 15),
}
results, err := resolveDevices(cfg, req.MAC, req.IP)
```

**CRITICAL**: `MacTablePoll` **must not be zero**. If it is zero the MAC table job starts but the
poll loop runs 0 times, producing empty results. Use `firstNonZeroInt(parseIntEnv("MERAKI_MAC_POLL"), 15)`.
Networks that return an error (e.g. not a switch network) are silently skipped.

Response JSON per result row:
```json
{
  "orgName":      "...",
  "networkName":  "...",
  "deviceName":   "...",
  "deviceSerial": "...",
  "port":         "...",
  "mac":          "...",
  "ip":           "...",
  "hostname":     "...",
  "lastSeen":     "...",
  "manufacturer": "...",
  "vlan":         "...",
  "portMode":     "access|trunk"
}
```

---

## 3. HTML Template (`handleHome`)

The entire page is an inline Go string literal returned by `handleHome` with
`Content-Type: text/html`. There is **no separate HTML file** on disk.

### 3.1 DOM skeleton

```
<div id="app">
  <div class="topbar">
    <h1>🔍 Find Meraki Ports</h1>
    <span class="context-badge" id="contextBadge"></span>
    <div class="spacer"></div>
    <div class="status-row hidden" id="statusRow">
      <span class="dot dot-idle" id="statusDot"></span>
      <span id="statusLabel"></span>
    </div>
    <span class="version">v{Version}</span>
  </div>

  <div class="body-wrap">
    <!-- SIDEBAR -->
    <div class="sidebar">

      <!-- API Key card — class="card hidden" initially -->
      <div class="card hidden" id="keySection">
        <input type="password" id="apiKey" ...>
        <button id="validateKeyBtn">Validate Key</button>
      </div>

      <!-- Scope card (always visible) -->
      <div class="card">
        <div class="field hidden" id="orgRow">
          <select id="orgSelect">...</select>
        </div>
        <div class="field hidden" id="networkRow">
          <select id="networkSelect">...</select>
        </div>
        <div class="field" id="scopeHint">Loading organizations…</div>
      </div>

      <!-- Resolve card — class="card hidden" initially -->
      <div class="card hidden" id="resolveSection">
        <input type="text" id="macInput" placeholder="00:11:22:33:44:55">
        <input type="text" id="ipInput"  placeholder="192.168.1.100">
        <button id="resolveBtn">Resolve</button>
        <button id="topologyBtn"
            data-tip="Click a row in the results table to choose which switch to highlight">
          Topology</button>
        <button id="clearBtn">Clear</button>
        <!-- Manufacturer row — hidden until resolve -->
        <div class="hidden" id="mfrRow">
          <span class="mfr-badge" id="mfrBadge"></span>
        </div>
      </div>

    </div><!-- /sidebar -->

    <!-- MAIN -->
    <div class="main">

      <!-- Results card -->
      <div class="card" id="resultsWrap">
        <span class="results-count" id="resultsCount"></span>
        <div class="btn-group hidden" id="exportBtns">
          <button id="exportCsvBtn">↓ CSV</button>
          <button id="exportJsonBtn">↓ JSON</button>
        </div>
        <div id="uplinkNote" class="hidden uplink-note"></div>
        <table id="resultsTable">
          <thead>
            <tr>
              <th data-col="device" class="sortable">Switch / Device</th>
              <th data-col="network" class="sortable">Network</th>
              <th data-col="mac"    class="sortable">MAC</th>
              <th data-col="ip"     class="sortable">IP</th>
              <th data-col="port"   class="sortable">Port</th>
              <th data-col="vlan"   class="sortable">VLAN</th>
              <th data-col="hostname" class="sortable">Hostname</th>
              <th data-col="manufacturer" class="sortable">Manufacturer</th>
              <th data-col="mode"   class="sortable">Mode</th>
            </tr>
          </thead>
          <tbody id="resultsTbody">
            <tr><td colspan="9" class="no-results">
              Select a network and enter a MAC or IP address to begin.
            </td></tr>
          </tbody>
        </table>
      </div>

      <!-- Log card — collapsible -->
      <div class="card" id="logSection">
        <div class="card-header" id="logToggle">
          <h3>Logs <i class="chevron">▼</i></h3>
          <select id="logLevelSel">
            <option value="DEBUG">DEBUG</option>
            <option value="INFO">INFO</option>
            <option value="WARNING">WARNING</option>
            <option value="ERROR">ERROR</option>
          </select>
          <button id="clearLogsBtn">Clear</button>
          <button id="exportLogsBtn">↓ Export</button>
        </div>
        <div class="collapsible">
          <div class="log-console" id="logConsole"></div>
        </div>
      </div>

    </div><!-- /main -->
  </div><!-- /body-wrap -->
</div><!-- /app -->

<div id="toastContainer"></div>
<script src="/static/js/app.js"></script>
```

---

## 4. Frontend — `static/js/app.js`

Single vanilla-JS class `App` — no framework, no build step.

### 4.1 Constructor / state

```javascript
class App {
  constructor() {
    this.apiKey         = '';
    this.orgs           = [];
    this.selectedOrg    = null;
    this.networks       = [];
    this.selectedNetwork = null;
    this.results        = [];
    this.wsLogs         = null;
    this.logFilter      = 'DEBUG';
    this._sortCol       = null;
    this._sortDir       = 1;          // 1 = asc, -1 = desc
    this._preset        = { mac: '', ip: '', org: '', network: '' };
    this._autoResolvePending = false; // true when CLI supplied --mac or --ip
    this._presetApplied      = false; // guards preset input fill (one-time)
    this._selectedRowKey     = null;  // persists row selection: "mac|port|serial"
  }
}
```

### 4.2 Init sequence

```javascript
constructor() {
  // ...state...
  this._restorePrefs();
  this._bindEvents();
  this._connectLogSocket();
  this._loadConfig();
}
```

### 4.3 `_loadConfig()`

```javascript
async _loadConfig() {
  const cfg = await fetch('/api/config').then(r => r.json());
  this.apiKey = cfg.apiKey || '';
  this._preset = {
    mac:     cfg.presetMAC     || '',
    ip:      cfg.presetIP      || '',
    org:     cfg.presetOrg     || '',
    network: cfg.presetNetwork || '',
  };
  this._autoResolvePending = !!(this._preset.mac || this._preset.ip);
  if (this.apiKey) {
    this._hideKeySection();
    await this._validateAndLoadOrgs(this.apiKey);
  } else {
    document.getElementById('keySection').classList.remove('hidden');
    document.getElementById('scopeHint').textContent = 'Enter your API key to continue.';
  }
}
```

### 4.4 `_hideKeySection()` / `_showResolve()` / `_hideResolve()`

```javascript
_hideKeySection() {
  document.getElementById('keySection').classList.add('hidden');
}

_showResolve() {
  document.getElementById('resolveSection').classList.remove('hidden');
  document.getElementById('mfrRow').classList.add('hidden');

  // Fill inputs from CLI presets — exactly once
  if (!this._presetApplied && (this._preset.mac || this._preset.ip)) {
    this._presetApplied = true;
    if (this._preset.mac) document.getElementById('macInput').value = this._preset.mac;
    if (this._preset.ip)  document.getElementById('ipInput').value  = this._preset.ip;
  }

  // Auto-resolve — exactly once
  if (this._autoResolvePending) {
    this._autoResolvePending = false;
    setTimeout(() => this._resolve(), 0);
  }
}

_hideResolve() {
  document.getElementById('resolveSection').classList.add('hidden');
  document.getElementById('mfrRow').classList.add('hidden');
}
```

### 4.5 `_validateAndLoadOrgs(apiKey)`

POSTs `{apiKey}` to `/api/validate-key`. On success, stores `this.orgs`, calls `_populateOrgs()`.

Auto-select priority (first match wins):
1. Preset org — find by name (`o.name === this._preset.org`).
2. Saved org — `this._savedOrg` — **only if** its ID still exists in the new org list
   (validated with `.some(o => o.id === this._savedOrg)`).
3. Single-org auto-select — if `this.orgs.length === 1`.

On selection, updates `orgSelect.value`, stores `this.selectedOrg`, calls `_loadNetworks(orgId)`.

### 4.6 `_populateOrgs()`

Clears `#orgSelect`, adds a blank placeholder option, then appends one `<option>` per org.
Shows `#orgRow`, hides `#scopeHint`.

### 4.7 `_loadNetworks(orgId)`

GETs `/api/networks?orgId=&apiKey=`. Stores `this.networks`.

Populates `#networkSelect`: first option is `value="ALL"` "All Networks", then one per network.

Auto-select priority (first match wins):
1. Preset network — name match (`n.name === this._preset.network`); the string `"ALL"` selects
   the All Networks option.
2. Saved network — `this._savedNetwork` — **only if** the saved value equals `"ALL"` or its ID
   still exists in the current network list.
3. Default to `"ALL"`.

On selection, shows `#networkRow`, stores `this.selectedNetwork`, calls `_showResolve()`.

### 4.8 `_bindEvents()`

All UI event handlers wired here:

| Element | Event | Action |
|---------|-------|--------|
| `#validateKeyBtn` | click | `_validateAndLoadOrgs(apiKey.value)` |
| `#orgSelect` | change | `_loadNetworks(value)` |
| `#networkSelect` | change | update `selectedNetwork`, call `_showResolve()` |
| `#resolveBtn` | click | `_resolve()` |
| `#clearBtn` | click | clear inputs, clear results, hide `#mfrRow` |
| `#topologyBtn` | click | open topology URL in new tab |
| `#exportCsvBtn` | click | `_exportCSV()` |
| `#exportJsonBtn` | click | `_exportJSON()` |
| `thead th.sortable` | click | toggle sort column/direction, re-render |
| `#logToggle` | click | toggle `.collapsed` on `#logSection` |
| `#logLevelSel` | change | update `logFilter`, `_savePrefs()` |
| `#clearLogsBtn` | click | clear `#logConsole` |
| `#exportLogsBtn` | click | download log text |
| `#resultsTbody` | click (delegated) | set `_selectedRowKey`, re-render to highlight |

### 4.9 `_resolve()`

```javascript
async _resolve() {
  const mac  = document.getElementById('macInput').value.trim();
  const ip   = document.getElementById('ipInput').value.trim();
  if (!mac && !ip) { this.toast('Enter a MAC or IP address', 'warn'); return; }

  this._setBusy(true, 'Resolving…');

  const netId = this.selectedNetwork;
  const body  = { mac, ip, apiKey: this.apiKey, orgId: this.selectedOrg?.id };

  if (netId === 'ALL') {
    body.networkIds = this.networks.map(n => n.id);
  } else {
    body.networkId = netId;
  }

  const data = await fetch('/api/resolve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).then(r => r.json());

  this._setBusy(false);
  this.results = data.results || [];
  this._renderResults();

  if (this.results.length === 0) {
    this.toast('No results found', 'warn');
  } else {
    this.toast(`Found ${this.results.length} result(s)`, 'success');
    // If lookup was IP-only, fill macInput with the found MAC
    if (!mac && this.results[0]?.mac) {
      document.getElementById('macInput').value = this.results[0].mac;
    }
    this._lookupManufacturer(this.results[0].mac);
  }
  document.getElementById('exportBtns').classList.toggle('hidden', this.results.length === 0);
}
```

### 4.10 `_lookupManufacturer(mac)`

GETs `/api/manufacturer?mac=`. If result is non-empty, shows `#mfrRow` and sets `#mfrBadge` text.

### 4.11 Sort helpers

- `_colValue(row, col)` — extracts sortable value for a column key.
- `_cmpIP(a, b)` — numeric IP compare (splits on `.`, pads segments).
- `_cmpPort(a, b)` — numeric port compare (parses leading digits).
- `_sortedResults()` — returns `this.results` sorted by `_sortCol` / `_sortDir`.
- `_updateSortHeaders()` — sets `.sort-asc` / `.sort-desc` on the active `<th>`.

`<th data-col="...">` elements drive sorting. Clicking the same header twice reverses direction.

### 4.12 `_renderResults()`

1. Calls `_sortedResults()`.
2. Clears `#resultsTbody`.
3. If empty, inserts `<td colspan="9" class="no-results">No results found.</td>`.
4. Otherwise builds a `<tr>` per result:
   - `.row-uplink` class when `portMode === 'trunk'`
   - `.row-selected` class when row's key matches `_selectedRowKey`
   - `data-col` values set per cell for copy/reference
   - Mode badge: `<span class="mode-badge mode-trunk">trunk</span>` or `mode-access`
5. **Uplink note** — after rows:
   ```javascript
   const trunkResults = sorted.filter(r => r.portMode === 'trunk');
   if (trunkResults.length > 0) {
     const allTrunk = sorted.every(r => r.portMode === 'trunk');
     let html = `<strong>Possible uplink port(s) detected.</strong>
       The MAC address was seen on trunk/uplink port(s): <code>device:port</code> list.
       This may indicate the device is reachable through another switch.`;
     if (allTrunk) {
       html += `<p class="uplink-note-extra">
         All results are trunk ports. Try searching all networks to find the access port.</p>`;
     }
     document.getElementById('uplinkNote').innerHTML = html;
     document.getElementById('uplinkNote').classList.remove('hidden');
   } else {
     document.getElementById('uplinkNote').classList.add('hidden');
   }
   ```
6. Updates `#resultsCount` text.

### 4.13 Export

- `_exportCSV()` — builds CSV string with header row, calls `_download(text, 'results.csv', 'text/csv')`.
- `_exportJSON()` — `JSON.stringify(this.results, null, 2)`, calls `_download(...)`.
- `_download(content, filename, mime)` — creates a Blob URL, clicks a temporary `<a>`, revokes URL.

### 4.14 WebSocket log streaming

```javascript
_connectLogSocket() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  this.wsLogs = new WebSocket(`${proto}://${location.host}/ws/logs`);
  this.wsLogs.onmessage = e => this._appendLog(JSON.parse(e.data));
  this.wsLogs.onclose   = () => setTimeout(() => this._connectLogSocket(), 3000);
}

_appendLog(entry) {
  const levels = ['DEBUG','INFO','WARNING','ERROR'];
  if (levels.indexOf(entry.level) < levels.indexOf(this.logFilter)) return;
  const console = document.getElementById('logConsole');
  const line = document.createElement('div');
  line.className = `log-line log-${entry.level.toLowerCase()}`;
  line.textContent = `[${entry.level}] ${entry.message}`;
  console.appendChild(line);
  // Cap at 500 lines
  while (console.children.length > 500) console.removeChild(console.firstChild);
  console.scrollTop = console.scrollHeight;
}
```

### 4.15 UI helpers

- `_updateBadge()` — sets `#contextBadge` text to current org + network names.
- `_setStatus(text, state)` — shows/updates `#statusRow`; `state` is `'ok'|'warn'|'err'|'idle'`.
- `_setBusy(busy, label)` — shows spinner in `#resolveBtn`, sets status row.
- `_esc(str)` — HTML-escapes `<>&"'`.
- `toast(message, type)` — appends `.toast .toast-{type}` to `#toastContainer`, auto-removes after 4 s. Types: `success`, `error`, `info`, `warn`.

### 4.16 Preferences (localStorage)

Key: `meraki_prefs`. Shape: `{org, net, logFilter}`.

- `_savePrefs()` — called after org/network selection and log filter change.
- `_restorePrefs()` — called first in constructor; populates `this._savedOrg`, `this._savedNetwork`, `this.logFilter`.

**IMPORTANT**: Saved org/network IDs must be **validated** against the current list before use.
Stale IDs (from a previous API key) will silently produce wrong auto-selections.

---

## 5. CSS — `static/css/style.css`

### 5.1 Design tokens (CSS variables on `:root`)

```css
--fg: #111827;
--bg: #f9fafb;
--primary: #1d4ed8;
--primary-light: #eff6ff;
--border: #e5e7eb;
--radius: 8px;
--gray-50: #f9fafb; --gray-100: #f3f4f6; --gray-200: #e5e7eb;
--gray-300: #d1d5db; --gray-400: #9ca3af; --gray-500: #6b7280;
--gray-600: #4b5563; --gray-700: #374151; --gray-800: #1f2937;
--shadow-sm: 0 1px 3px rgba(0,0,0,.07), 0 1px 2px rgba(0,0,0,.05);
```

### 5.2 Layout

```
body → flex column → height 100vh
  .topbar → fixed-height flex row: title | context-badge | spacer | status-row | version
  .body-wrap → flex row flex-1 overflow-hidden
    .sidebar → 320px wide, overflow-y auto, border-right
    .main → flex-1, overflow-y auto, padding, gap between cards
```

### 5.3 Key class inventory

| Class | Purpose |
|-------|---------|
| `.topbar` | Top navigation bar |
| `.context-badge` | Small pill showing current org/network |
| `.body-wrap` | Flex row container for sidebar + main |
| `.sidebar` | Left panel |
| `.main` | Right content area |
| `.card` | White rounded card with shadow |
| `.card-header` | Flex row: title + optional controls |
| `.card-body` | Padding inside card |
| `.field` | Label + input stacked |
| `.hint` | Small grey helper text below input |
| `.btn` | Base button |
| `.btn-primary` | Blue filled button |
| `.btn-secondary` | Grey outlined button |
| `.btn-ghost` | No-border transparent button |
| `.btn-sm` | Smaller button |
| `.btn-wide` | Full-width button |
| `.btn-group` | Inline flex group of buttons |
| `.has-tip` | Element with tooltip; set `data-tip="..."` — shown via `::after` on hover |
| `.spinner` | Spinning CSS animation (light background) |
| `.spinner-dark` | Spinning CSS animation (dark background) |
| `#toastContainer` | Fixed bottom-right toast host |
| `.toast` | Base toast notification |
| `.toast-success / .toast-error / .toast-info / .toast-warn` | Toast colour variants |
| `.uplink-note` | Dark blue-gray bg, 3px amber left border (`#f59e0b`) — advisory callout |
| `.uplink-note code` | Monospace, dark bg inside uplink note |
| `.uplink-note-extra` | Amber text (`color:#fbbf24`) for all-trunk extra warning |
| `.dot` | Status indicator circle |
| `.dot-ok / .dot-warn / .dot-err / .dot-idle` | Colour variants for status dot |
| `.status-row` | Flex row: dot + label in topbar |
| `.results-toolbar` | Flex row above table |
| `.results-count` | Result count text |
| `.table-wrap` | Overflow-x auto wrapper for table |
| `table` | `border-collapse:collapse; width:100%` |
| `thead th.sortable` | Clickable header; `cursor:pointer`; has `::after` sort indicator |
| `.sort-asc` | `▲` indicator on active sort column |
| `.sort-desc` | `▼` indicator on active sort column |
| `.cell-mono` | Monospace font for MAC/IP cells |
| `.no-results` | Centered grey placeholder cell |
| `.mfr-badge` | Chip for manufacturer name |
| `.mode-badge` | Base port mode badge |
| `.mode-trunk` | Grey badge — trunk port |
| `.mode-access` | Blue badge — access port |
| `.row-uplink` | Muted text on trunk-port result rows |
| `.row-selected` | Amber highlight on clicked row (`background:rgba(245,158,11,.12)`) |
| `.log-console` | Dark terminal-style log area (`#0d1117` bg) |
| `.log-line` | Single log line |
| `.log-debug / .log-info / .log-warning / .log-error` | Colour per log level |
| `.log-toolbar` | Flex row of log controls |
| `.chevron` | Animated chevron icon for collapse |
| `.collapsible` | Content hidden when parent has `.collapsed` |
| `.collapsed` | Collapses `.collapsible` child; rotates `.chevron` |
| `.hidden` | `display:none !important` |

### 5.4 Responsive breakpoint

At `max-width: 700px`: sidebar becomes full-width, stacked above main; `.context-badge` hidden.

---

## 6. Known Bugs Fixed (do not repeat)

### Bug 1 — Browser serves cached `app.js`, new code never runs

**Symptom**: Console shows nothing, or shows old logs; UI behaviour unchanged after edits.  
**Cause**: Browser caches `/static/js/app.js` aggressively.  
**Fix**: Wrap the `http.FileServer` for `/static/` with a middleware that sets
`w.Header().Set("Cache-Control", "no-store")` **before** calling the next handler.

### Bug 2 — `MacTablePoll: 0` in `handleResolve` produces empty results

**Symptom**: Resolve returns `{results:[]}` even when the MAC is definitely in the network.  
**Cause**: The `Config` struct built inside `handleResolve` gets `MacTablePoll` as the zero value;
the MAC table job starts but never polls, so the table stays empty.  
**Fix**: `MacTablePoll: firstNonZeroInt(parseIntEnv("MERAKI_MAC_POLL"), 15)`.

### Bug 3 — Auto-resolve fires before network is selected (stale localStorage)

**Symptom**: Page loads, resolve fires immediately with a stale network ID from a previous
session; `_hideResolve()` is called when the empty select fires a change event.  
**Cause**: `_savedNetwork` was dispatching a change event with an empty/stale value before
`_loadNetworks` replaced the network list.  
**Fix**: Moved preset input filling and auto-resolve trigger entirely into `_showResolve()`.
`_showResolve()` is only called after a valid network has been selected, ensuring the resolve
fires at the right moment. Guards:
- `_presetApplied` — prevents filling inputs more than once.
- `_autoResolvePending` — prevents triggering resolve more than once.

### Bug 4 — Stale `_savedOrg` defeats single-org auto-select

**Symptom**: With a fresh API key that has one org, the org is not auto-selected because a
`_savedOrg` ID from a different key is found in localStorage and takes priority, but since it
doesn't match any org in the new key's list, `selectedOrg` is set to nothing.  
**Fix**: Before restoring saved org, validate:
```javascript
if (this._savedOrg && this.orgs.some(o => o.id === this._savedOrg)) { ... }
```

---

## 7. File Layout

```
static/
  css/
    style.css          ← all styles, no external CSS imports
  js/
    app.js             ← complete App class, no framework
main.go                ← handleHome (inline HTML), startWebServer, all handler funcs
                          global preset vars, handleGetConfig, handleResolve (MacTablePoll fix)
```

---

## 8. Help Text (`--help` / `printUsage()`)

All of these flags must appear in `printUsage()`:

```
--interactive       Launch the web UI (HTTP server)
--web-port PORT     Port to listen on (default 8080)
--web-host HOST     Host to bind to (default 0.0.0.0)
--mac ADDRESS       Pre-fill MAC address input; triggers auto-resolve if combined with --interactive
--ip ADDRESS        Pre-fill IP address input; triggers auto-resolve if combined with --interactive
--org NAME          Pre-select organization by name in web UI
--network NAME      Pre-select network by name in web UI
DNS_SERVERS         Env var for custom DNS servers (comma-separated)
```

Example entries for `--interactive`:
```
  Find-Meraki-Ports-With-MAC --interactive
  Find-Meraki-Ports-With-MAC --interactive --mac 00:11:22:33:44:55 --org "Acme Corp"
```
