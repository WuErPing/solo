export function isSvgContent(source: string): boolean {
  if (!source) return false;
  const trimmed = source.trim();
  return trimmed.startsWith("<svg") && trimmed.includes('xmlns="http://www.w3.org/2000/svg"');
}

export function sanitizeSvgContent(source: string): string {
  let sanitized = source;
  sanitized = sanitized.replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, "");
  sanitized = sanitized.replace(/\bon\w+\s*=\s*["'][^"']*["']/gi, "");
  sanitized = sanitized.replace(/\bon\w+\s*=\s*[^\s>]*/gi, "");
  sanitized = sanitized.replace(/xlink:href\s*=\s*["']javascript:[^"']*["']/gi, "");
  sanitized = sanitized.replace(/href\s*=\s*["']javascript:[^"']*["']/gi, "");
  return sanitized;
}

export function ensureSvgDimensions(source: string): string {
  if (source.includes('width=') && source.includes('height=')) {
    return source;
  }
  // Don't add width/height if viewBox exists - let CSS handle scaling
  if (source.includes('viewBox=')) {
    return source;
  }
  return source.replace(/<svg\b/, '<svg width="100%" height="100%"');
}

export function generateSvgHtml(source: string, isDark: boolean): string {
  const background = isDark ? "#0d1117" : "#ffffff";
  const sanitized = sanitizeSvgContent(source);
  const withDimensions = ensureSvgDimensions(sanitized);

  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=5.0, user-scalable=yes">
<style>
* {
  box-sizing: border-box;
}
html, body {
  margin: 0;
  padding: 0;
  width: 100%;
  height: 100%;
  overflow: auto;
  background: ${background};
}
body {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  min-height: 100dvh;
}
svg {
  width: 100%;
  height: auto;
  max-width: 100%;
  max-height: 100vh;
  max-height: 100dvh;
  display: block;
  touch-action: pinch-zoom;
}
</style>
</head>
<body>
${withDimensions}
</body>
</html>`;
}
