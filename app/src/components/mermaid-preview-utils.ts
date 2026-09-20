export function escapeHtml(unsafe: string): string {
  return unsafe
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

/**
 * Static, source-independent HTML shell that loads mermaid exactly once and
 * exposes `window.__renderMermaid(source, theme)` for incremental re-renders.
 * The mermaid source is pushed in at runtime (postMessage on web,
 * injectJavaScript on native) rather than interpolated here, so changing the
 * diagram never reloads the document or re-fetches the CDN bundle.
 */
export function generateMermaidShellHtml(): string {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
<style>
body {
  margin: 0;
  padding: 16px;
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  box-sizing: border-box;
  background: #ffffff;
}
#mermaid-graph {
  width: 100%;
  text-align: center;
}
</style>
</head>
<body>
<div id="mermaid-graph"></div>
<script>
(function () {
  var cache = new Map();
  var currentTheme = null;
  var seq = 0;
  var pending = null;

  function escapeText(value) {
    return String(value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  function showError(message) {
    var el = document.getElementById('mermaid-graph');
    el.innerHTML = '<pre style="color:#c0392b;padding:16px;white-space:pre-wrap;text-align:left;">' +
      escapeText(message) + '</pre>';
    notify();
  }

  function applyTheme(theme) {
    document.body.style.background = theme === 'dark' ? '#0d1117' : '#ffffff';
    if (theme !== currentTheme) {
      mermaid.initialize({ startOnLoad: false, theme: theme });
      currentTheme = theme;
    }
  }

  function notify() {
    var el = document.getElementById('mermaid-graph');
    var height = Math.ceil(el.getBoundingClientRect().height);
    parent.postMessage({ type: 'mermaid:rendered', height: height }, '*');
  }

  function render(source, theme) {
    var el = document.getElementById('mermaid-graph');
    var key = theme + '\\u0000' + source;
    if (cache.has(key)) {
      applyTheme(theme);
      el.innerHTML = cache.get(key);
      notify();
      return;
    }
    applyTheme(theme);
    mermaid.render('mermaid-' + (++seq), source).then(function (result) {
      cache.set(key, result.svg);
      el.innerHTML = result.svg;
      notify();
    }).catch(function (err) {
      el.innerHTML = '<pre style="color:red;padding:16px;">' +
        escapeText((err && err.message) || err) + '</pre>';
      notify();
    });
  }

  var waitTimer = null;
  var waitedMs = 0;
  var LOAD_TIMEOUT_MS = 10000;

  function flushPending() {
    if (pending) {
      var queued = pending;
      pending = null;
      window.__renderMermaid(queued.source, queued.theme);
    }
  }

  // The CDN script is render-blocking, so mermaid is normally defined by now.
  // If it is missing (blocked by network/CSP or still loading), wait briefly and
  // surface a visible error instead of leaving the preview silently blank.
  function waitForMermaidThenFlush() {
    if (waitTimer) return;
    waitTimer = setInterval(function () {
      waitedMs += 250;
      if (typeof mermaid !== 'undefined') {
        clearInterval(waitTimer);
        waitTimer = null;
        flushPending();
      } else if (waitedMs >= LOAD_TIMEOUT_MS) {
        clearInterval(waitTimer);
        waitTimer = null;
        showError('Mermaid 库加载失败：无法从 cdn.jsdelivr.net 获取。请检查网络或内容安全策略（CSP）是否放行该域名。');
      }
    }, 250);
  }

  window.__renderMermaid = function (source, theme) {
    var nextTheme = theme || 'default';
    if (typeof mermaid === 'undefined') {
      pending = { source: source, theme: nextTheme };
      waitForMermaidThenFlush();
      return;
    }
    render(String(source == null ? '' : source).trim(), nextTheme);
  };

  window.addEventListener('message', function (event) {
    var data = event.data;
    if (data && data.type === 'mermaid:render') {
      window.__renderMermaid(data.source, data.theme);
    }
  });

  function onReady() {
    parent.postMessage({ type: 'mermaid:ready' }, '*');
    if (typeof mermaid !== 'undefined') {
      flushPending();
    }
  }

  if (typeof mermaid !== 'undefined') {
    onReady();
  } else {
    window.addEventListener('load', onReady);
  }
})();
</script>
</body>
</html>`;
}
