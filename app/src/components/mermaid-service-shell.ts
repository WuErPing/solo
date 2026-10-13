// HTML shell for the shared native mermaid render service. Unlike the
// per-diagram preview shell (mermaid-preview-utils.ts), this document is a
// pure renderer: it exposes `__renderToSvg(id, source, theme)`, renders the
// diagram off-screen, inlines computed styles as presentation attributes
// (react-native-svg has no CSS cascade), strips <style> blocks, and posts the
// resulting SVG markup back through the ReactNativeWebView bridge.
//
// The mermaid bundle is inlined (mermaid-assets.ts), so the service works
// offline and pays the multi-MB JS parse exactly once per app run.

export function generateMermaidServiceShellHtml(mermaidJs: string): string {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
/* The host WebView is deliberately tiny, but mermaid measures the document
   width for some diagram types (gantt shrank to 2px in practice). Give the
   page a stable layout width so diagrams render at natural size. */
html, body { margin: 0; padding: 0; background: transparent; min-width: 1000px; }
/* Off-screen but fully laid out: visibility/opacity/display here would
   pollute every element's getComputedStyle, which we inline into the SVG. */
#mermaid-service-stage {
  position: absolute;
  left: -100000px;
  top: 0;
}
</style>
</head>
<body>
<div id="mermaid-service-stage"></div>
<script>${mermaidJs}</script>
<script>
(function () {
  var stage = document.getElementById('mermaid-service-stage');
  var currentTheme = null;

  // Serialize renders: a burst of concurrent mermaid.render calls in one
  // WebView thrashes layout and canvas encoding; a FIFO keeps each render's
  // latency predictable and avoids bridge message pile-ups.
  var queue = [];
  var working = false;

  function enqueue(job) {
    queue.push(job);
    if (!working) {
      working = true;
      next();
    }
  }

  function next() {
    var job = queue.shift();
    if (!job) {
      working = false;
      return;
    }
    job.run().then(function () { next(); }, function () { next(); });
  }

  // Presentation attributes react-native-svg understands. Values are copied
  // from getComputedStyle so class-based rules survive the loss of <style>.
  var STYLE_PROPS = [
    'fill', 'fill-opacity', 'fill-rule',
    'stroke', 'stroke-width', 'stroke-opacity', 'stroke-dasharray',
    'stroke-dashoffset', 'stroke-linecap', 'stroke-linejoin', 'stroke-miterlimit',
    'opacity', 'color', 'font-family', 'font-size', 'font-weight', 'font-style',
    'text-anchor', 'dominant-baseline', 'visibility', 'display'
  ];

  function post(message) {
    if (window.ReactNativeWebView) {
      window.ReactNativeWebView.postMessage(JSON.stringify(message));
    }
  }

  function applyTheme(theme) {
    if (theme === currentTheme) return;
    mermaid.initialize({
      startOnLoad: false,
      theme: theme,
      // react-native-svg cannot render HTML labels inside foreignObject.
      flowchart: { htmlLabels: false },
      securityLevel: 'strict'
    });
    currentTheme = theme;
  }

  function inlineStyles(root) {
    var els = root.querySelectorAll('*');
    for (var i = 0; i < els.length; i++) {
      var el = els[i];
      var cs = getComputedStyle(el);
      for (var j = 0; j < STYLE_PROPS.length; j++) {
        var prop = STYLE_PROPS[j];
        var value = cs.getPropertyValue(prop);
        if (value !== '' && value != null) {
          el.setAttribute(prop, value);
        }
      }
    }
    var styles = root.querySelectorAll('style');
    for (var k = 0; k < styles.length; k++) {
      styles[k].parentNode.removeChild(styles[k]);
    }
  }

  // mermaid emits <marker> elements as loose children of the root group.
  // Browsers never render <marker> directly, but react-native-svg's XmlParser
  // paints them in place — collect them into a <defs> so they only render
  // through marker-end/marker-start references.
  function collectMarkersIntoDefs(svgEl) {
    var markers = svgEl.querySelectorAll('marker');
    if (!markers.length) return;
    var defs = svgEl.querySelector('defs');
    if (!defs) {
      defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
      svgEl.insertBefore(defs, svgEl.firstChild);
    }
    for (var i = 0; i < markers.length; i++) {
      defs.appendChild(markers[i]);
    }
  }

  function naturalSize(svgEl) {
    var vb = (svgEl.getAttribute('viewBox') || '').trim().split(/\\s+/).map(Number);
    var widthAttr = svgEl.getAttribute('width') || '';
    var heightAttr = svgEl.getAttribute('height') || '';
    var width = widthAttr.indexOf('%') >= 0 ? NaN : parseFloat(widthAttr);
    var height = heightAttr.indexOf('%') >= 0 ? NaN : parseFloat(heightAttr);
    if ((!isFinite(width) || !isFinite(height) || width <= 0 || height <= 0) && vb.length === 4) {
      width = vb[2];
      height = vb[3];
    }
    return { width: width, height: height };
  }

  window.__renderToSvg = function (id, source, theme) {
    post({ type: 'mermaid:debug', id: id, phase: 'enter' });
    enqueue({
      run: function () {
        return new Promise(function (done) {
          var fail = function (error) {
            post({ type: 'mermaid:svg', id: id, ok: false, error: String(error) });
            done();
          };
          try {
            if (typeof mermaid === 'undefined') {
              fail('mermaid unavailable');
              return;
            }
            applyTheme(theme === 'dark' ? 'dark' : 'default');
            mermaid.render('mermaid-svc-' + id, String(source == null ? '' : source).trim())
              .then(function (result) {
                post({ type: 'mermaid:debug', id: id, phase: 'rendered' });
                try {
                  stage.innerHTML = result.svg;
                  var svgEl = stage.querySelector('svg');
                  if (!svgEl) {
                    stage.innerHTML = '';
                    fail('no svg produced');
                    return;
                  }
                  if (svgEl.querySelector('foreignObject')) {
                    stage.innerHTML = '';
                    fail('unsupported:foreignObject');
                    return;
                  }
                  inlineStyles(svgEl);
                  collectMarkersIntoDefs(svgEl);
                  var size = naturalSize(svgEl);
                  if (!isFinite(size.width) || !isFinite(size.height) || size.width <= 0 || size.height <= 0) {
                    stage.innerHTML = '';
                    fail('unmeasurable');
                    return;
                  }
                  // Canvas rasterization needs intrinsic dimensions on the root.
                  svgEl.setAttribute('width', String(size.width));
                  svgEl.setAttribute('height', String(size.height));
                  var markup = svgEl.outerHTML;
                  stage.innerHTML = '';

                  // Paint to a bitmap: an RN Image costs one native view and a
                  // few MB of decoded pixels, where the SVG view tree costs
                  // hundreds of views per diagram. 2x keeps text sharp on
                  // high-density screens.
                  var scale = 2;
                  var canvas = document.createElement('canvas');
                  canvas.width = Math.min(Math.ceil(size.width * scale), 8192);
                  canvas.height = Math.min(Math.ceil(size.height * scale), 8192);
                  var postSvg = function () {
                    post({
                      type: 'mermaid:svg', id: id, ok: true,
                      svg: markup, width: size.width, height: size.height
                    });
                    done();
                  };
                  var img = new Image();
                  img.onload = function () {
                    try {
                      var ctx = canvas.getContext('2d');
                      if (theme === 'dark') {
                        ctx.fillStyle = '#0d1117';
                        ctx.fillRect(0, 0, canvas.width, canvas.height);
                      }
                      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
                      var png = canvas.toDataURL('image/png');
                      // A healthy diagram raster is kilobytes at minimum; a
                      // tiny PNG means the canvas drew nothing (observed on
                      // some Android WebViews) — fall back to the vector path.
                      if (png.length < 2000) {
                        postSvg();
                        return;
                      }
                      post({
                        type: 'mermaid:svg',
                        id: id,
                        ok: true,
                        png: png,
                        width: size.width,
                        height: size.height
                      });
                      done();
                    } catch (error) {
                      postSvg();
                    }
                  };
                  img.onerror = postSvg;
                  img.src = 'data:image/svg+xml;base64,' +
                    btoa(unescape(encodeURIComponent(markup)));
                } catch (error) {
                  stage.innerHTML = '';
                  fail((error && error.message) || error);
                }
              })
              .catch(function (error) {
                fail((error && error.message) || error);
              });
          } catch (error) {
            fail((error && error.message) || error);
          }
        });
      }
    });
  };

  post({ type: 'mermaid:service-ready' });

  // Health probe: the host re-creates the WebView when the shell fails to
  // boot (e.g. renderer died under memory pressure), which a plain
  // onLoadEnd alone cannot distinguish.
  window.__mermaidPing = function (id) {
    post({ type: 'mermaid:pong', id: id });
  };
})();
</script>
</body>
</html>`;
}
