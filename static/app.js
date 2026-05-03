import { html, render, useState, useMemo, useCallback, useEffect, useRef } from './htm-preact-standalone.js';

// --- Constants ---
var STATUS_MESSAGES = {
  pending: '準備中...',
  scraping: '戦績を取得中...（数分かかります）',
  analyzing: '分析中...',
  done: '完了',
  error: 'エラーが発生しました',
};

var PERIOD_KEYS = ['all', '90d', '60d', '30d', '14d', '7d', '3d', '1d'];

// --- Utility ---
function esc(s) {
  if (s == null) return '';
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function pct(n) { return n != null ? n.toFixed(1) + '%' : '-'; }
function num(n, d) { return n != null ? n.toFixed(d != null ? d : 0) : '-'; }

// --- Share helpers ---
var SVG_X = '<svg viewBox="0 0 24 24"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>';
var SVG_BSKY = '<svg viewBox="0 0 568 501"><path d="M123.121 33.664C188.241 82.553 258.281 181.68 284 234.873c25.719-53.192 95.759-152.32 160.879-201.21C491.866-1.611 568-28.906 568 57.947c0 17.346-9.945 145.713-15.778 166.555-20.275 72.453-94.155 90.933-159.875 79.748C507.222 323.8 536.444 388.56 473.333 453.32c-119.86 122.992-172.272-30.859-185.702-70.281-2.462-7.227-3.614-10.608-3.631-7.733-.017-2.875-1.169.506-3.631 7.733-13.43 39.422-65.842 193.273-185.702 70.281-63.111-64.76-33.889-129.52 80.986-149.071-65.72 11.185-139.6-7.295-159.875-79.748C9.945 203.659 0 75.291 0 57.946 0-28.906 76.135-1.612 123.121 33.664z"/></svg>';
var SVG_LINE = '<svg viewBox="0 0 24 24"><path d="M19.365 9.863c.349 0 .63.285.63.631 0 .345-.281.63-.63.63H17.61v1.125h1.755c.349 0 .63.283.63.63 0 .344-.281.629-.63.629h-2.386c-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63h2.386c.346 0 .627.285.627.63 0 .349-.281.63-.63.63H17.61v1.125h1.755zm-3.855 3.016c0 .27-.174.51-.432.596-.064.021-.133.031-.199.031-.211 0-.391-.09-.51-.25l-2.443-3.317v2.94c0 .344-.279.629-.631.629-.346 0-.626-.285-.626-.629V8.108c0-.27.173-.51.43-.595.06-.023.136-.033.194-.033.195 0 .375.104.495.254l2.462 3.33V8.108c0-.345.282-.63.63-.63.345 0 .63.285.63.63v4.771zm-5.741 0c0 .344-.282.629-.631.629-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63.346 0 .628.285.628.63v4.771zm-2.466.629H4.917c-.345 0-.63-.285-.63-.629V8.108c0-.345.285-.63.63-.63.348 0 .63.285.63.63v4.141h1.756c.348 0 .629.283.629.63 0 .344-.282.629-.629.629M24 10.314C24 4.943 18.615.572 12 .572S0 4.943 0 10.314c0 4.811 4.27 8.842 10.035 9.608.391.082.923.258 1.058.59.12.301.079.766.038 1.08l-.164 1.02c-.045.301-.24 1.186 1.049.645 1.291-.539 6.916-4.078 9.436-6.975C23.176 14.393 24 12.458 24 10.314"/></svg>';
var SVG_COPY = '<svg viewBox="0 0 24 24"><path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/></svg>';
var SVG_CHECK = '<svg viewBox="0 0 24 24"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>';

function buildShareText(items) {
  var lines = ['【EXVS2IB 戦績診断】'];
  for (var i = 0; i < items.length; i++) {
    var item = items[i];
    if (item.type === 'top_ms') {
      lines.push('🤖 最多使用: ' + item.ms + '（' + item.count + '戦）');
    } else if (item.type === 'strong_enemy') {
      lines.push('💪 ' + item.enemy + '相手に勝率' + item.wr + '%！');
    } else if (item.type === 'weak_enemy') {
      lines.push('😈 ' + item.enemy + 'に勝率' + item.wr + '%...天敵かも');
    } else if (item.type === 'dmg_efficiency') {
      var desc = item.value >= 1.0 ? '与ダメが上回ってます' : '被ダメが上回ってます';
      lines.push('⚔ ' + item.ms + 'の与被ダメ比: ' + item.value + '（' + desc + '）');
    }
  }
  lines.push('');
  lines.push('▶ 自分も診断してみる');
  lines.push(location.origin);
  return lines.join('\n');
}

// --- Generic components ---

function Tips({ tips }) {
  if (!tips || !tips.length) return null;
  return html`<blockquote><strong>💡アドバイス:</strong><br />${tips.map(function (t, i) {
    return html`${i > 0 && html`<br />`}${t}`;
  })}</blockquote>`;
}

function SortableTable({ headers, rows, sortableColumns, defaultLimit }) {
  if (!rows || !rows.length) return null;
  var sortRef = useState({ col: -1, asc: true });
  var sortState = sortRef[0], setSortState = sortRef[1];
  var limitRef = useState(defaultLimit || 0);
  var limit = limitRef[0], setLimit = limitRef[1];

  var sortedRows = useMemo(function () {
    if (sortState.col < 0) return rows;
    var col = sortState.col;
    var sorted = rows.slice().sort(function (a, b) {
      var va = a[col], vb = b[col];
      // 数値文字列からパース（%や+を除去）
      var na = parseFloat(String(va).replace(/[%+戦件回]/g, ''));
      var nb = parseFloat(String(vb).replace(/[%+戦件回]/g, ''));
      if (!isNaN(na) && !isNaN(nb)) {
        return sortState.asc ? na - nb : nb - na;
      }
      var sa = String(va), sb = String(vb);
      return sortState.asc ? sa.localeCompare(sb) : sb.localeCompare(sa);
    });
    return sorted;
  }, [rows, sortState]);

  var expanded = limit === 0;
  var displayRows = limit > 0 ? sortedRows.slice(0, limit) : sortedRows;
  var hasMore = limit > 0 && sortedRows.length > limit;

  function handleSort(colIdx) {
    if (sortState.col === colIdx) {
      setSortState({ col: colIdx, asc: !sortState.asc });
    } else {
      setSortState({ col: colIdx, asc: false });
    }
  }

  var sortable = sortableColumns || [];

  return html`<div>
    <div class="table-wrap"><table>
      <thead><tr>${headers.map(function (h, i) {
        var isSortable = h !== '' && (sortable.length === 0 || sortable.indexOf(i) >= 0);
        var indicator = sortState.col === i ? (sortState.asc ? ' ▲' : ' ▼') : (isSortable ? ' △' : '');
        return html`<th class=${isSortable ? 'sortable' : ''} onClick=${isSortable ? function () { handleSort(i); } : undefined}>${h}${indicator}</th>`;
      })}</tr></thead>
      <tbody>${displayRows.map(function (row) {
        return html`<tr>${row.map(function (cell) { return html`<td>${cell}</td>`; })}</tr>`;
      })}</tbody>
    </table></div>
    ${defaultLimit > 0 && sortedRows.length > defaultLimit && html`<div class="show-more-wrap">
      ${hasMore
        ? html`<button class="show-more-btn" onClick=${function () { setLimit(0); }}>もっと見る (+${sortedRows.length - limit}件)</button>`
        : html`<button class="show-more-btn" onClick=${function () { setLimit(defaultLimit); }}>折りたたむ</button>`}
    </div>`}
  </div>`;
}

function Table({ headers, rows }) {
  if (!rows || !rows.length) return null;
  return html`<${SortableTable} headers=${headers} rows=${rows} />`;
}

function Section({ title, open, children }) {
  return html`<details ...${{ open: open || false }}>
    <summary><strong>${title}</strong></summary>
    ${children}
  </details><hr />`;
}

function SubSection({ title, open, children }) {
  return html`<details ...${{ open: open || false }}>
    <summary>${title}</summary>
    ${children}
  </details>`;
}

// --- Calendar component ---

var DOW_LABELS = ['日', '月', '火', '水', '木', '金', '土'];

function CalendarPicker({ selectedDate, onSelect }) {
  var now = selectedDate ? new Date(selectedDate) : new Date();
  var viewRef = useState({ year: now.getFullYear(), month: now.getMonth() });
  var view = viewRef[0], setView = viewRef[1];

  function prevMonth() {
    setView(function (v) {
      var m = v.month - 1;
      return m < 0 ? { year: v.year - 1, month: 11 } : { year: v.year, month: m };
    });
  }
  function nextMonth() {
    setView(function (v) {
      var m = v.month + 1;
      return m > 11 ? { year: v.year + 1, month: 0 } : { year: v.year, month: m };
    });
  }

  var firstDay = new Date(view.year, view.month, 1).getDay();
  var daysInMonth = new Date(view.year, view.month + 1, 0).getDate();

  var cells = [];
  for (var i = 0; i < firstDay; i++) cells.push(null);
  for (var d = 1; d <= daysInMonth; d++) cells.push(d);
  while (cells.length < 42) cells.push(null);

  var selStr = selectedDate || '';

  function isSelected(day) {
    if (!day || !selStr) return false;
    var m = String(view.month + 1).padStart(2, '0');
    var dd = String(day).padStart(2, '0');
    return selStr === view.year + '-' + m + '-' + dd;
  }

  function handleClick(day) {
    if (!day) return;
    var m = String(view.month + 1).padStart(2, '0');
    var dd = String(day).padStart(2, '0');
    onSelect(view.year + '-' + m + '-' + dd);
  }

  return html`<div class="cal">
    <div class="cal-header">
      <button class="cal-nav" onClick=${prevMonth}>\u25C0</button>
      <span class="cal-title">${view.year}年${view.month + 1}月</span>
      <button class="cal-nav" onClick=${nextMonth}>\u25B6</button>
    </div>
    <div class="cal-grid">
      ${DOW_LABELS.map(function (d) { return html`<span class="cal-dow">${d}</span>`; })}
      ${cells.map(function (day) {
        if (!day) return html`<span class="cal-empty" />`;
        return html`<button class=${'cal-day' + (isSelected(day) ? ' selected' : '')}
          onClick=${function () { handleClick(day); }}>${day}</button>`;
      })}
    </div>
  </div>`;
}

// --- Time selector ---

var MINUTES_START = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55];
var MINUTES_END = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 59];

function TimeSelector({ hour, minute, onChangeHour, onChangeMinute, isEnd }) {
  var hours = [];
  for (var h = 0; h < 24; h++) hours.push(h);
  var minutes = isEnd ? MINUTES_END : MINUTES_START;

  return html`<div class="time-sel">
    <select class="time-select" value=${hour} onChange=${function (e) { onChangeHour(parseInt(e.target.value)); }}>
      ${hours.map(function (h) { return html`<option value=${h}>${String(h).padStart(2, '0')}時</option>`; })}
    </select>
    <span class="time-colon">:</span>
    <select class="time-select" value=${minute} onChange=${function (e) { onChangeMinute(parseInt(e.target.value)); }}>
      ${minutes.map(function (m) { return html`<option value=${m}>${String(m).padStart(2, '0')}分</option>`; })}
    </select>
  </div>`;
}

// --- Period selector (GCP/AWS style dropdown) ---

function PeriodSelector({ periods, selected, onSelect, userKey, onCustomReport }) {
  var keys = PERIOD_KEYS.filter(function (k) { return periods[k]; });
  if (keys.length <= 1 && !userKey) return null;

  var openRef = useState(false);
  var isOpen = openRef[0], setIsOpen = openRef[1];
  var customRef = useState(false);
  var showCustom = customRef[0], setShowCustom = customRef[1];
  var loadingRef = useState(false);
  var isLoading = loadingRef[0], setIsLoading = loadingRef[1];
  var errorRef = useState('');
  var customError = errorRef[0], setCustomError = errorRef[1];

  // カスタム日時の状態（日付文字列 + 時/分）
  var startDateRef = useState('');
  var startDate = startDateRef[0], setStartDate = startDateRef[1];
  var startHourRef = useState(0);
  var startHour = startHourRef[0], setStartHour = startHourRef[1];
  var startMinRef = useState(0);
  var startMin = startMinRef[0], setStartMin = startMinRef[1];
  var endDateRef = useState('');
  var endDate = endDateRef[0], setEndDate = endDateRef[1];
  var endHourRef = useState(23);
  var endHour = endHourRef[0], setEndHour = endHourRef[1];
  var endMinRef = useState(59);
  var endMin = endMinRef[0], setEndMin = endMinRef[1];
  var timeRef = useState(false);
  var showTime = timeRef[0], setShowTime = timeRef[1];

  var containerRef = useRef(null);

  useEffect(function () {
    function handleClick(e) {
      if (containerRef.current && !containerRef.current.contains(e.target)) {
        setIsOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return function () { document.removeEventListener('mousedown', handleClick); };
  }, []);

  // スマホでドロップダウン表示中はbodyスクロールを止める
  useEffect(function () {
    if (isOpen && window.innerWidth <= 600) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return function () { document.body.style.overflow = ''; };
  }, [isOpen]);

  var currentLabel = selected === 'custom'
    ? (periods.custom ? periods.custom.label : '日付指定')
    : (periods[selected] ? periods[selected].label : '全データ');

  function selectPreset(k) {
    onSelect(k);
    setIsOpen(false);
    setShowCustom(false);
  }

  function formatDt(date, hour, min) {
    return date + ' ' + String(hour).padStart(2, '0') + ':' + String(min).padStart(2, '0');
  }

  function handleCustomApply() {
    if (!startDate || !endDate) {
      setCustomError('開始日と終了日をカレンダーから選択してください');
      return;
    }
    var start = showTime ? formatDt(startDate, startHour, startMin) : startDate + ' 00:00';
    var end = showTime ? formatDt(endDate, endHour, endMin) : endDate + ' 23:59';
    setIsLoading(true);
    setCustomError('');
    fetch('/period?user_key=' + encodeURIComponent(userKey) + '&start=' + encodeURIComponent(start) + '&end=' + encodeURIComponent(end))
      .then(function (res) { return res.json(); })
      .then(function (data) {
        setIsLoading(false);
        if (data.error) {
          setCustomError(data.error);
          return;
        }
        onCustomReport(data.report);
        setIsOpen(false);
      })
      .catch(function (e) {
        setIsLoading(false);
        setCustomError(e.message);
      });
  }

  return html`<div class="period-selector" ref=${containerRef}>
    <button class="period-trigger" onClick=${function () { setIsOpen(!isOpen); }}>
      ${currentLabel} <span class="period-arrow">${isOpen ? '\u25B2' : '\u25BC'}</span>
    </button>
    ${isOpen && html`<div class="period-backdrop" onClick=${function () { setIsOpen(false); }} />`}
    ${isOpen && html`<div class="period-dropdown">
      <div class="period-dropdown-list">
        ${keys.map(function (k) {
          return html`<button class=${'period-dropdown-item' + (selected === k ? ' active' : '')}
            onClick=${function () { selectPreset(k); }}>${periods[k].label}</button>`;
        })}
        ${userKey && html`<button class=${'period-dropdown-item period-dropdown-custom' + (showCustom ? ' active' : '')}
          onClick=${function () { setShowCustom(!showCustom); }}>日付指定</button>`}
      </div>
      ${showCustom && html`<div class="period-custom">
        <div class="period-custom-range">
          <div class="period-custom-col">
            <span class="period-custom-title">開始</span>
            <span class="period-custom-value">${startDate || '日付を選択'}${showTime ? ' ' + String(startHour).padStart(2, '0') + ':' + String(startMin).padStart(2, '0') : ''}</span>
            <${CalendarPicker} selectedDate=${startDate} onSelect=${setStartDate} />
            ${showTime && html`<${TimeSelector} hour=${startHour} minute=${startMin}
              onChangeHour=${setStartHour} onChangeMinute=${setStartMin} />`}
          </div>
          <div class="period-custom-col">
            <span class="period-custom-title">終了</span>
            <span class="period-custom-value">${endDate || '日付を選択'}${showTime ? ' ' + String(endHour).padStart(2, '0') + ':' + String(endMin).padStart(2, '0') : ''}</span>
            <${CalendarPicker} selectedDate=${endDate} onSelect=${setEndDate} />
            ${showTime && html`<${TimeSelector} hour=${endHour} minute=${endMin}
              onChangeHour=${setEndHour} onChangeMinute=${setEndMin} isEnd />`}
          </div>
        </div>
        <button class="period-time-toggle" onClick=${function () { setShowTime(!showTime); }}>
          ${showTime ? '時刻指定を解除' : '時刻を指定'}</button>
        <button class="period-custom-apply" onClick=${handleCustomApply} disabled=${isLoading}>
          ${isLoading ? '分析中...' : '適用'}</button>
        ${customError && html`<p class="period-custom-error">${customError}</p>`}
      </div>`}
    </div>`}
  </div>`;
}

// --- Report sections ---

function SummarySection({ summary }) {
  if (!summary || !summary.categories || !summary.categories.length) return null;
  return html`<${Section} title="総合アドバイス" open>
    ${summary.categories.map(function (cat) {
      return html`<div>
        <strong>${esc(cat.title)}</strong>
        <ul>${cat.items.map(function (item) { return html`<li>${item}</li>`; })}</ul>
      </div>`;
    })}
  <//>`;
}

function BasicStatsSection({ stats }) {
  if (!stats) return null;
  var rows = [
    ['試合数', stats.matches + '戦 (' + stats.wins + '勝' + stats.losses + '敗)'],
    ['勝率', pct(stats.win_rate)],
    ['平均与ダメージ', num(stats.avg_dmg_given)],
    ['平均被ダメージ', num(stats.avg_dmg_taken)],
    ['与被ダメ比', num(stats.dmg_efficiency, 3)],
    ['平均撃墜', num(stats.avg_kills, 2)],
    ['平均被撃墜', num(stats.avg_deaths, 2)],
    ['K/D比', num(stats.kd_ratio, 2)],
    ['平均EXダメージ', num(stats.avg_ex_dmg)],
  ];
  return html`<div>
    <${Table} headers=${['項目', '値']} rows=${rows} />
    <${Tips} tips=${stats.tips} />
  </div>`;
}

function WinLossPatternSection({ pattern }) {
  if (!pattern) return null;
  var rows = (pattern.metrics || []).map(function (m) {
    var diff = m.diff >= 0 ? '+' + num(m.diff, 1) : num(m.diff, 1);
    return [m.label, num(m.win_avg, 1), num(m.loss_avg, 1), diff];
  });
  return html`<div>
    <${Table} headers=${['項目', '勝利時', '敗北時', '差分']} rows=${rows} />
    <${Tips} tips=${pattern.tips} />
  </div>`;
}

function EnemyMatchupSection({ matchup, msName }) {
  if (!matchup) return null;
  var headers = ['機体名', '試合', '勝率', '与被ダメ比', '与ダメ', '被ダメ'];
  function matchupRows(list) {
    return (list || []).map(function (e) {
      return [esc(e.ms), e.matches, pct(e.win_rate), num(e.dmg_efficiency, 3), num(e.avg_dmg_given, 1), num(e.avg_dmg_taken, 1)];
    });
  }
  return html`<div>
    ${matchup.strong && matchup.strong.length > 0 && html`<p><strong>得意な相手:</strong></p><${SortableTable} headers=${headers} rows=${matchupRows(matchup.strong)} defaultLimit=${5} />`}
    ${matchup.weak && matchup.weak.length > 0 && html`<p><strong>苦手な相手:</strong></p><${SortableTable} headers=${headers} rows=${matchupRows(matchup.weak)} defaultLimit=${5} />`}
    ${matchup.even && matchup.even.length > 0 && html`<p><strong>互角の相手:</strong></p><${SortableTable} headers=${headers} rows=${matchupRows(matchup.even)} defaultLimit=${5} />`}
    <${Tips} tips=${matchup.tips} />
  </div>`;
}

function PartnerSection({ partners, msName }) {
  if (!partners || !partners.length) return null;
  var rows = partners.map(function (p) {
    return [esc(p.ms), p.matches, pct(p.win_rate), num(p.dmg_efficiency, 3)];
  });
  return html`<div>
    <${SortableTable} headers=${['機体名', '試合', '勝率', '与被ダメ比']} rows=${rows} defaultLimit=${10} />
  </div>`;
}

function msAnchorId(msName, idx) {
  return 'sec-ms-' + idx;
}

function MsStatsSection({ msStats }) {
  if (!msStats) return null;
  var entries = Object.keys(msStats).sort(function (a, b) {
    return msStats[b].matches - msStats[a].matches;
  });
  if (!entries.length) return null;
  return entries.map(function (msName, idx) {
    var ms = msStats[msName];
    return html`<div id=${msAnchorId(msName, idx)}><${Section} title=${'機体別分析: ' + msName}>
      <${SubSection} title="基本データ" open>
        <${BasicStatsSection} stats=${ms.basic_stats} />
      <//>
      <${SubSection} title="被撃墜数と勝率">
        <${DeathsImpactSubSection} deaths=${ms.deaths_impact} />
      <//>
      <${SubSection} title="勝利時/敗北時のダメージ傾向">
        <${WinLossPatternSection} pattern=${ms.win_loss_pattern} />
      <//>
      <${SubSection} title="敵機体との相性">
        <${EnemyMatchupSection} matchup=${ms.enemy_matchup} msName=${msName} />
      <//>
      <${SubSection} title="相方機体との相性">
        <${PartnerSection} partners=${ms.partner} msName=${msName} />
      <//>
      <${SubSection} title="編成別勝率">
        <${MsPairSubSection} msPair=${ms.ms_pair} />
      <//>
      <${SubSection} title="コスト編成別勝率">
        <${CostPairSubSection} costPair=${ms.cost_pair} />
      <//>
      <${SubSection} title="ダメージ貢献率">
        <${DmgContributionSubSection} dmg=${ms.dmg_contribution} />
      <//>
    <//></div>`;
  });
}

function MsPairSubSection({ msPair }) {
  if (!msPair) return null;
  var list = msPair.by_matches || [];
  if (!list.length) return null;
  var rows = list.map(function (p) {
    return [esc(p.pair), p.matches, pct(p.win_rate), num(p.dmg_efficiency, 3)];
  });
  return html`<div>
    <${SortableTable} headers=${['編成', '試合数', '勝率', '与被ダメ比']} rows=${rows} defaultLimit=${10} />
  </div>`;
}

function CostPairSubSection({ costPair }) {
  if (!costPair || !costPair.length) return null;
  var rows = costPair.map(function (p) {
    return [esc(p.pair), p.matches, pct(p.win_rate), num(p.dmg_efficiency, 3)];
  });
  return html`<div>
    <${SortableTable} headers=${['コスト編成', '試合数', '勝率', '与被ダメ比']} rows=${rows} defaultLimit=${10} />
  </div>`;
}

function DmgContributionSubSection({ dmg }) {
  if (!dmg) return null;
  function diffPct(win, lose) {
    if (win == null || lose == null) return '-';
    var d = win - lose;
    var s = d >= 0 ? '+' : '';
    return s + d.toFixed(1) + '%';
  }
  var rows = [];
  (dmg.by_cost || []).forEach(function (c) {
    rows.push([c.matches, pct(c.avg_contribution), pct(c.avg_win_contribution), pct(c.avg_lose_contribution), diffPct(c.avg_win_contribution, c.avg_lose_contribution)]);
  });
  return html`<div>
    <${Table} headers=${['試合数', '平均貢献率', '勝利時', '敗北時', '差分']} rows=${rows} />
  </div>`;
}

function FixedPartnersSection({ partners }) {
  if (!partners) return null;
  var list = partners.partners || partners;
  if (Array.isArray(list) && !list.length) {
    if (partners.notice) {
      return html`<${Section} title="固定相方分析">
        <p class="notice">${esc(partners.notice)}</p>
      <//>`;
    }
    return null;
  }
  var items = Array.isArray(list) ? list : [];
  return html`<${Section} title="固定相方分析">
    ${partners.notice && html`<p class="notice">${esc(partners.notice)}</p>`}
    ${items.map(function (p) {
      var statsRows = [
        ['平均与ダメージ', num(p.my_stats.avg_dmg_given), num(p.partner_stats.avg_dmg_given)],
        ['平均被ダメージ', num(p.my_stats.avg_dmg_taken), num(p.partner_stats.avg_dmg_taken)],
        ['与被ダメ比', num(p.my_stats.dmg_efficiency, 3), num(p.partner_stats.dmg_efficiency, 3)],
        ['平均撃墜', num(p.my_stats.avg_kills, 2), num(p.partner_stats.avg_kills, 2)],
        ['平均被撃墜', num(p.my_stats.avg_deaths, 2), num(p.partner_stats.avg_deaths, 2)],
      ];
      var msRows = (p.partner_ms_breakdown || []).map(function (m) {
        return [esc(m.ms), m.matches, pct(m.win_rate)];
      });
      var title = p.team_name ? esc(p.partner_name) + '【' + esc(p.team_name) + '】' : esc(p.partner_name);
      return html`<div>
        <h3>${title} (${p.matches}戦)</h3>
        <p>${p.wins}勝${p.losses}敗 (勝率 ${pct(p.win_rate)})</p>
        <${Table} headers=${['項目', '自分', '相方']} rows=${statsRows} />
        ${msRows.length > 0 && html`<p><strong>相方の使用機体:</strong></p><${Table} headers=${['機体', '試合', '勝率']} rows=${msRows} />`}
        <${Tips} tips=${p.tips} />
      </div>`;
    })}
  <//>`;
}

function DeathsImpactSubSection({ deaths }) {
  if (!deaths || !deaths.length) return null;
  return deaths.map(function (d) {
    var rows = (d.buckets || []).map(function (b) {
      return [b.label, b.matches + '戦', pct(b.win_rate)];
    });
    return html`<div>
      <${Table} headers=${['被撃墜数', '試合数', '勝率']} rows=${rows} />
      <${Tips} tips=${d.tips} />
    </div>`;
  });
}

function TimeOfDaySection({ time }) {
  if (!time || !time.hours || !time.hours.length) return null;
  var rows = time.hours.map(function (h) {
    var mark = h.mark === 'good' ? '◎' : h.mark === 'bad' ? '△' : '';
    return [h.hour + '時', h.matches, pct(h.win_rate), num(h.dmg_efficiency, 3), mark];
  });
  return html`<${Section} title="時間帯別の勝率">
    <${Table} headers=${['時間帯', '試合', '勝率', '与被ダメ比', '']} rows=${rows} />
    <${Tips} tips=${time.tips} />
  <//>`;
}

function DayOfWeekSection({ dow }) {
  if (!dow) return null;
  var summaryRows = [];
  if (dow.weekday) summaryRows.push(['平日', dow.weekday.matches, pct(dow.weekday.win_rate), num(dow.weekday.dmg_efficiency, 3)]);
  if (dow.weekend) summaryRows.push(['土日', dow.weekend.matches, pct(dow.weekend.win_rate), num(dow.weekend.dmg_efficiency, 3)]);
  var dayRows = (dow.days || []).map(function (d) {
    return [d.name + '曜', d.matches, pct(d.win_rate), num(d.dmg_efficiency, 3)];
  });
  var headers = ['曜日', '試合', '勝率', '与被ダメ比'];
  return html`<${Section} title="曜日別の勝率">
    ${summaryRows.length > 0 && html`<div>
      <h3>平日 vs 土日</h3>
      <${Table} headers=${headers} rows=${summaryRows} />
    </div>`}
    ${dayRows.length > 0 && html`<div>
      <h3>曜日別</h3>
      <${Table} headers=${headers} rows=${dayRows} />
    </div>`}
    <${Tips} tips=${dow.tips} />
  <//>`;
}

function DailyTrendSection({ daily }) {
  if (!daily || !daily.days || !daily.days.length) return null;
  var rows = daily.days.map(function (d) {
    var mark = d.mark === 'good' ? '◎' : d.mark === 'bad' ? '△' : '';
    return [d.date + ' (' + d.dow_name + ')', d.matches, pct(d.win_rate), num(d.dmg_efficiency, 3), mark];
  });
  return html`<${Section} title="日別勝率">
    <${Table} headers=${['日付', '試合', '勝率', '与被ダメ比', '']} rows=${rows} />
    <${Tips} tips=${daily.tips} />
  <//>`;
}

function SeasonSection({ seasons }) {
  if (!seasons || !seasons.length) return null;
  return html`<${Section} title="シーズン別分析">
    ${seasons.map(function (s) {
      var rows = [['全体', s.matches, pct(s.win_rate), num(s.dmg_efficiency, 3)]];
      if (s.first_half) rows.push(['前半', s.first_half.matches, pct(s.first_half.win_rate), num(s.first_half.dmg_efficiency, 3)]);
      if (s.second_half) rows.push(['後半', s.second_half.matches, pct(s.second_half.win_rate), num(s.second_half.dmg_efficiency, 3)]);
      return html`<div>
        <h3>${esc(s.name)}</h3>
        <${Table} headers=${['期間', '試合', '勝率', '与被ダメ比']} rows=${rows} />
        <${Tips} tips=${s.tips} />
      </div>`;
    })}
  <//>`;
}

// --- Share area ---

function ShareArea({ shareData }) {
  if (!shareData || !shareData.length) return null;
  var text = buildShareText(shareData);
  var encoded = encodeURIComponent(text);
  var xUrl = 'https://x.com/intent/tweet?text=' + encoded;
  var bskyUrl = 'https://bsky.app/intent/compose?text=' + encoded;
  var lineUrl = 'https://line.me/R/share?text=' + encoded;

  function CopyButton() {
    var ref = useState(false);
    var copied = ref[0], setCopied = ref[1];
    function handleCopy() {
      navigator.clipboard.writeText(text).then(function () {
        setCopied(true);
        setTimeout(function () { setCopied(false); }, 2000);
      });
    }
    return html`<button class=${'share-btn share-copy' + (copied ? ' copied' : '')} onClick=${handleCopy} aria-label="テキストをコピー"
      dangerouslySetInnerHTML=${{ __html: copied ? SVG_CHECK : SVG_COPY }} />`;
  }

  return html`<div class="share-area">
    <span class="share-label">共有</span>
    <a href=${xUrl} target="_blank" rel="noopener noreferrer" class="share-btn share-x" aria-label="Xで共有" dangerouslySetInnerHTML=${{ __html: SVG_X }} />
    <a href=${bskyUrl} target="_blank" rel="noopener noreferrer" class="share-btn share-bsky" aria-label="Blueskyで共有" dangerouslySetInnerHTML=${{ __html: SVG_BSKY }} />
    <a href=${lineUrl} target="_blank" rel="noopener noreferrer" class="share-btn share-line" aria-label="LINEで共有" dangerouslySetInnerHTML=${{ __html: SVG_LINE }} />
    <${CopyButton} />
  </div>`;
}

// --- Table of Contents ---

function TableOfContents({ data }) {
  function toggleAll(open) {
    var details = document.querySelectorAll('#report details');
    for (var i = 0; i < details.length; i++) {
      details[i].open = open;
    }
  }

  var msEntries = [];
  if (data.ms_stats) {
    msEntries = Object.keys(data.ms_stats).sort(function (a, b) {
      return data.ms_stats[b].matches - data.ms_stats[a].matches;
    });
  }

  return html`<div class="toc-area">
    <details open>
      <summary><strong>目次</strong></summary>
      <ol>
        <li><a href="#sec-summary">総合アドバイス</a></li>
        <li><a href="#sec-basic">基本データ</a></li>
        ${msEntries.length > 0 && html`<li><a href=${'#' + msAnchorId(msEntries[0], 0)}>機体別分析</a>
          <details class="toc-ms-details">
            <summary>機体一覧</summary>
            <ul class="toc-ms-list">
              ${msEntries.map(function (msName, idx) {
                return html`<li><a href=${'#' + msAnchorId(msName, idx)}>${esc(msName)}</a></li>`;
              })}
            </ul>
          </details>
        </li>`}
        <li><a href="#sec-fixed">固定相方分析</a></li>
        <li><a href="#sec-time">時間帯別の勝率</a></li>
        <li><a href="#sec-dow">曜日別の勝率</a></li>
        <li><a href="#sec-daily">日別勝率</a></li>
        <li><a href="#sec-season">シーズン別分析</a></li>
      </ol>
    </details>
    <div class="toggle-all">
      <button class="toggle-btn" onClick=${function () { toggleAll(true); }}>すべて開く</button>
      <button class="toggle-btn" onClick=${function () { toggleAll(false); }}>すべて閉じる</button>
    </div>
    <hr />
  </div>`;
}

// --- Main report ---

function Report({ data, userKey }) {
  if (!data) return null;
  var periodRef = useState('all');
  var selectedPeriod = periodRef[0], setSelectedPeriod = periodRef[1];
  var customDataRef = useState(null);
  var customData = customDataRef[0], setCustomData = customDataRef[1];

  var periods = data.periods || {};
  // カスタム期間データがある場合はマージ
  var allPeriods = customData ? Object.assign({}, periods, { custom: customData.periods.custom }) : periods;
  var pd = allPeriods[selectedPeriod] || allPeriods['all'];
  if (!pd) return null;

  var shareData = selectedPeriod === 'custom' && customData ? customData.share_data : data.share_data;

  function handleCustomReport(report) {
    setCustomData(report);
    setSelectedPeriod('custom');
  }

  return html`
    <h1>${esc(data.player_name)} - 戦績分析レポート</h1>
    <${ShareArea} shareData=${shareData} />
    <${PeriodSelector} periods=${allPeriods} selected=${selectedPeriod} onSelect=${setSelectedPeriod}
      userKey=${userKey} onCustomReport=${handleCustomReport} />
    <${TableOfContents} data=${pd} />
    <div key="sec-summary" id="sec-summary"><${SummarySection} summary=${pd.summary} /></div>
    <div key="sec-basic" id="sec-basic"><${Section} title="基本データ">
      <${SubSection} title="基本データ" open>
        <${BasicStatsSection} stats=${pd.basic_stats} />
      <//>
      <${SubSection} title="勝利時/敗北時のダメージ傾向">
        <${WinLossPatternSection} pattern=${pd.win_loss_pattern} />
      <//>
    <//></div>
    <div key="sec-ms"><${MsStatsSection} msStats=${pd.ms_stats} /></div>
    <div key="sec-fixed" id="sec-fixed"><${FixedPartnersSection} partners=${pd.fixed_partners} /></div>
    <div key="sec-time" id="sec-time"><${TimeOfDaySection} time=${pd.time_of_day} /></div>
    <div key="sec-dow" id="sec-dow"><${DayOfWeekSection} dow=${pd.day_of_week} /></div>
    <div key="sec-daily" id="sec-daily"><${DailyTrendSection} daily=${pd.daily_trend} /></div>
    <div key="sec-season" id="sec-season"><${SeasonSection} seasons=${pd.season} /></div>
    <${ShareArea} shareData=${shareData} />
  `;
}

// --- Main app logic ---

function renderReport(data, userKey) {
  var reportEl = document.getElementById('report');
  reportEl.style.display = 'block';
  render(html`<${Report} data=${data} userKey=${userKey} />`, reportEl);
}

async function analyze() {
  var username = document.getElementById('username').value;
  var password = document.getElementById('password').value;
  var btn = document.getElementById('analyzeBtn');
  var status = document.getElementById('status');
  var statusText = document.getElementById('statusText');
  var error = document.getElementById('error');
  var reportEl = document.getElementById('report');

  if (!username || !password) {
    error.style.display = 'block';
    error.textContent = 'メールアドレスとパスワードを入力してください。';
    return;
  }

  btn.disabled = true;
  status.style.display = 'block';
  statusText.textContent = STATUS_MESSAGES.pending;
  error.style.display = 'none';
  reportEl.style.display = 'none';
  render(null, reportEl);

  document.getElementById('loginForm').style.display = 'none';
  var preliminaryShown = false;

  try {
    var res = await fetch('/analyze', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username, password: password }),
    });

    var data = await res.json();
    if (data.error) {
      throw new Error(data.error);
    }

    var jobId = data.id;

    while (true) {
      await new Promise(function (r) { setTimeout(r, 3000); });

      var statusRes = await fetch('/status/' + jobId);
      var statusData = await statusRes.json();

      if (statusData.error && statusData.status !== 'error') {
        throw new Error(statusData.error);
      }

      statusText.textContent = statusData.message || STATUS_MESSAGES[statusData.status] || statusData.status;

      var progressWrap = document.getElementById('progressWrap');
      if (statusData.progress_total > 0) {
        var p = Math.round(100 * statusData.progress / statusData.progress_total);
        document.getElementById('progressFill').style.width = p + '%';
        document.getElementById('progressPct').textContent = p + '%';
        document.getElementById('progressCount').textContent = statusData.progress + '/' + statusData.progress_total + '件';
        progressWrap.style.display = 'block';
      } else {
        progressWrap.style.display = 'none';
      }

      if (statusData.has_preliminary_report && !preliminaryShown) {
        var prelimRes = await fetch('/result/' + jobId);
        var prelimData = await prelimRes.json();
        if (prelimData.report && prelimData.preliminary) {
          renderReport(prelimData.report, prelimData.user_key);
          statusText.textContent = '最新データを取得中...';
          preliminaryShown = true;
        }
      }

      if (statusData.status === 'error') {
        throw new Error(statusData.error || '分析に失敗しました');
      }

      if (statusData.status === 'done') {
        var resultRes = await fetch('/result/' + jobId);
        var resultData = await resultRes.json();

        if (resultData.error) {
          throw new Error(resultData.error);
        }

        renderReport(resultData.report, resultData.user_key);
        break;
      }
    }
  } catch (e) {
    error.style.display = 'block';
    error.textContent = e.message;
    document.getElementById('loginForm').style.display = 'block';
  } finally {
    btn.disabled = false;
    status.style.display = 'none';
  }
}

if (document.getElementById('analyzeBtn')) {
  document.getElementById('analyzeBtn').addEventListener('click', analyze);
  document.getElementById('password').addEventListener('keypress', function (e) {
    if (e.key === 'Enter') analyze();
  });
}
