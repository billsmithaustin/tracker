// ── Shared helpers ────────────────────────────────────────────────────────────

function fmtDate(iso) {
  if (!iso) return '';
  // Bare YYYY-MM-DD is parsed as UTC midnight; add a noon offset to avoid
  // the date shifting backwards in negative-UTC-offset timezones.
  const d = iso.length === 10 ? new Date(iso + 'T12:00:00') : new Date(iso);
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function fmtDuration(minutes) {
  if (minutes == null) return '—';
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

function fmtLodging(type) {
  const map = {
    camping: '⛺ Camping', hotel: '🏨 Hotel', motel: '🏨 Motel',
    warmshowers: '🏠 WarmShowers', bnb: '🛏 B&B', other: 'Other',
  };
  return type ? (map[type] || type) : null;
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;')
    .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ── Shared log entry renderer ─────────────────────────────────────────────────

function renderLogEntry(c, deleteBtn = '') {
  const badges = [];
  if (c.is_rest_day)    badges.push('<span class="log-badge rest">Rest Day</span>');
  if (c.lodging_type === 'camping') badges.push('<span class="log-badge camp">⛺ Camp</span>');
  else if (c.lodging_type)          badges.push(`<span class="log-badge">${fmtLodging(c.lodging_type)}</span>`);

  const stats = [];
  if (c.miles_today)          stats.push(`<span><span class="hi">${c.miles_today.toFixed(1)}</span> mi</span>`);
  if (c.elevation_gain_today) stats.push(`<span><span class="hi">+${c.elevation_gain_today.toLocaleString()}</span> ft gain</span>`);
  if (c.elevation_ft)         stats.push(`<span><span class="hi">${c.elevation_ft.toLocaleString()}</span> ft elev</span>`);
  if (c.avg_speed_today)      stats.push(`<span><span class="hi">${c.avg_speed_today.toFixed(1)}</span> mph avg</span>`);
  if (c.moving_time_minutes)  stats.push(`<span><span class="hi">${fmtDuration(c.moving_time_minutes)}</span> riding</span>`);

  const townState = c.town ? `${c.town}${c.state ? ', ' + c.state : ''}` : (c.state || '');
  const location = c.name ? (townState ? `${c.name} · ${townState}` : c.name) : (townState || '—');

  return `
    <div class="log-entry">
      <div class="log-date">${fmtDate(c.date || c.created_at)}</div>
      <div class="log-body">
        <div class="log-header">
          <span class="log-location">${escHtml(location)}</span>
          ${badges.join('')}
          ${deleteBtn}
        </div>
        ${stats.length ? `<div class="log-stats">${stats.join('')}</div>` : ''}
        ${c.note ? `<div class="log-note">${escHtml(c.note)}</div>` : ''}
        ${c.photo_url ? `<a href="${escHtml(c.photo_url)}" target="_blank" rel="noopener"><img class="log-photo" src="${escHtml(c.photo_url)}" alt="Day photo" loading="lazy"></a>` : ''}
      </div>
    </div>`;
}
