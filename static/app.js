const STATUS_MESSAGES = {
  pending: '準備中...',
  scraping: '戦績を取得中...（数分かかります）',
  analyzing: '分析中...',
  done: '完了',
  error: 'エラーが発生しました',
};

async function analyze() {
  const username = document.getElementById('username').value;
  const password = document.getElementById('password').value;
  const btn = document.getElementById('analyzeBtn');
  const status = document.getElementById('status');
  const statusText = document.getElementById('statusText');
  const error = document.getElementById('error');
  const report = document.getElementById('report');

  if (!username || !password) {
    error.style.display = 'block';
    error.textContent = 'メールアドレスとパスワードを入力してください。';
    return;
  }

  btn.disabled = true;
  status.style.display = 'block';
  statusText.textContent = STATUS_MESSAGES.pending;
  error.style.display = 'none';
  report.style.display = 'none';
  document.querySelectorAll('.share-area').forEach(function(el) { el.remove(); });

  try {
    // ジョブ作成
    const res = await fetch('/analyze', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });

    const data = await res.json();
    if (data.error) {
      throw new Error(data.error);
    }

    const jobId = data.id;

    // ポーリングで進捗確認
    while (true) {
      await new Promise(r => setTimeout(r, 3000));

      const statusRes = await fetch(`/status/${jobId}`);
      const statusData = await statusRes.json();

      if (statusData.error && statusData.status !== 'error') {
        throw new Error(statusData.error);
      }

      statusText.textContent = statusData.message || STATUS_MESSAGES[statusData.status] || statusData.status;

      var progressWrap = document.getElementById('progressWrap');
      if (statusData.progress_total > 0) {
        var pct = Math.round(100 * statusData.progress / statusData.progress_total);
        document.getElementById('progressFill').style.width = pct + '%';
        document.getElementById('progressPct').textContent = pct + '%';
        document.getElementById('progressCount').textContent = statusData.progress + '/' + statusData.progress_total + '件';
        progressWrap.style.display = 'block';
      } else {
        progressWrap.style.display = 'none';
      }

      if (statusData.status === 'error') {
        throw new Error(statusData.error || '分析に失敗しました');
      }

      if (statusData.status === 'done') {
        // レポート取得
        const resultRes = await fetch(`/result/${jobId}`);
        const resultData = await resultRes.json();

        if (resultData.error) {
          throw new Error(resultData.error);
        }

        report.style.display = 'block';
        report.innerHTML = DOMPurify.sanitize(marked.parse(resultData.report), {ADD_TAGS: ['details', 'summary']});
        report.querySelectorAll('h2, h3').forEach(function(h) {
          h.id = h.textContent.replace(/\s+/g, '-');
        });
        report.querySelectorAll('table').forEach(function(table) {
          var wrap = document.createElement('div');
          wrap.className = 'table-wrap';
          table.parentNode.insertBefore(wrap, table);
          wrap.appendChild(table);
        });
        showShareButton(resultData.report);
        break;
      }
    }
  } catch (e) {
    error.style.display = 'block';
    error.textContent = e.message;
  } finally {
    btn.disabled = false;
    status.style.display = 'none';
  }
}

function buildShareText(items) {
  var lines = ['【EXVS2XB 戦績診断】'];
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
  lines.push('#EXVS2XB');
  return lines.join('\n');
}

var SVG_X = '<svg viewBox="0 0 24 24"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>';
var SVG_BSKY = '<svg viewBox="0 0 568 501"><path d="M123.121 33.664C188.241 82.553 258.281 181.68 284 234.873c25.719-53.192 95.759-152.32 160.879-201.21C491.866-1.611 568-28.906 568 57.947c0 17.346-9.945 145.713-15.778 166.555-20.275 72.453-94.155 90.933-159.875 79.748C507.222 323.8 536.444 388.56 473.333 453.32c-119.86 122.992-172.272-30.859-185.702-70.281-2.462-7.227-3.614-10.608-3.631-7.733-.017-2.875-1.169.506-3.631 7.733-13.43 39.422-65.842 193.273-185.702 70.281-63.111-64.76-33.889-129.52 80.986-149.071-65.72 11.185-139.6-7.295-159.875-79.748C9.945 203.659 0 75.291 0 57.946 0-28.906 76.135-1.612 123.121 33.664z"/></svg>';
var SVG_LINE = '<svg viewBox="0 0 24 24"><path d="M19.365 9.863c.349 0 .63.285.63.631 0 .345-.281.63-.63.63H17.61v1.125h1.755c.349 0 .63.283.63.63 0 .344-.281.629-.63.629h-2.386c-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63h2.386c.346 0 .627.285.627.63 0 .349-.281.63-.63.63H17.61v1.125h1.755zm-3.855 3.016c0 .27-.174.51-.432.596-.064.021-.133.031-.199.031-.211 0-.391-.09-.51-.25l-2.443-3.317v2.94c0 .344-.279.629-.631.629-.346 0-.626-.285-.626-.629V8.108c0-.27.173-.51.43-.595.06-.023.136-.033.194-.033.195 0 .375.104.495.254l2.462 3.33V8.108c0-.345.282-.63.63-.63.345 0 .63.285.63.63v4.771zm-5.741 0c0 .344-.282.629-.631.629-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63.346 0 .628.285.628.63v4.771zm-2.466.629H4.917c-.345 0-.63-.285-.63-.629V8.108c0-.345.285-.63.63-.63.348 0 .63.285.63.63v4.141h1.756c.348 0 .629.283.629.63 0 .344-.282.629-.629.629M24 10.314C24 4.943 18.615.572 12 .572S0 4.943 0 10.314c0 4.811 4.27 8.842 10.035 9.608.391.082.923.258 1.058.59.12.301.079.766.038 1.08l-.164 1.02c-.045.301-.24 1.186 1.049.645 1.291-.539 6.916-4.078 9.436-6.975C23.176 14.393 24 12.458 24 10.314"/></svg>';
var SVG_COPY = '<svg viewBox="0 0 24 24"><path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/></svg>';
var SVG_CHECK = '<svg viewBox="0 0 24 24"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>';

function createShareArea(id, text) {
  var encoded = encodeURIComponent(text);
  var xUrl = 'https://x.com/intent/tweet?text=' + encoded;
  var bskyUrl = 'https://bsky.app/intent/compose?text=' + encoded;
  var lineUrl = 'https://line.me/R/share?text=' + encoded;

  var area = document.createElement('div');
  area.id = id;
  area.className = 'share-area';
  area.innerHTML =
    '<span class="share-label">共有</span>' +
    '<a href="' + xUrl + '" target="_blank" rel="noopener noreferrer" class="share-btn share-x" aria-label="Xで共有">' + SVG_X + '</a>' +
    '<a href="' + bskyUrl + '" target="_blank" rel="noopener noreferrer" class="share-btn share-bsky" aria-label="Blueskyで共有">' + SVG_BSKY + '</a>' +
    '<a href="' + lineUrl + '" target="_blank" rel="noopener noreferrer" class="share-btn share-line" aria-label="LINEで共有">' + SVG_LINE + '</a>' +
    '<button class="share-btn share-copy" onclick="copyShareText(this)" aria-label="テキストをコピー">' + SVG_COPY + '</button>';
  area.dataset.shareText = text;
  return area;
}

function showShareButton(markdown) {
  document.querySelectorAll('.share-area').forEach(function(el) { el.remove(); });

  var match = markdown.match(/<!-- SHARE_DATA:(.*?) -->/);
  if (!match) return;

  var items;
  try { items = JSON.parse(match[1]); } catch (e) { return; }
  if (!items.length) return;

  var text = buildShareText(items);
  var report = document.getElementById('report');
  report.before(createShareArea('shareAreaTop', text));
  report.after(createShareArea('shareAreaBottom', text));
}

function copyShareText(btn) {
  var area = btn.closest('.share-area');
  if (!area) return;
  navigator.clipboard.writeText(area.dataset.shareText).then(function() {
    btn.classList.add('copied');
    btn.innerHTML = SVG_CHECK;
    setTimeout(function() {
      btn.classList.remove('copied');
      btn.innerHTML = SVG_COPY;
    }, 2000);
  });
}

if (document.getElementById('analyzeBtn')) {
  document.getElementById('analyzeBtn').addEventListener('click', analyze);
  document.getElementById('password').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') analyze();
  });
}
