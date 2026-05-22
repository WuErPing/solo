export function escapeHtml(unsafe: string): string {
  return unsafe
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

export function generateMermaidHtml(source: string, isDark = false): string {
  const theme = isDark ? "dark" : "default";
  const background = isDark ? "#0d1117" : "#ffffff";

  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
<script>
mermaid.initialize({ startOnLoad: false, theme: '${theme}' });
document.addEventListener('DOMContentLoaded', async function () {
  var el = document.getElementById('mermaid-graph');
  try {
    var result = await mermaid.render('mermaid-' + Date.now(), el.textContent.trim());
    el.innerHTML = result.svg;
  } catch (err) {
    el.innerHTML = '<pre style="color:red;padding:16px;">' + (err.message || String(err)) + '</pre>';
  }
});
</script>
<style>
body {
  margin: 0;
  padding: 16px;
  display: flex;
  justify-content: center;
  align-items: center;
  background: ${background};
  min-height: 100vh;
  box-sizing: border-box;
}
#mermaid-graph {
  width: 100%;
  text-align: center;
}
</style>
</head>
<body>
<div id="mermaid-graph" class="mermaid">${escapeHtml(source)}</div>
</body>
</html>`;
}
