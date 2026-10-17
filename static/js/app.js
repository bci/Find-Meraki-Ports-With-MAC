// Find-Meraki-Ports-With-MAC - Interactive UI

/**
 * @module app
 * @file Single-page UI for the --interactive web server.
 *
 * Served at /static/js/app.js and loaded by the page from handleHome (web.go).
 * It talks to the Go server only: GET /api/config, POST /api/validate-key,
 * GET /api/networks, POST /api/resolve, GET /api/manufacturer, and the
 * /ws/logs WebSocket. The API key is held in memory and sent with each
 * request; org/network/log-level choices persist in localStorage under
 * "meraki_prefs".
 */

/**
 * One organization as returned by /api/validate-key.
 * @typedef {Object} Org
 * @property {string} id   Meraki organization ID.
 * @property {string} name Organization name.
 */

/**
 * One network as returned by /api/networks.
 * @typedef {Object} Network
 * @property {string} id   Meraki network ID.
 * @property {string} name Network name.
 */

/**
 * One lookup result row as returned by /api/resolve (see handleResolve in
 * web_handlers.go).
 * @typedef {Object} Result
 * @property {string} orgName
 * @property {string} networkName
 * @property {string} deviceName    Switch name (serial when unnamed).
 * @property {string} deviceSerial
 * @property {string} port          Port ID, e.g. "24" or "AGGR/0".
 * @property {?string[]} aggrPorts  Member ports of an AGGR port, or null.
 * @property {string} mac           Colon-separated lowercase MAC.
 * @property {string} ip
 * @property {string} hostname
 * @property {string} lastSeen
 * @property {string} manufacturer  OUI vendor name, or "".
 * @property {number} vlan          Access/native VLAN, 0 if unknown.
 * @property {string} portMode      "access", "trunk" or "".
 * @property {boolean} isUplink     True when the switch reports the port as an uplink.
 */

/**
 * Controller for the whole page: owns UI state, binds DOM events, and talks
 * to the server. One instance is created on DOMContentLoaded as window._app.
 */
class App {
  /**
   * Initialise state, restore saved preferences, bind events, open the log
   * WebSocket and start loading config from the server.
   */
  constructor() {
    /** @type {string} Meraki API key in use (from /api/config or typed in). */
    this.apiKey = '';
    /** @type {Org[]} Organizations visible to the key. */
    this.orgs = [];
    /** @type {?string} Selected organization ID. */
    this.selectedOrg = null;
    /** @type {Network[]} Networks of the selected organization. */
    this.networks = [];
    /** @type {?string} Selected network ID, or "ALL". */
    this.selectedNetwork = null;
    /** @type {Result[]} Rows from the last resolve. */
    this.results = [];
    /** @type {?WebSocket} Connection to /ws/logs. */
    this.wsLogs = null;
    /** @type {string} Minimum log level shown in the console. */
    this.logFilter = 'DEBUG';
    /** @type {?string} Column key currently sorted on, or null for server order. */
    this._sortCol = null;
    /** @type {number} Sort direction. */
    this._sortDir = 1; // 1 = asc, -1 = desc
    /** @type {{mac: string, ip: string, org: string, network: string}} CLI presets from /api/config. */
    this._preset = { mac: '', ip: '', org: '', network: '' };
    /** @type {boolean} True until the one automatic resolve for presets/demo mode has run. */
    this._autoResolvePending = false;
    /** @type {boolean} True once preset MAC/IP have been copied into the inputs. */
    this._presetApplied = false;
    /** @type {boolean} True when the server runs with --test-data. */
    this._testDataMode = false;

    this._restorePrefs();
    this._bindEvents();
    this._connectLogSocket();
    this._loadConfig();
  }

  // ── Init ──────────────────────────────────────────────────

  /**
   * Fetch /api/config. If the server has a key, hide the key form and validate
   * it; otherwise show the key form. Also records CLI presets and demo mode.
   * @returns {Promise<void>}
   */
  async _loadConfig() {
    try {
      const res = await fetch('/api/config');
      const data = await res.json();
      // Store presets from CLI flags
      this._preset = {
        mac:     data.presetMAC     || '',
        ip:      data.presetIP      || '',
        org:     data.presetOrg     || '',
        network: data.presetNetwork || ''
      };
      this._testDataMode = !!data.testData;
      this._autoResolvePending = !!(this._preset.mac || this._preset.ip) || this._testDataMode;
      if (data.apiKey) {
        this.apiKey = data.apiKey;
        document.getElementById('apiKey').value = '••••••••••••••••';
        this._hideKeySection();
        await this._validateAndLoadOrgs(data.apiKey);
        return;
      }
    } catch (e) {
      console.warn('Config fetch failed:', e);
    }
    // No key from server - show key input
    document.getElementById('scopeHint').textContent = 'Enter an API key to begin.';
    document.getElementById('keySection').classList.remove('hidden');
  }

  /**
   * Hide the API key card.
   */
  _hideKeySection() {
    document.getElementById('keySection').classList.add('hidden');
  }

  /**
   * Validate apiKey via /api/validate-key, fill the organization list, and
   * select an org: the CLI preset by name, else the saved org, else the only
   * org. Errors are reported as toasts, not thrown.
   * @param {string} apiKey Meraki API key to validate.
   * @returns {Promise<void>}
   */
  async _validateAndLoadOrgs(apiKey) {
    this._setStatus('Connecting…', 'idle');
    try {
      const res = await fetch('/api/validate-key', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ apiKey })
      });
      const data = await res.json();
      if (data.error) { this._setStatus('Auth failed', 'err'); this.toast(data.error, 'error'); return; }
      this.orgs = data.organizations || [];
      this._populateOrgs();
      this._setStatus('Connected', 'ok');
      this.toast('API key validated — ' + this.orgs.length + ' org(s) found', 'success');

      // Prefer preset org (by name) over saved prefs, then single-org auto-select
      const preset = this._preset || {};
      if (preset.org) {
        const match = this.orgs.find(o => o.name === preset.org);
        if (match) {
          document.getElementById('orgSelect').value = match.id;
          document.getElementById('orgSelect').dispatchEvent(new Event('change'));
          return;
        }
      }
      // Restore previously selected org (validate it still exists for this key)
      if (this._savedOrg && this.orgs.some(o => o.id === this._savedOrg)) {
        document.getElementById('orgSelect').value = this._savedOrg;
        document.getElementById('orgSelect').dispatchEvent(new Event('change'));
      } else if (this.orgs.length === 1) {
        document.getElementById('orgSelect').value = this.orgs[0].id;
        document.getElementById('orgSelect').dispatchEvent(new Event('change'));
      }
    } catch (e) {
      this._setStatus('Error', 'err');
      this.toast('Validation failed: ' + e.message, 'error');
    }
  }

  /**
   * Rebuild the organization dropdown from this.orgs and show it.
   */
  _populateOrgs() {
    document.getElementById('scopeHint').classList.add('hidden');
    const sel = document.getElementById('orgSelect');
    sel.innerHTML = '<option value="">— Select organization —</option>';
    this.orgs.forEach(o => {
      const opt = document.createElement('option');
      opt.value = o.id; opt.textContent = o.name;
      sel.appendChild(opt);
    });
    document.getElementById('orgRow').classList.remove('hidden');
  }

  /**
   * Load networks for orgId into the network dropdown and select one: the CLI
   * preset by name, else the saved network if it still exists, else "ALL".
   * @param {string} orgId Organization ID.
   * @returns {Promise<void>}
   */
  async _loadNetworks(orgId) {
    const sel = document.getElementById('networkSelect');
    sel.innerHTML = '<option value="">Loading…</option>';
    document.getElementById('networkRow').classList.remove('hidden');
    try {
      const res = await fetch('/api/networks?orgId=' + encodeURIComponent(orgId) + '&apiKey=' + encodeURIComponent(this.apiKey));
      const data = await res.json();
      if (data.error) { this.toast(data.error, 'error'); return; }
      this.networks = data.networks || [];
      sel.innerHTML = '<option value="ALL">— All Networks —</option>';
      this.networks.forEach(n => {
        const opt = document.createElement('option');
        opt.value = n.id; opt.textContent = n.name;
        sel.appendChild(opt);
      });
      // Prefer preset network (by name) over saved prefs
      const preset = this._preset || {};
      if (preset.network) {
        if (preset.network.toUpperCase() === 'ALL') {
          sel.value = 'ALL';
        } else {
          const match = this.networks.find(n => n.name === preset.network);
          if (match) sel.value = match.id;
        }
        sel.dispatchEvent(new Event('change'));
        return;
      }
      // restore / auto-select
      if (this._savedNetwork) {
        // Only restore if the network ID still exists in this org; otherwise fall through to ALL
        const stillValid = this._savedNetwork === 'ALL' ||
          this.networks.some(n => n.id === this._savedNetwork);
        if (stillValid) {
          sel.value = this._savedNetwork;
          sel.dispatchEvent(new Event('change'));
          return;
        }
      }
      // Default to All Networks
      sel.value = 'ALL';
      sel.dispatchEvent(new Event('change'));
    } catch (e) { this.toast('Failed to load networks: ' + e.message, 'error'); }
  }

  // ── Events ────────────────────────────────────────────────

  /**
   * Attach all DOM event handlers. Called once from the constructor.
   */
  _bindEvents() {
    // Manual key entry
    document.getElementById('validateKeyBtn').addEventListener('click', async () => {
      const key = document.getElementById('apiKey').value.trim();
      if (!key || key.startsWith('•')) { this.toast('Enter a valid API key', 'warn'); return; }
      this.apiKey = key;
      this._setBusy('validateKeyBtn', true, 'Validating…');
      await this._validateAndLoadOrgs(key);
      this._setBusy('validateKeyBtn', false, 'Validate Key');
    });
    document.getElementById('apiKey').addEventListener('keydown', e => {
      if (e.key === 'Enter') document.getElementById('validateKeyBtn').click();
    });

    // Org select
    document.getElementById('orgSelect').addEventListener('change', async e => {
      const orgId = e.target.value;
      this.selectedOrg = orgId;
      this.selectedNetwork = null;
      this._savePrefs();
      if (!orgId) { document.getElementById('networkRow').classList.add('hidden'); this._hideResolve(); return; }
      const org = this.orgs.find(o => o.id === orgId);
      this._updateBadge(org ? org.name : '', '');
      await this._loadNetworks(orgId);
    });

    // Network select
    document.getElementById('networkSelect').addEventListener('change', e => {
      const netId = e.target.value;
      this.selectedNetwork = netId;
      this._savePrefs();
      if (!netId) { this._hideResolve(); return; }
      const net = this.networks.find(n => n.id === netId);
      const org = this.orgs.find(o => o.id === this.selectedOrg);
      const netLabel = netId === 'ALL' ? 'All Networks' : (net ? net.name : '');
      this._updateBadge(org ? org.name : '', netLabel);
      this._showResolve();
    });

    // Resolve
    document.getElementById('resolveBtn').addEventListener('click', () => this._resolve());
    document.getElementById('macInput').addEventListener('keydown', e => { if (e.key === 'Enter') this._resolve(); });
    document.getElementById('ipInput').addEventListener('keydown', e => { if (e.key === 'Enter') this._resolve(); });

    // Clear inputs
    document.getElementById('clearBtn').addEventListener('click', () => {
      document.getElementById('macInput').value = '';
      document.getElementById('ipInput').value = '';
      this.results = [];
      this._renderResults();
    });

    // Topology
    document.getElementById('topologyBtn').addEventListener('click', () => {
      if (!this.selectedNetwork) { this.toast('Select a network first', 'warn'); return; }
      // Use the row-selected result (click a row to change), fall back to first
      const first = this.selectedResult || (this.results && this.results[0]);
      const serial   = first ? (first.deviceSerial || '') : '';
      const port     = first ? (first.port || '') : '';
      const name     = first ? (first.deviceName || first.switchName || '') : '';
      const mac      = first ? (first.mac || '') : '';
      const portMode = first ? (first.portMode || '') : '';
      const hostname = first ? (first.hostname || '') : '';
      const netId    = this.selectedNetwork === 'ALL' ? (this.networks[0] && this.networks[0].id || '') : this.selectedNetwork;
      let url = '/topology?networkId=' + encodeURIComponent(netId)
              + '&orgId='    + encodeURIComponent(this.selectedOrg || '')
              + '&apiKey='   + encodeURIComponent(this.apiKey || '')
              + '&highlightSerial=' + encodeURIComponent(serial)
              + '&highlightPort='   + encodeURIComponent(port)
              + '&highlightName='   + encodeURIComponent(name)
              + '&portMode='        + encodeURIComponent(portMode)
              + '&mac='             + encodeURIComponent(mac)
              + '&hostname='        + encodeURIComponent(hostname);
      window.open(url, '_blank');
    });

    // Export
    document.getElementById('exportCsvBtn').addEventListener('click', () => this._exportCSV());
    document.getElementById('exportJsonBtn').addEventListener('click', () => this._exportJSON());

    // Table header sort
    document.getElementById('resultsTable').addEventListener('click', e => {
      const th = e.target.closest('th[data-col]');
      if (!th) return;
      const col = th.dataset.col;
      if (this._sortCol === col) {
        this._sortDir *= -1;
      } else {
        this._sortCol = col;
        this._sortDir = 1;
      }
      this._renderResults();
    });

    // Log controls
    document.getElementById('logLevelSel').addEventListener('change', e => {
      this.logFilter = e.target.value;
      this._savePrefs();
    });
    document.getElementById('clearLogsBtn').addEventListener('click', () => {
      document.getElementById('logConsole').innerHTML = '';
    });
    document.getElementById('exportLogsBtn').addEventListener('click', () => {
      const text = Array.from(document.querySelectorAll('.log-line')).map(el => el.textContent).join('\n');
      this._download('meraki-logs.txt', text, 'text/plain');
    });

    // Log section collapse
    document.getElementById('logToggle').addEventListener('click', () => {
      document.getElementById('logSection').classList.toggle('collapsed');
    });
  }

  // ── Resolve ───────────────────────────────────────────────

  /**
   * Run a lookup with the current MAC/IP inputs via /api/resolve, render the
   * results, back-fill whichever of MAC/IP was not entered, and look up the
   * manufacturer. "All Networks" sends every network ID in one request.
   * @returns {Promise<void>}
   */
  async _resolve() {
    const mac = document.getElementById('macInput').value.trim();
    const ip  = document.getElementById('ipInput').value.trim();
    if (!mac && !ip && !this._testDataMode) { this.toast('Enter a MAC or IP address', 'warn'); return; }
    if (!this.selectedNetwork && !this._testDataMode) { this.toast('Select a network first', 'warn'); return; }

    this._setBusy('resolveBtn', true, 'Resolving…');
    this.results = [];
    this._renderResults();

    const isAll = this.selectedNetwork === 'ALL';
    const payload = {
      mac, ip, apiKey: this.apiKey,
      orgId: this.selectedOrg,
      networkId:  isAll ? '' : this.selectedNetwork,
      networkIds: isAll ? this.networks.map(n => n.id) : []
    };

    try {
      const res = await fetch('/api/resolve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await res.json();
      if (data.error) { this.toast(data.error, 'error'); return; }
      this.results = data.results || [];
      this._renderResults();
      if (this.results.length === 0) { this.toast('No devices found', 'warn'); }
      else {
        this.toast(this.results.length + ' result(s) found', 'success');
      }

      // Determine the effective MAC to use for display/manufacturer lookup
      let effectiveMac = mac;
      if (ip && !mac && this.results[0] && this.results[0].mac) {
        effectiveMac = this.results[0].mac;
        document.getElementById('macInput').value = effectiveMac;
      }

      // If we searched by MAC and the results resolved an IP, back-fill the IP field
      if (!ip && this.results[0] && this.results[0].ip) {
        document.getElementById('ipInput').value = this.results[0].ip;
      }

      // Manufacturer lookup
      if (effectiveMac) this._lookupManufacturer(effectiveMac);
    } catch (e) {
      this.toast('Resolve failed: ' + e.message, 'error');
    } finally {
      this._setBusy('resolveBtn', false, 'Resolve');
    }
  }

  /**
   * Show the manufacturer badge for mac (best effort; errors are ignored).
   * @param {string} mac MAC address with separators.
   * @returns {Promise<void>}
   */
  async _lookupManufacturer(mac) {
    try {
      const res = await fetch('/api/manufacturer?mac=' + encodeURIComponent(mac));
      const data = await res.json();
      if (data.manufacturer && data.manufacturer !== 'Unknown') {
        document.getElementById('mfrBadge').textContent = data.manufacturer;
        document.getElementById('mfrRow').classList.remove('hidden');
      }
    } catch (e) { /* best-effort */ }
  }

  // ── Sort ──────────────────────────────────────────────────

  /**
   * Sort key for a result row and column.
   * @param {Result} r Result row.
   * @param {string} col Column key from the header's data-col attribute.
   * @returns {string} Value to compare; mode sorts uplinks as "uplink".
   */
  _colValue(r, col) {
    switch (col) {
      case 'device':       return (r.deviceName || r.switchName || '').toLowerCase();
      case 'network':      return (r.networkName || '').toLowerCase();
      case 'mac':          return (r.mac || '').toLowerCase();
      case 'ip':           return r.ip || '';
      case 'port':         return r.port || r.portId || '';
      case 'vlan':         return String(r.vlan || '').toLowerCase();
      case 'hostname':     return (r.hostname || '').toLowerCase();
      case 'manufacturer': return (r.manufacturer || '').toLowerCase();
      case 'mode':         return r.isUplink ? 'uplink' : (r.portMode || '').toLowerCase();
      default:             return '';
    }
  }

  /**
   * Compare two dotted IPv4 strings numerically, octet by octet.
   * @param {string} a
   * @param {string} b
   * @returns {number} Negative, zero or positive.
   */
  _cmpIP(a, b) {
    const parse = ip => {
      const parts = (ip || '').split('.');
      while (parts.length < 4) parts.push('0');
      return parts.slice(0, 4).map(n => parseInt(n, 10) || 0);
    };
    const pa = parse(a), pb = parse(b);
    for (let i = 0; i < 4; i++) {
      if (pa[i] !== pb[i]) return pa[i] - pb[i];
    }
    return 0;
  }

  /**
   * Compare two port labels by their first integer.
   * @param {string} a
   * @param {string} b
   * @returns {number} Negative, zero or positive.
   */
  _cmpPort(a, b) {
    // Extract first integer sequence (e.g. "Gi1/0/3" → 1, "24" → 24)
    const num = s => { const m = (s || '').match(/\d+/); return m ? parseInt(m[0], 10) : 0; };
    return num(a) - num(b);
  }

  /**
   * Results in the current sort order (a copy), or this.results unsorted.
   * @returns {Result[]}
   */
  _sortedResults() {
    if (!this._sortCol) return this.results;
    const col = this._sortCol;
    const dir = this._sortDir;
    return [...this.results].sort((ra, rb) => {
      const a = this._colValue(ra, col);
      const b = this._colValue(rb, col);
      let cmp;
      if (col === 'ip')   cmp = this._cmpIP(a, b);
      else if (col === 'port') cmp = this._cmpPort(a, b);
      else                cmp = a < b ? -1 : a > b ? 1 : 0;
      return cmp * dir;
    });
  }

  /**
   * Mark the sorted column header with sort-asc or sort-desc.
   */
  _updateSortHeaders() {
    document.querySelectorAll('#resultsTable th.sortable').forEach(th => {
      th.classList.remove('sort-asc', 'sort-desc');
      if (th.dataset.col === this._sortCol) {
        th.classList.add(this._sortDir === 1 ? 'sort-asc' : 'sort-desc');
      }
    });
  }

  // ── Render results ────────────────────────────────────────

  /**
   * Re-render the results table, keep or default the selected row, and show
   * the uplink or trunk advisory note when relevant.
   */
  _renderResults() {
    const tbody = document.getElementById('resultsTbody');
    const count = document.getElementById('resultsCount');
    const exportBtns = document.getElementById('exportBtns');
    const noteEl = document.getElementById('uplinkNote');

    tbody.innerHTML = '';
    noteEl.innerHTML = ''; noteEl.classList.add('hidden');
    if (!this.results || this.results.length === 0) {
      tbody.innerHTML = '<tr><td colspan="9" class="no-results">No results — enter a MAC or IP address and click Resolve.</td></tr>';
      count.textContent = '';
      exportBtns.classList.add('hidden');
      return;
    }

    count.textContent = this.results.length + ' result' + (this.results.length !== 1 ? 's' : '');
    exportBtns.classList.remove('hidden');
    this._updateSortHeaders();

    const sorted = this._sortedResults();
    // Re-apply selectedResult after re-render (match by mac+port+serial)
    const prevSel = this.selectedResult;
    this.selectedResult = null;
    let renderedCount = 0;

    sorted.forEach((r, idx) => {
      // Normalise aggrPorts: Go nil slice → JSON null → ensure it's always array or null
      if (r.aggrPorts !== null && r.aggrPorts !== undefined && !Array.isArray(r.aggrPorts)) {
        r.aggrPorts = null;
      }
      const tr = document.createElement('tr');
      let modeCell;
      if (r.isUplink) {
        modeCell = '<span class="mode-badge mode-uplink" title="Confirmed inter-switch uplink (link-layer topology)">Uplink</span>';
      } else if (r.portMode === 'trunk') {
        modeCell = '<span class="mode-badge mode-trunk">Trunk</span>';
      } else if (r.portMode === 'access') {
        const vlanLabel = r.vlan != null && r.vlan !== '' ? ' ' + r.vlan : '';
        modeCell = '<span class="mode-badge mode-access">Access' + this._esc(vlanLabel) + '</span>';
      } else {
        modeCell = '—';
      }
      const vlanDisplay = (r.vlan != null && r.vlan !== '') ? String(r.vlan) : '—';
      try {
        tr.innerHTML =
          '<td>' + this._esc(r.deviceName || r.switchName || '—') + '</td>' +
          '<td>' + this._esc(r.networkName || '—') + '</td>' +
          '<td class="cell-mono">' + this._esc(r.mac || '—') + '</td>' +
          '<td class="cell-mono">' + this._esc(r.ip || '—') + '</td>' +
          '<td>' + (() => {
            const portLabel = this._esc(r.port || r.portId || '—');
            if (r.aggrPorts && r.aggrPorts.length) {
              return portLabel + ' <span class="aggr-members" title="Member ports">(' + this._esc(r.aggrPorts.join(', ')) + ')</span>';
            }
            return portLabel;
          })() + '</td>' +
          '<td>' + this._esc(vlanDisplay) + '</td>' +
          '<td>' + this._esc(r.hostname || '—') + '</td>' +
          '<td>' + (r.manufacturer ? '<span class="mfr-badge">' + this._esc(r.manufacturer) + '</span>' : '—') + '</td>' +
          '<td>' + modeCell + '</td>';
      } catch(e) {
        console.error('Row render error for result:', r, e);
        tr.innerHTML = '<td colspan="9" style="color:red;background:#fee2e2;padding:6px">⚠ Error rendering row (' + this._esc(r.port || '?') + ' / ' + this._esc(r.mac || '?') + '): ' + this._esc(String(e)) + '</td>';
      }
      if (r.isUplink) tr.classList.add('row-uplink-confirmed');
      renderedCount++;

      tr.addEventListener('click', () => {
        tbody.querySelectorAll('tr').forEach(t => t.classList.remove('row-selected'));
        tr.classList.add('row-selected');
        this.selectedResult = r;
      });

      // Auto-select: restore previous selection or default to first row
      const isSame = prevSel && prevSel.mac === r.mac && prevSel.port === r.port
                     && prevSel.deviceSerial === r.deviceSerial;
      if (isSame || (!prevSel && idx === 0)) {
        tr.classList.add('row-selected');
        this.selectedResult = r;
      }

      tbody.appendChild(tr);
    });

    // Update count to match what was actually rendered
    count.textContent = renderedCount + ' result' + (renderedCount !== 1 ? 's' : '');

    // Note: confirmed uplinks (from topology) get a specific warning.
    // Unconfirmed trunk ports get a softer neutral note.
    const uplinkResults = sorted.filter(r => r.isUplink);
    const trunkOnlyResults = sorted.filter(r => !r.isUplink && r.portMode === 'trunk' && !(r.port || '').startsWith('AGGR'));

    if (uplinkResults.length > 0) {
      noteEl.innerHTML =
        '<strong>&#9650; Uplink detected:</strong> ' +
        (uplinkResults.length > 1 ? 'These ports are' : 'This port is') +
        ' a confirmed inter-switch uplink according to the Meraki link-layer topology.' +
        ' The MAC address belongs to a device further upstream — this switch is not the access point.';
      noteEl.classList.remove('hidden');
    } else if (trunkOnlyResults.length > 0) {
      noteEl.innerHTML =
        '<strong>&#9432; Note:</strong> This MAC was found on a <em>trunk</em> port (carries multiple VLANs). ' +
        'This may be an uplink, a multi-VLAN device (phone, AP, server), or a misconfigured port.';
      noteEl.classList.remove('hidden');
    }
  }

  // ── Export ────────────────────────────────────────────────

  /**
   * Download the current results as meraki-results.csv (every field quoted).
   */
  _exportCSV() {
    const header = ['Device','Network','MAC','IP','Port','AggrPorts','VLAN','Hostname','Manufacturer','Mode','Uplink'];
    const rows = this.results.map(r => [
      r.deviceName || r.switchName || '',
      r.networkName || '',
      r.mac || '', r.ip || '',
      r.port || r.portId || '',
      (r.aggrPorts && r.aggrPorts.length) ? r.aggrPorts.join(', ') : '',
      r.vlan || '', r.hostname || '',
      r.manufacturer || '',
      r.portMode || '',
      r.isUplink ? 'yes' : ''
    ].map(v => '"' + String(v).replace(/"/g, '""') + '"').join(','));
    this._download('meraki-results.csv', [header.join(','), ...rows].join('\r\n'), 'text/csv');
  }

  /**
   * Download the current results as meraki-results.json.
   */
  _exportJSON() {
    this._download('meraki-results.json', JSON.stringify(this.results, null, 2), 'application/json');
  }

  /**
   * Trigger a browser download of in-memory content.
   * @param {string} filename Suggested file name.
   * @param {string} content File contents.
   * @param {string} mime MIME type.
   */
  _download(filename, content, mime) {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([content], { type: mime }));
    a.download = filename;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  // ── Log WebSocket ─────────────────────────────────────────

  /**
   * Open the /ws/logs WebSocket, reconnecting 3 s after any close.
   */
  _connectLogSocket() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    this.wsLogs = new WebSocket(proto + '://' + location.host + '/ws/logs');
    this.wsLogs.onmessage = e => this._appendLog(e.data);
    this.wsLogs.onerror = () => {};
    this.wsLogs.onclose = () => setTimeout(() => this._connectLogSocket(), 3000);
  }

  /**
   * Append one log line to the console if its level passes the filter, keeping
   * at most 500 lines. The level is read from a "[LEVEL]" tag; untagged lines
   * count as INFO.
   * @param {string} message Raw log line.
   */
  _appendLog(message) {
    const level = (message.match(/\[(DEBUG|INFO|WARNING|ERROR)\]/i) || [])[1] || 'INFO';
    const levels = { DEBUG: 0, INFO: 1, WARNING: 2, ERROR: 3 };
    if (levels[level.toUpperCase()] < levels[this.logFilter]) return;

    const div = document.createElement('div');
    div.className = 'log-line log-' + level.toLowerCase();
    div.textContent = message;
    const console_ = document.getElementById('logConsole');
    console_.appendChild(div);
    // Keep last 500 lines
    while (console_.children.length > 500) console_.removeChild(console_.firstChild);
    console_.scrollTop = console_.scrollHeight;
  }

  // ── UI helpers ────────────────────────────────────────────

  /**
   * Show the lookup card, apply CLI presets once, and trigger the one-time
   * automatic resolve when pending.
   */
  _showResolve() {
    document.getElementById('resolveSection').classList.remove('hidden');
    document.getElementById('mfrRow').classList.add('hidden');
    // Apply CLI presets to inputs once when the section first becomes visible
    if (!this._presetApplied) {
      this._presetApplied = true;
      if (this._preset.mac) document.getElementById('macInput').value = this._preset.mac;
      if (this._preset.ip)  document.getElementById('ipInput').value  = this._preset.ip;
    }
    // Auto-resolve once if CLI supplied MAC/IP or in test-data mode
    if (this._autoResolvePending) {
      this._autoResolvePending = false;
      setTimeout(() => this._resolve(), 0);
    }
  }
  /**
   * Hide the lookup card.
   */
  _hideResolve() {
    document.getElementById('resolveSection').classList.add('hidden');
  }

  /**
   * Show "org › network" in the top bar.
   * @param {string} orgName Organization name; empty clears the badge.
   * @param {string} netName Network label; may be empty.
   */
  _updateBadge(orgName, netName) {
    const badge = document.getElementById('contextBadge');
    if (!orgName) { badge.textContent = ''; return; }
    badge.textContent = netName ? orgName + ' › ' + netName : orgName;
  }

  /**
   * Show the connection status dot and label.
   * @param {string} text Label text.
   * @param {string} type "idle", "ok" or "err" (selects the dot-* CSS class).
   */
  _setStatus(text, type) {
    const row = document.getElementById('statusRow');
    const dot = document.getElementById('statusDot');
    const lbl = document.getElementById('statusLabel');
    dot.className = 'dot dot-' + type;
    lbl.textContent = text;
    row.classList.remove('hidden');
  }

  /**
   * Disable a button and show a spinner, or restore it.
   * @param {string} id Button element ID.
   * @param {boolean} busy True to show the busy state.
   * @param {string} label Text shown while busy, or fallback when restoring.
   */
  _setBusy(id, busy, label) {
    const el = document.getElementById(id);
    el.disabled = busy;
    if (busy) {
      el.dataset.orig = el.textContent;
      el.innerHTML = '<span class="spinner"></span> ' + label;
    } else {
      el.innerHTML = el.dataset.orig || label;
    }
  }

  /**
   * Escape a value for safe insertion into HTML text or a quoted attribute.
   * @param {*} str Value to escape (converted with String()).
   * @returns {string}
   */
  _esc(str) {
    return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  /**
   * Show a transient notification for 4 seconds.
   * @param {string} msg Message text (inserted as text, not HTML).
   * @param {string} [type='info'] "info", "success", "warn" or "error".
   */
  toast(msg, type = 'info') {
    const el = document.createElement('div');
    el.className = 'toast toast-' + type;
    el.textContent = msg;
    document.getElementById('toastContainer').appendChild(el);
    setTimeout(() => el.remove(), 4000);
  }

  // ── Prefs ─────────────────────────────────────────────────

  /**
   * Persist the selected org, network and log filter to localStorage.
   */
  _savePrefs() {
    localStorage.setItem('meraki_prefs', JSON.stringify({
      org: this.selectedOrg,
      net: this.selectedNetwork,
      logFilter: this.logFilter
    }));
  }

  /**
   * Load saved org, network and log filter from localStorage, ignoring bad data.
   */
  _restorePrefs() {
    try {
      const p = JSON.parse(localStorage.getItem('meraki_prefs') || '{}');
      this._savedOrg     = p.org || null;
      this._savedNetwork = p.net || null;
      this.logFilter     = p.logFilter || 'DEBUG';
      const sel = document.getElementById('logLevelSel');
      if (sel) sel.value = this.logFilter;
    } catch (e) {}
  }
}

// Create the single App instance once the DOM is ready; exposed as
// window._app for debugging from the browser console.
document.addEventListener('DOMContentLoaded', () => { window._app = new App(); });
