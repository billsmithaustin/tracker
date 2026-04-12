/* global L, Chart, ROUTE_WAYPOINTS, ROUTE_ELEVATION, ROUTE_SECTIONS, latLngForMile, currentSection, ROUTE_TOTAL_MILES, BIKE_COOPS, ROUTE_STOPS */

const API = '/api';
const POLL_MS = 30 * 60_000;

// ── State ─────────────────────────────────────────────────────────────────────
let map, riderMarker, elevChart;
let lastCheckinId = null;
let riderXLabel = null, riderElevFt = null;

// ── Init ──────────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  initMap();
  initElevChart();
  fetchAndRender();
  setInterval(fetchAndRender, POLL_MS);
});

// ── Map ───────────────────────────────────────────────────────────────────────
function initMap() {
  map = L.map('map', {
    center: [39.5, -100],
    zoom: 4,
    minZoom: 4,
    zoomControl: false,
    attributionControl: true,
  });
  const zoomControl = L.control.zoom({ position: 'bottomleft' }).addTo(map);

  // Append reset button into the same leaflet-bar as +/−
  const resetBtn = L.DomUtil.create('a', 'map-reset-btn', zoomControl.getContainer());
  resetBtn.title = 'Reset view';
  resetBtn.href = '#';
  resetBtn.innerHTML = '&#8962;'; // ⌂
  L.DomEvent.on(resetBtn, 'click', L.DomEvent.stopPropagation)
            .on(resetBtn, 'click', L.DomEvent.preventDefault)
            .on(resetBtn, 'click', () => map.setView([39.5, -100], 4));

  L.tileLayer('https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png', {
    attribution: '&copy; <a href="https://carto.com/">CARTO</a>',
    subdomains: 'abcd',
    maxZoom: 19,
  }).addTo(map);

  // Draw full route
  const latlngs = ROUTE_WAYPOINTS.map(w => [w.lat, w.lng]);
  L.polyline(latlngs, {
    color: '#7ab0d0',
    weight: 3,
    opacity: 0.7,
  }).addTo(map);

  // Key waypoints (start, end, major towns)
  const keyWaypoints = [0, ROUTE_WAYPOINTS.length - 1];
  for (const idx of keyWaypoints) {
    const w = ROUTE_WAYPOINTS[idx];
    L.marker([w.lat, w.lng], {
      icon: L.divIcon({ className: 'waypoint-dot', iconSize: [7, 7] }),
    }).bindTooltip(w.name, { permanent: false, direction: 'top', className: 'leaflet-tooltip-dark' })
      .addTo(map);
  }

  // Start / finish labels
  addTextMarker(ROUTE_WAYPOINTS[0], '⬟ START (YORKTOWN)');
  addTextMarker(ROUTE_WAYPOINTS.at(-1), '⬟ FINISH (ASTORIA)');

  // ── Layer definitions ────────────────────────────────────────────────────
  const LAYER_DEFS = [
    { key: 'towns',     label: 'Towns',        color: '#7abcd8', markerClass: 'stop-marker-town'      },
    { key: 'campsite',  label: 'Campsites',   color: '#27ae60', markerClass: 'stop-marker-campsite'  },
    { key: 'lodging',   label: 'Lodging',      color: '#9b59b6', markerClass: 'stop-marker-lodging'   },
    { key: 'bike_shop', label: 'Bike Shops',   color: '#e67e22', markerClass: 'stop-marker-bike_shop' },
    { key: 'library',   label: 'Libraries',    color: '#2980b9', markerClass: 'stop-marker-library'   },
    { key: 'services',  label: 'Services',     color: '#f39c12', markerClass: 'stop-marker-services'  },
    { key: 'food',      label: 'Food',         color: '#e74c3c', markerClass: 'stop-marker-food'      },
    { key: 'caution',   label: 'Caution',      color: '#e74c3c', markerClass: 'stop-marker-caution'   },
    { key: 'coop',      label: 'Bike Co-ops',  color: '#ff9d00', markerClass: 'coop-marker'           },
  ];

  // Build layer groups from ROUTE_STOPS
  const layerGroups = {};
  for (const def of LAYER_DEFS) {
    layerGroups[def.key] = L.layerGroup();
  }

  for (const s of ROUTE_STOPS) {
    const def = LAYER_DEFS.find(d => d.key === s.cat);
    if (!def) continue;
    const icon = L.divIcon({ className: def.markerClass, iconSize: [10, 10], iconAnchor: [5, 5] });
    const popup = `<div class="stop-popup">
      <div class="stop-popup-name" style="color:${def.color}">${escHtml(s.name)}</div>
      <div class="stop-popup-cat">${def.label}</div>
      ${s.desc ? `<div class="stop-popup-desc">${escHtml(s.desc)}</div>` : ''}
    </div>`;
    L.marker([s.lat, s.lng], { icon }).bindPopup(popup, { className: 'stop-popup-wrap' })
      .addTo(layerGroups[s.cat]);
  }

  // Town layer (from GPX geocache waypoints)
  for (const t of ROUTE_TOWNS) {
    const icon = L.divIcon({ className: 'stop-marker-town', iconSize: [6, 6], iconAnchor: [3, 3] });
    const popup = `<div class="stop-popup">
      <div class="stop-popup-name" style="color:#7abcd8">${escHtml(t.name)}</div>
      <div class="stop-popup-cat">Town</div>
    </div>`;
    L.marker([t.lat, t.lng], { icon }).bindPopup(popup, { className: 'stop-popup-wrap' })
      .addTo(layerGroups['towns']);
  }

  // Bike co-op layer (from separate curated data)
  for (const c of BIKE_COOPS) {
    const icon = L.divIcon({ className: 'coop-marker', iconSize: [10, 10], iconAnchor: [5, 5] });
    const popup = `<div class="stop-popup">
      <div class="stop-popup-name" style="color:#ff9d00">${escHtml(c.name)}</div>
      <div class="stop-popup-cat">Bike Co-op · ${escHtml(c.town)}, ${escHtml(c.state)}</div>
      <div class="stop-popup-desc"><a href="${c.url}" target="_blank" rel="noopener">Website →</a></div>
    </div>`;
    L.marker([c.lat, c.lng], { icon }).bindPopup(popup, { className: 'stop-popup-wrap' })
      .addTo(layerGroups['coop']);
  }

  // ── Unified layer toggle panel ────────────────────────────────────────────
  const LayerPanel = L.Control.extend({
    onAdd() {
      const panel = L.DomUtil.create('div', 'layer-panel');
      panel.innerHTML = '<div class="layer-panel-title">LAYERS</div>';

      for (const def of LAYER_DEFS) {
        const row = L.DomUtil.create('div', 'layer-row', panel);
        const btn = L.DomUtil.create('button', 'layer-btn', row);
        btn.dataset.key = def.key;
        btn.innerHTML = `<span class="layer-dot" style="background:${def.color}"></span>${def.label}`;
        btn.title = `Toggle ${def.label}`;

        L.DomEvent.on(btn, 'click', L.DomEvent.stopPropagation)
                  .on(btn, 'click', () => {
                    const active = map.hasLayer(layerGroups[def.key]);
                    if (active) {
                      map.removeLayer(layerGroups[def.key]);
                      btn.classList.remove('active');
                    } else {
                      layerGroups[def.key].addTo(map);
                      btn.classList.add('active');
                    }
                  });
      }
      return panel;
    },
    onRemove() {},
  });
  new LayerPanel({ position: 'topright' }).addTo(map);
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function mileForLatLng(lat, lng) {
  const cosLat = Math.cos(lat * Math.PI / 180);
  let best = null, bestDist = Infinity;
  for (const w of ROUTE_WAYPOINTS) {
    const dlat = w.lat - lat;
    const dlng = (w.lng - lng) * cosLat;
    const d = dlat * dlat + dlng * dlng;
    if (d < bestDist) { bestDist = d; best = w; }
  }
  return best ? best.mile : 0;
}

function addTextMarker(wp, label) {
  L.marker([wp.lat, wp.lng], {
    icon: L.divIcon({
      html: `<div style="
        font-family:'Rajdhani',sans-serif;font-size:11px;font-weight:700;
        color:#003399;letter-spacing:.08em;white-space:nowrap;
        text-shadow:1px 1px 0 rgba(255,255,255,0.8);pointer-events:none;">${label}</div>`,
      className: '',
      iconSize: [60, 16],
      iconAnchor: [0, 8],
    }),
  }).addTo(map);
}

function updateMapRider(mile, town, state, checkinLat, checkinLng) {
  const [lat, lng] = (checkinLat != null && checkinLng != null)
    ? [checkinLat, checkinLng]
    : latLngForMile(mile);

  if (riderMarker) {
    riderMarker.setLatLng([lat, lng]);
  } else {
    riderMarker = L.marker([lat, lng], {
      icon: L.divIcon({ className: 'rider-marker', iconSize: [16, 16], iconAnchor: [8, 8] }),
      zIndexOffset: 1000,
    }).addTo(map);
  }

  riderMarker.bindTooltip(
    `<strong>${town || 'En route'}</strong><br>Mile ${Math.round(mile)}`,
    { direction: 'top' }
  );

  // Tint completed segment (always trace the route, end at interpolated mile position)
  if (window._completedLine) window._completedLine.remove();
  const completedPts = [];
  for (const w of ROUTE_WAYPOINTS) {
    if (w.mile <= mile) completedPts.push([w.lat, w.lng]);
  }
  completedPts.push(latLngForMile(mile));
  if (completedPts.length >= 2) {
    window._completedLine = L.polyline(completedPts, {
      color: '#0055cc',
      weight: 4,
      opacity: 0.9,
    }).addTo(map);
  }
}

// ── Elevation chart ───────────────────────────────────────────────────────────
const startEndMarkerPlugin = {
  id: 'startEndMarkers',
  afterDraw(chart) {
    const { ctx, chartArea: { left, right, top } } = chart;
    const data = chart.data.datasets[0].data;
    if (!data.length || !chart.scales.y) return;

    const col = '#7abcd8';
    const arrowSize = 5;
    const labelY = top + 13;

    ctx.save();
    ctx.font = "bold 9px 'Share Tech Mono', monospace";
    ctx.fillStyle = col;
    ctx.strokeStyle = col;
    ctx.lineWidth = 1;
    ctx.textBaseline = 'middle';

    function drawArrowTo(x1, y1, x2, y2) {
      const angle = Math.atan2(y2 - y1, x2 - x1);
      const gap = 8;
      x2 -= gap * Math.cos(angle);
      y2 -= gap * Math.sin(angle);
      ctx.beginPath(); ctx.moveTo(x1, y1); ctx.lineTo(x2, y2); ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(x2, y2);
      ctx.lineTo(x2 - arrowSize * Math.cos(angle - Math.PI / 6), y2 - arrowSize * Math.sin(angle - Math.PI / 6));
      ctx.lineTo(x2 - arrowSize * Math.cos(angle + Math.PI / 6), y2 - arrowSize * Math.sin(angle + Math.PI / 6));
      ctx.closePath(); ctx.fill();
    }

    // END label → leftmost curve point (Astoria)
    const endCurveY = chart.scales.y.getPixelForValue(data[0]);
    ctx.textAlign = 'left';
    const endW = ctx.measureText('END').width;
    ctx.fillText('END', left + 6, labelY);
    drawArrowTo(left + 6 + endW / 2, labelY + 8, left, endCurveY);

    // START label → rightmost curve point (Yorktown)
    const startCurveY = chart.scales.y.getPixelForValue(data[data.length - 1]);
    ctx.textAlign = 'right';
    const startW = ctx.measureText('START').width;
    ctx.fillText('START', right - 6, labelY);
    drawArrowTo(right - 6 - startW / 2, labelY + 8, right, startCurveY);

    // LATEST CHECKIN label → rider dot
    if (riderXLabel !== null && riderElevFt !== null && chart.scales.x) {
      const riderPixelX = chart.scales.x.getPixelForValue(riderXLabel);
      const riderPixelY = chart.scales.y.getPixelForValue(riderElevFt);
      const riderCol = '#00ff9d';
      ctx.fillStyle = riderCol;
      ctx.strokeStyle = riderCol;
      ctx.textAlign = 'center';
      const label = 'LATEST CHECKIN';
      const labelW = ctx.measureText(label).width;
      // Clamp label x so it stays within the chart area
      const clampedX = Math.min(Math.max(riderPixelX, left + labelW / 2 + 4), right - labelW / 2 - 4);
      ctx.fillText(label, clampedX, labelY);
      drawArrowTo(clampedX, labelY + 8, riderPixelX, riderPixelY);
    }

    ctx.restore();
  },
};

function initElevChart() {
  const ctx = document.getElementById('elevation-chart-canvas').getContext('2d');
  elevChart = new Chart(ctx, {
    type: 'line',
    plugins: [startEndMarkerPlugin],
    data: { labels: [], datasets: [{
      data: [],
      borderColor: '#00d4ff',
      borderWidth: 1.5,
      backgroundColor: 'rgba(0,212,255,0.08)',
      fill: true,
      tension: 0.4,
      pointRadius: 0,
    }]},
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      plugins: { legend: { display: false }, tooltip: {
        callbacks: {
          label: ctx => `${ctx.parsed.y.toLocaleString()} ft`,
          title: ctx => ctx[0].label,
        },
        backgroundColor: 'rgba(4,14,28,0.95)',
        borderColor: 'rgba(0,200,255,0.3)',
        borderWidth: 1,
        titleColor: '#00d4ff',
        bodyColor: '#c8e8f8',
      }},
      scales: {
        x: {
          ticks: { color: '#4a7090', font: { size: 10 }, maxTicksLimit: 10 },
          grid:  { color: 'rgba(0,100,160,0.12)' },
        },
        y: {
          ticks: {
            color: '#4a7090',
            font: { size: 10 },
            callback: v => v >= 1000 ? `${(v/1000).toFixed(0)}k` : v,
          },
          grid:  { color: 'rgba(0,100,160,0.12)' },
        },
      },
    },
  });
}

function updateElevChart(checkins, latest, derivedMile) {
  // Prefer the full GPS-derived elevation profile; fall back to check-in points
  if (typeof ROUTE_ELEVATION !== 'undefined' && ROUTE_ELEVATION.length > 0) {
    // Reverse so west (Astoria) is on the left, matching the map's geography
    const elev = ROUTE_ELEVATION.slice().reverse();
    elevChart.data.labels            = elev.map(p => `${p.mile.toFixed(0)} mi`);
    elevChart.data.datasets[0].data  = elev.map(p => p.ele);

    // START (Yorktown) = rightmost label; END (Astoria) = leftmost label
    const startLabel = elevChart.data.labels[elev.length - 1];
    const endLabel   = elevChart.data.labels[0];
    const markerLine = { type: 'line', scaleID: 'x', borderWidth: 1, borderDash: [3, 3], borderColor: 'rgba(120,180,220,0.35)' };
    const annotations = {
      startLine: { ...markerLine, value: startLabel },
      endLine:   { ...markerLine, value: endLabel   },
    };

    // Overlay a dot at the rider's current position
    riderXLabel = null; riderElevFt = null;
    if (latest && derivedMile) {
      // Find the label whose mile value is closest to derivedMile
      let closestIdx = 0, minDist = Infinity;
      for (let i = 0; i < elev.length; i++) {
        const d = Math.abs(elev[i].mile - derivedMile);
        if (d < minDist) { minDist = d; closestIdx = i; }
      }
      const xLabel = elevChart.data.labels[closestIdx];
      const elevFt = latest.elevation_ft != null
        ? latest.elevation_ft
        : elev[closestIdx].ele;  // fall back to route elevation (already in feet)
      riderXLabel = xLabel;
      riderElevFt = elevFt;
    }
    elevChart.options.plugins.annotation = { annotations };
  } else {
    // Fall back: plot elevation at each check-in, chronological (oldest first)
    const pts = checkins.filter(c => c.elevation_ft != null).slice().reverse();
    if (!pts.length) return;
    elevChart.data.labels            = pts.map(c => fmtDate(c.created_at));
    elevChart.data.datasets[0].data  = pts.map(c => c.elevation_ft);
  }
  elevChart.update();
}

// ── Data fetch & render ───────────────────────────────────────────────────────
async function fetchAndRender() {
  try {
    const [statusRes, checkinsRes] = await Promise.all([
      fetch(`${API}/checkins/latest`),
      fetch(`${API}/checkins`),
    ]);
    if (!statusRes.ok || !checkinsRes.ok) throw new Error('API error');
    const { latest, stats, config } = await statusRes.json();
    const checkins = await checkinsRes.json();

    const derivedMile = (latest && latest.lat != null)
      ? mileForLatLng(latest.lat, latest.lng)
      : 0;

    const riderEl = document.getElementById('rider-name');
    if (riderEl) riderEl.textContent = config.rider_name || 'Rider';
    renderHeader(latest, derivedMile, config);
    renderProgress(derivedMile);
    renderTelemetry(latest, derivedMile, stats);
    renderWeather(latest);
    renderCumulative(stats);
    updateElevChart(checkins, latest, derivedMile);
    renderLog(checkins);
    if (latest) updateMapRider(derivedMile, latest.town, latest.state, latest.lat, latest.lng);
    document.getElementById('last-updated').textContent =
      'Updated ' + new Date().toLocaleTimeString();
  } catch (e) {
    console.error('Fetch error:', e);
  }
}

function renderHeader(latest, derivedMile, config) {
  // Status pill
  const dot  = document.querySelector('.status-dot');
  const pill = document.querySelector('.status-pill span');
  if (!latest) {
    dot.className = 'status-dot offline';
    pill.textContent = 'NOT STARTED';
  } else if (latest.is_rest_day) {
    dot.className = 'status-dot rest';
    pill.textContent = 'REST DAY';
  } else {
    dot.className = 'status-dot';
    pill.textContent = 'EN ROUTE';
  }

  // Elapsed days
  const startDate = config.start_date ? new Date(config.start_date) : null;
  const elapsed = startDate
    ? Math.floor((Date.now() - startDate) / 86400000)
    : null;
  const dayEl = document.getElementById('hdr-day');
  if (dayEl) dayEl.textContent = elapsed != null ? `${elapsed}d` : '—';
  const msElapsed = document.getElementById('ms-elapsed');
  if (msElapsed) msElapsed.textContent = elapsed != null ? `${elapsed}d` : '—';

  // Location
  const locEl = document.getElementById('tel-location');
  if (locEl && latest) {
    const townState = latest.town ? `${latest.town}, ${latest.state}` : latest.state || null;
    locEl.textContent = latest.name
      ? (townState ? `${latest.name} · ${townState}` : latest.name)
      : (townState || '—');
  }

  // % complete
  const pct = ((derivedMile / ROUTE_TOTAL_MILES) * 100).toFixed(1);
  const pctEl = document.getElementById('hdr-pct');
  if (pctEl) pctEl.textContent = `${pct}%`;
  const msPct = document.getElementById('ms-pct');
  if (msPct) msPct.textContent = `${pct}%`;
}

function renderProgress(derivedMile) {
  const pct = +((derivedMile / ROUTE_TOTAL_MILES) * 100).toFixed(1);
  document.getElementById('progress-fill').style.width = `${pct}%`;
  const marker = document.getElementById('progress-marker');
  marker.style.transition = 'none';
  marker.style.left = '100%';
  requestAnimationFrame(() => {
    marker.style.transition = '';
    marker.style.left = `${100 - pct}%`;
  });
  const milesLabel = Math.round(derivedMile).toLocaleString();
  document.getElementById('progress-miles').textContent =
    `${milesLabel} / ${ROUTE_TOTAL_MILES.toLocaleString()} mi`;
  const msMiles = document.getElementById('ms-miles');
  if (msMiles) msMiles.textContent = `${milesLabel} mi`;
}

function renderTelemetry(latest, derivedMile, stats) {
  if (!latest) return;

  const section = currentSection(derivedMile);
  document.getElementById('map-location-name').textContent =
    latest.town ? `${latest.town}, ${latest.state}` : (latest.state || '—');
  document.getElementById('map-section-name').textContent =
    section ? `Section: ${section}` : '';

  set('tel-mile',    Math.round(derivedMile).toLocaleString());
  set('tel-elev',    latest.elevation_ft != null ? latest.elevation_ft.toLocaleString() : '—');
  set('tel-today',   latest.miles_today != null ? latest.miles_today.toFixed(1) : '—');
  set('tel-speed',   latest.avg_speed_today != null ? latest.avg_speed_today.toFixed(1) : '—');
  set('tel-gain',    latest.elevation_gain_today != null ? `+${latest.elevation_gain_today.toLocaleString()}` : '—');
  set('tel-loss',    latest.elevation_loss_today != null ? `-${latest.elevation_loss_today.toLocaleString()}` : '—');
  set('tel-time',    latest.moving_time_minutes != null ? fmtDuration(latest.moving_time_minutes) : '—');
  set('tel-lodging', fmtLodging(latest.lodging_type));
  const remainMi = Math.round(ROUTE_TOTAL_MILES - derivedMile).toLocaleString();
  set('tel-remain', remainMi);
  const msRemaining = document.getElementById('ms-remaining');
  if (msRemaining) msRemaining.textContent = `${remainMi} mi`;
}

function renderWeather(latest) {
  if (!latest || latest.weather_temp_f == null) return;
  document.getElementById('wx-temp').textContent = `${Math.round(latest.weather_temp_f)}°`;
  document.getElementById('wx-condition').textContent = latest.weather_condition || '';
  document.getElementById('wx-wind').textContent =
    latest.weather_wind_mph != null
      ? `Wind ${Math.round(latest.weather_wind_mph)} mph ${latest.weather_wind_dir || ''}`
      : '';
}

function renderCumulative(stats) {
  set('cum-total',   stats.totalMiles.toLocaleString());
  set('cum-climb',   stats.totalClimbing.toLocaleString());
  set('cum-days',    stats.ridingDays);
  set('cum-rest',    stats.restDays);
  set('cum-avg',     stats.avgMilesPerRidingDay);
  set('cum-longest', stats.longestDay);
  set('cum-camped',  stats.nightsCamped);
  set('cum-indoor',  stats.nightsIndoor);
}

function renderLog(checkins) {
  const wrap = document.getElementById('log-entries');
  if (!checkins.length) {
    wrap.innerHTML = '<p class="log-empty">No check-ins yet. Trip hasn\'t started!</p>';
    return;
  }
  wrap.innerHTML = checkins.slice(0, 5).map(c => renderLogEntry(c)).join('');
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function set(id, val) {
  const el = document.getElementById(id);
  if (el) el.textContent = val ?? '—';
}
