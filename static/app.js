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

function showShareButton(markdown) {
  var existing = document.getElementById('shareArea');
  if (existing) existing.remove();

  var match = markdown.match(/<!-- SHARE_DATA:(.*?) -->/);
  if (!match) return;

  var items;
  try { items = JSON.parse(match[1]); } catch (e) { return; }
  if (!items.length) return;

  var text = buildShareText(items);
  var encoded = encodeURIComponent(text);
  var xUrl = 'https://x.com/intent/tweet?text=' + encoded;
  var bskyUrl = 'https://bsky.app/intent/compose?text=' + encoded;
  var lineUrl = 'https://line.me/R/share?text=' + encoded;

  var area = document.createElement('div');
  area.id = 'shareArea';
  area.className = 'share-area';
  area.innerHTML =
    '<a href="' + xUrl + '" target="_blank" rel="noopener" class="share-btn share-x">X</a>' +
    '<a href="' + bskyUrl + '" target="_blank" rel="noopener" class="share-btn share-bsky">Bluesky</a>' +
    '<a href="' + lineUrl + '" target="_blank" rel="noopener" class="share-btn share-line">LINE</a>' +
    '<button class="share-btn share-copy" onclick="copyShareText()">コピー</button>';
  area.dataset.shareText = text;

  document.getElementById('report').before(area);
}

function copyShareText() {
  var area = document.getElementById('shareArea');
  if (!area) return;
  navigator.clipboard.writeText(area.dataset.shareText).then(function() {
    var btn = area.querySelector('.share-copy');
    btn.textContent = 'コピーしました！';
    setTimeout(function() { btn.textContent = 'テキストをコピー'; }, 2000);
  });
}

document.getElementById('analyzeBtn').addEventListener('click', analyze);
document.getElementById('password').addEventListener('keypress', function(e) {
  if (e.key === 'Enter') analyze();
});
