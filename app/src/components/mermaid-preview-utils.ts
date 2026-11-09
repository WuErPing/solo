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
<script src="https://cdn.jsdelivr.net/npm/mermaid@12/dist/mermaid.min.js"></script>
<style>
body {
  margin: 0;
  padding: 16px;
  box-sizing: border-box;
  background: #ffffff;
}
#mermaid-graph {
  width: 100%;
  text-align: center;
}
/* mermaid appends an unstyled measurement div directly to body while it
   renders. Keep it fully laid out but off-screen so the in-progress diagram
   never flashes into view or stretches the document (same technique as the
   native service shell stage; visibility/display would break measurement). */
body > div:not(#mermaid-graph) {
  position: absolute !important;
  left: -100000px !important;
  top: 0 !important;
}
</style>
</head>
<body>
<div id="mermaid-graph"></div>
<script>
(function () {
  var cache = new Map();
  var currentTheme = null;
  var currentWidth = null;
  var seq = 0;
  var pending = null;
  var inFlightKey = null;
  var renderedKey = null;
  var latestKey = null;

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

  // mermaid sizes gantt charts from the task list alone (a wide timeline
  // still comes out ~226px), ignoring the container. useWidth pins the diagram
  // to the pane width so it scales with the device instead of a narrow strip.
  function applyConfig(theme, useWidth) {
    document.body.style.background = theme === 'dark' ? '#0d1117' : '#ffffff';
    if (theme === currentTheme && useWidth === currentWidth) {
      return;
    }
    mermaid.initialize({
      startOnLoad: false,
      theme: theme,
      gantt: { useWidth: useWidth }
    });
    currentTheme = theme;
    currentWidth = useWidth;
  }

  function paneWidth(el) {
    var width = Math.floor(el.clientWidth || (window.innerWidth - 32));
    return width > 0 ? width : 800;
  }

  function notify() {
    // body scrollHeight includes the 16px padding, so the host can size the
    // iframe container to the diagram and nothing gets clipped.
    var height = Math.ceil(document.body.scrollHeight);
    var message = { type: 'mermaid:rendered', height: height };
    if (window.ReactNativeWebView) {
      window.ReactNativeWebView.postMessage(JSON.stringify(message));
    }
    parent.postMessage(message, '*');
  }

  function render(source, theme) {
    var el = document.getElementById('mermaid-graph');
    var useWidth = paneWidth(el);
    var key = theme + '\\u0000' + useWidth + '\\u0000' + source;
    latestKey = key;
    // The host posts each request twice (iframe load + shell ready) and can
    // post a newer diagram before an older one finishes. Skip work that is
    // already on screen or already in flight, and ignore a finished render
    // that a newer request has superseded — otherwise duplicate mermaid.render
    // calls flash a half-drawn diagram and stale results clobber the latest.
    if (key === renderedKey || key === inFlightKey) {
      return;
    }
    applyConfig(theme, useWidth);
    if (cache.has(key)) {
      el.innerHTML = cache.get(key);
      renderedKey = key;
      notify();
      return;
    }
    inFlightKey = key;
    mermaid.render('mermaid-' + (++seq), source).then(function (result) {
      inFlightKey = null;
      cache.set(key, result.svg);
      if (key !== latestKey) {
        return;
      }
      el.innerHTML = result.svg;
      renderedKey = key;
      notify();
    }).catch(function (err) {
      inFlightKey = null;
      if (key !== latestKey) {
        return;
      }
      el.innerHTML = '<pre style="color:red;padding:16px;">' +
        escapeText((err && err.message) || err) + '</pre>';
      renderedKey = key;
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
