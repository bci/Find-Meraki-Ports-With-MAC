// Find-Meraki-Ports-With-MAC - Interactive UI

class App {
  constructor() {
    this.apiKey = '';
    this.orgs = [];
    this.selectedOrg = null;
    this.networks = [];
    this.selectedNetwork = null;
    this.results = [];
    this.wsLogs = null;
    this.logFilter = 'DEBUG';
    this._sortCol = null;
    this._sortDir = 1; // 1 = asc, -1 = desc

    this._restorePrefs();
    this._bindEvents();
    this._connectLogSocket();
    this._loadConfig();
  }

  // ── Init ──────────────────────────────────────────────────

  async _loadConfig() {
    try {
      const res = await fetch('/api/config');
      const data = await res.json();
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

  _hideKeySection() {
    document.getElementById('keySection').classList.add('hidden');
  }

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

      // Restore previously selected org
      if (this._savedOrg) {
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

  async _loadNetworks(orgId) {
    const sel = document.getElementById('networkSelect');
    sel.innerHTML = '<option value="">Loading…</option>';
    document.getElementById('networkRow').classList.remove('hidden');
    try {
      const res = await fetch('/api/networks?orgId=' + orgId + '&apiKey=' + encodeURIComponent(this.apiKey));
      const data = await res.json();
      if (data.error) { this.toast(data.error, 'error'); return; }
      this.networks = data.networks || [];
      sel.innerHTML = '<option value="ALL">— All Networks —</option>';
      this.networks.forEach(n => {
        const opt = document.createElement('option');
        opt.value = n.id; opt.textContent = n.name;
        sel.appendChild(opt);
      });
      // restore / auto-select
      if (this._savedNetwork) {
        sel.value = this._savedNetwork;
        sel.dispatchEvent(new Event('change'));
      } else {
        // Default to All Networks
        sel.value = 'ALL';
        sel.dispatchEvent(new Event('change'));
      }
    } catch (e) { this.toast('Failed to load networks: ' + e.message, 'error'); }
  }

  // ── Events ────────────────────────────────────────────────

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

  async _resolve() {
    const mac = document.getElementById('macInput').value.trim();
    const ip  = document.getElementById('ipInput').value.trim();
    if (!mac && !ip) { this.toast('Enter a MAC or IP address', 'warn'); return; }
    if (!this.selectedNetwork) { this.toast('Select a network first', 'warn'); return; }

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

      // Manufacturer lookup
      if (effectiveMac) this._lookupManufacturer(effectiveMac);
    } catch (e) {
      this.toast('Resolve failed: ' + e.message, 'error');
    } finally {
      this._setBusy('resolveBtn', false, 'Resolve');
    }
  }

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
      case 'mode':         return (r.portMode || '').toLowerCase();
      default:             return '';
    }
  }

  _cmpIP(a, b) {
    const parse = ip => ip.split('.').map(n => parseInt(n, 10) || 0);
    const pa = parse(a), pb = parse(b);
    for (let i = 0; i < 4; i++) {
      if (pa[i] !== pb[i]) return pa[i] - pb[i];
    }
    return 0;
  }

  _cmpPort(a, b) {
    // Extract first integer sequence (e.g. "Gi1/0/3" → 1, "24" → 24)
    const num = s => { const m = s.match(/\d+/); return m ? parseInt(m[0], 10) : 0; };
    return num(a) - num(b);
  }

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

  _updateSortHeaders() {
    document.querySelectorAll('#resultsTable th.sortable').forEach(th => {
      th.classList.remove('sort-asc', 'sort-desc');
      if (th.dataset.col === this._sortCol) {
        th.classList.add(this._sortDir === 1 ? 'sort-asc' : 'sort-desc');
      }
    });
  }

  // ── Render results ────────────────────────────────────────

  _renderResults() {
    const tbody = document.getElementById('resultsTbody');
    const count = document.getElementById('resultsCount');
    const exportBtns = document.getElementById('exportBtns');

    tbody.innerHTML = '';
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

    sorted.forEach((r, idx) => {
      const tr = document.createElement('tr');
      const isTrunk = r.portMode === 'trunk';
      if (isTrunk) tr.classList.add('row-uplink');
      let modeCell;
      if (r.portMode === 'trunk') {
        modeCell = '<span class="mode-badge mode-trunk">Trunk</span>';
      } else if (r.portMode === 'access') {
        const vlanLabel = r.vlan ? ' ' + r.vlan : '';
        modeCell = '<span class="mode-badge mode-access">Access' + this._esc(vlanLabel) + '</span>';
      } else {
        modeCell = '—';
      }
      tr.innerHTML =
        '<td>' + this._esc(r.deviceName || r.switchName || '—') + '</td>' +
        '<td>' + this._esc(r.networkName || '—') + '</td>' +
        '<td class="cell-mono">' + this._esc(r.mac || '—') + '</td>' +
        '<td class="cell-mono">' + this._esc(r.ip || '—') + '</td>' +
        '<td>' + this._esc(r.port || r.portId || '—') + '</td>' +
        '<td>' + this._esc(String(r.vlan || '—')) + '</td>' +
        '<td>' + this._esc(r.hostname || '—') + '</td>' +
        '<td>' + (r.manufacturer ? '<span class="mfr-badge">' + this._esc(r.manufacturer) + '</span>' : '—') + '</td>' +
        '<td>' + modeCell + '</td>';

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
  }

  // ── Export ────────────────────────────────────────────────

  _exportCSV() {
    const header = ['Device','Network','MAC','IP','Port','VLAN','Hostname','Manufacturer','Mode'];
    const rows = this.results.map(r => [
      r.deviceName || r.switchName || '',
      r.networkName || '',
      r.mac || '', r.ip || '',
      r.port || r.portId || '',
      r.vlan || '', r.hostname || '',
      r.manufacturer || '',
      r.portMode || ''
    ].map(v => '"' + String(v).replace(/"/g, '""') + '"').join(','));
    this._download('meraki-results.csv', [header.join(','), ...rows].join('\r\n'), 'text/csv');
  }

  _exportJSON() {
    this._download('meraki-results.json', JSON.stringify(this.results, null, 2), 'application/json');
  }

  _download(filename, content, mime) {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([content], { type: mime }));
    a.download = filename;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  // ── Log WebSocket ─────────────────────────────────────────

  _connectLogSocket() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    this.wsLogs = new WebSocket(proto + '://' + location.host + '/ws/logs');
    this.wsLogs.onmessage = e => this._appendLog(e.data);
    this.wsLogs.onerror = () => {};
    this.wsLogs.onclose = () => setTimeout(() => this._connectLogSocket(), 3000);
  }

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

  _showResolve() {
    document.getElementById('resolveSection').classList.remove('hidden');
    document.getElementById('mfrRow').classList.add('hidden');
  }
  _hideResolve() {
    document.getElementById('resolveSection').classList.add('hidden');
  }

  _updateBadge(orgName, netName) {
    const badge = document.getElementById('contextBadge');
    if (!orgName) { badge.textContent = ''; return; }
    badge.textContent = netName ? orgName + ' › ' + netName : orgName;
  }

  _setStatus(text, type) {
    const row = document.getElementById('statusRow');
    const dot = document.getElementById('statusDot');
    const lbl = document.getElementById('statusLabel');
    dot.className = 'dot dot-' + type;
    lbl.textContent = text;
    row.classList.remove('hidden');
  }

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

  _esc(str) {
    return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  toast(msg, type = 'info') {
    const el = document.createElement('div');
    el.className = 'toast toast-' + type;
    el.textContent = msg;
    document.getElementById('toastContainer').appendChild(el);
    setTimeout(() => el.remove(), 4000);
  }

  // ── Prefs ─────────────────────────────────────────────────

  _savePrefs() {
    localStorage.setItem('meraki_prefs', JSON.stringify({
      org: this.selectedOrg,
      net: this.selectedNetwork,
      logFilter: this.logFilter
    }));
  }

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

document.addEventListener('DOMContentLoaded', () => { window._app = new App(); });
