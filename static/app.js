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
        report.innerHTML = DOMPurify.sanitize(marked.parse(resultData.report));
        report.querySelectorAll('h2, h3').forEach(function(h) {
          h.id = h.textContent.replace(/\s+/g, '-');
        });
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

document.getElementById('analyzeBtn').addEventListener('click', analyze);
document.getElementById('password').addEventListener('keypress', function(e) {
  if (e.key === 'Enter') analyze();
});
