/**
 * topology.js — D3 force-graph for network topology view.
 * Served at /static/js/topology.js by the static file server.
 *
 * Configuration is read from <meta> tags written by handleTopology:
 *   <meta name="topo-network-id"       content="...">
 *   <meta name="topo-org-id"           content="...">
 *   <meta name="topo-api-key"          content="...">
 *   <meta name="topo-highlight-serial" content="...">
 *   <meta name="topo-highlight-port"   content="...">
 *   <meta name="topo-highlight-name"   content="...">
 *   <meta name="topo-port-mode"        content="...">
 *   <meta name="topo-highlight-mac"    content="...">
 *   <meta name="topo-highlight-host"   content="...">
 */

(function () {
  'use strict';

  function meta(name) {
    const el = document.querySelector('meta[name="' + name + '"]');
    return el ? el.getAttribute('content') || '' : '';
  }

  const NETWORK_ID        = meta('topo-network-id');
  const ORG_ID            = meta('topo-org-id');
  const API_KEY           = meta('topo-api-key');
  const HIGHLIGHT_SERIAL  = meta('topo-highlight-serial');
  const HIGHLIGHT_PORT    = meta('topo-highlight-port');
  const HIGHLIGHT_NAME    = meta('topo-highlight-name');
  const PORT_MODE         = meta('topo-port-mode');
  const HIGHLIGHT_MAC     = meta('topo-highlight-mac');
  const HIGHLIGHT_HOSTNAME= meta('topo-highlight-host');

  const svg     = d3.select('#svg');
  const root    = d3.select('#root');
  const tooltip = document.getElementById('tooltip');
  const netPill = document.getElementById('netPill');
  const hlPill  = document.getElementById('hlPill');

  if (HIGHLIGHT_NAME || HIGHLIGHT_SERIAL) {
    const label = HIGHLIGHT_NAME || HIGHLIGHT_SERIAL;
    hlPill.textContent = '📍 ' + label + (HIGHLIGHT_PORT ? ' port ' + HIGHLIGHT_PORT : '');
    hlPill.style.display = '';
  }

  const url = '/api/topology?networkId=' + encodeURIComponent(NETWORK_ID)
            + '&apiKey='   + encodeURIComponent(API_KEY);

  let zoomBehavior;

  fetch(url)
    .then(r => r.json())
    .then(data => {
      const nodes = data.nodes || [];
      const links = data.links || [];
      netPill.textContent = data.networkName || ('Network: ' + NETWORK_ID.slice(0, 12));

      // Inject a virtual PC node for access-mode ports
      if (PORT_MODE === 'access' && HIGHLIGHT_SERIAL) {
        if (!nodes.find(n => n.id === HIGHLIGHT_SERIAL)) {
          nodes.push({ id: HIGHLIGHT_SERIAL, name: HIGHLIGHT_NAME || HIGHLIGHT_SERIAL,
                       type: 'switch' });
        }
        const pcLabel = HIGHLIGHT_HOSTNAME || HIGHLIGHT_MAC || 'Device';
        nodes.push({ id: '__pc__', name: pcLabel, type: 'pc', isPc: true,
                     mac: HIGHLIGHT_MAC, hostname: HIGHLIGHT_HOSTNAME });
        links.push({ source: '__pc__', target: HIGHLIGHT_SERIAL, isPcLink: true, port: HIGHLIGHT_PORT });
        document.getElementById('pcLegend').style.display = '';
      } else if (HIGHLIGHT_SERIAL && !nodes.find(n => n.id === HIGHLIGHT_SERIAL)) {
        nodes.push({ id: HIGHLIGHT_SERIAL, name: HIGHLIGHT_NAME || HIGHLIGHT_SERIAL,
                     type: 'switch' });
      }

      renderGraph(nodes, links);
    })
    .catch(err => {
      netPill.textContent = 'Error loading topology';
      console.error(err);
    });

  function nodeClass(d) {
    if (d.isPc) return 'pc';
    const t = (d.type || '').toLowerCase();
    if (t.includes('switch')) return 'switch';
    if (t.includes('wireless') || t.includes('ap') || t.includes('mr')) return 'wireless';
    if (t.includes('appliance') || t.includes('mx') || t.includes('vpn')) return 'appliance';
    if (t.includes('camera') || t.includes('mv')) return 'camera';
    return 'other';
  }

  function renderGraph(nodes, links) {
    const W = document.getElementById('canvas').clientWidth;
    const H = document.getElementById('canvas').clientHeight;

    zoomBehavior = d3.zoom().scaleExtent([0.1, 4]).on('zoom', e => root.attr('transform', e.transform));
    svg.call(zoomBehavior);

    // Links
    const linkSel = root.append('g').attr('class', 'links').selectAll('line')
      .data(links).join('line')
      .attr('class', d => {
        if (d.isPcLink) return 'link pc-link';
        const src = typeof d.source === 'object' ? d.source.id : d.source;
        const tgt = typeof d.target === 'object' ? d.target.id : d.target;
        return 'link' + (src === HIGHLIGHT_SERIAL || tgt === HIGHLIGHT_SERIAL ? ' highlighted' : '');
      });

    // Port labels on PC-link (shown near the switch)
    const portLabelSel = root.append('g').attr('class', 'port-labels').selectAll('text')
      .data(links.filter(l => l.isPcLink && l.port)).join('text')
      .attr('class', 'port-label')
      .text(d => 'port ' + d.port);

    // Nodes
    const nodeSel = root.append('g').attr('class', 'nodes').selectAll('g')
      .data(nodes).join('g')
      .attr('class', d => 'node ' + nodeClass(d) + (d.id === HIGHLIGHT_SERIAL ? ' highlighted' : ''))
      .call(d3.drag()
        .on('start', (e, d) => { if (!e.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; })
        .on('drag',  (e, d) => { d.fx = e.x; d.fy = e.y; })
        .on('end',   (e, d) => { if (!e.active) sim.alphaTarget(0); d.fx = null; d.fy = null; }))
      .on('mousemove', (e, d) => {
        tooltip.style.opacity = 1;
        tooltip.style.left = (e.clientX + 14) + 'px';
        tooltip.style.top  = (e.clientY - 10) + 'px';
        if (d.isPc) {
          tooltip.innerHTML = '<div class="tt-title">🖥 Found Device</div>' +
            '<div class="tt-row hi">MAC: <span>' + (d.mac || '—') + '</span></div>' +
            (d.hostname ? '<div class="tt-row hi">Host: <span>' + d.hostname + '</span></div>' : '') +
            (HIGHLIGHT_PORT ? '<div class="tt-row hi">Port: <span>' + HIGHLIGHT_PORT + '</span></div>' : '') +
            '<div class="tt-row">Mode: <span>Access</span></div>';
        } else {
          tooltip.innerHTML = '<div class="tt-title">' + (d.name || d.id) + '</div>' +
            '<div class="tt-row">Type: <span>' + (d.type || '—') + '</span></div>' +
            '<div class="tt-row">Serial: <span>' + (d.id || '—') + '</span></div>' +
            '<div class="tt-row">Model: <span>' + (d.model || '—') + '</span></div>' +
            (d.id === HIGHLIGHT_SERIAL && HIGHLIGHT_PORT
              ? '<div class="tt-row hi">Port ' + HIGHLIGHT_PORT + ': <span>Device connected</span></div>'
              : '');
        }
      })
      .on('mouseleave', () => { tooltip.style.opacity = 0; });

    // PC nodes: monitor icon as SVG rects
    const pcSel = nodeSel.filter(d => d.isPc);
    pcSel.append('rect').attr('class', 'pc-screen').attr('x', -14).attr('y', -14).attr('width', 28).attr('height', 18).attr('rx', 2);
    pcSel.append('rect').attr('class', 'pc-display').attr('x', -11).attr('y', -11).attr('width', 22).attr('height', 12).attr('rx', 1).style('fill', '#0f172a');
    pcSel.append('rect').attr('class', 'pc-stand').attr('x', -2).attr('y', 4).attr('width', 4).attr('height', 7);
    pcSel.append('rect').attr('class', 'pc-base').attr('x', -7).attr('y', 10).attr('width', 14).attr('height', 3);

    // Regular nodes: circles
    nodeSel.filter(d => !d.isPc)
      .append('circle').attr('r', d => d.id === HIGHLIGHT_SERIAL ? 18 : 12);

    // Labels
    nodeSel.append('text')
      .attr('dy', d => d.isPc ? 30 : (d.id === HIGHLIGHT_SERIAL ? 32 : 26))
      .text(d => (d.name || d.id || '').slice(0, 22));

    /* jshint ignore:start */
    var sim = d3.forceSimulation(nodes)
      .force('link', d3.forceLink(links).id(d => d.id).distance(d => d.isPcLink ? 80 : 120))
      .force('charge', d3.forceManyBody().strength(-400))
      .force('center', d3.forceCenter(W / 2, H / 2))
      .force('collision', d3.forceCollide(32))
      .on('tick', () => {
        linkSel.attr('x1', d => d.source.x).attr('y1', d => d.source.y)
               .attr('x2', d => d.target.x).attr('y2', d => d.target.y);
        portLabelSel
          .attr('x', d => d.source.x + (d.target.x - d.source.x) * 0.78)
          .attr('y', d => d.source.y + (d.target.y - d.source.y) * 0.78 - 6);
        nodeSel.attr('transform', d => 'translate(' + d.x + ',' + d.y + ')');
      });
    /* jshint ignore:end */

    // After settling, center on the switch (or PC if no serial given)
    const centerTarget = HIGHLIGHT_SERIAL || '__pc__';
    setTimeout(() => {
      const hn = nodes.find(n => n.id === centerTarget);
      if (hn && hn.x != null) {
        const scale = 1.5;
        const tx = W / 2 - scale * hn.x;
        const ty = H / 2 - scale * hn.y;
        svg.transition().duration(800)
          .call(zoomBehavior.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
      }
    }, 2200);
  }

  document.getElementById('resetBtn').addEventListener('click', () => {
    svg.transition().duration(400).call(zoomBehavior.transform, d3.zoomIdentity);
  });
}());
