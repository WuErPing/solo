// Shared mermaid render service for native. A single hidden WebView with the
// inlined mermaid bundle renders every diagram in the document to SVG; the
// results are cached process-wide and painted with react-native-svg. This
// replaces one WebView per diagram (each paying a multi-MB CDN download and
// JS parse) with one WebView total.
/* eslint-disable @typescript-eslint/no-require-imports -- lazy require keeps the multi-MB mermaid bundle string out of the file-pane import path */
import React, { useEffect, useMemo, useRef } from "react";
import { StyleSheet, useWindowDimensions, View } from "react-native";
import { WebView, type WebViewMessageEvent } from "react-native-webview";
import { generateMermaidServiceShellHtml } from "./mermaid-service-shell";
import {
  getCachedMermaidSvg,
  mermaidCacheKey,
  setCachedMermaidSvg,
  type MermaidSvgEntry,
} from "./mermaid-svg-cache";
import { perfLog, perfNow } from "@/utils/perf-log";

const REQUEST_TIMEOUT_MS = 45_000;

type RenderResolve = (entry: MermaidSvgEntry | null) => void;

interface PendingRequest {
  source: string;
  theme: string;
  resolve: RenderResolve;
  enqueuedAt: number;
  timer: ReturnType<typeof setTimeout>;
}

const pendingById = new Map<number, PendingRequest>();
const inFlightByKey = new Map<string, Promise<MermaidSvgEntry | null>>();
const queuedBeforeReady: { id: number; source: string; theme: string }[] = [];
let nextRequestId = 1;
let serviceWebView: WebView | null = null;
/** True once the shell's JS confirmed itself alive (ready message or pong). */
let shellAlive = false;
/** Bumped by the host to force a WebView remount; also invoked on failures. */
let requestHostRemount: () => void = () => undefined;

function buildRenderScript(id: number, source: string, theme: string): string {
  return `window.__renderToSvg(${id}, ${JSON.stringify(source)}, ${JSON.stringify(theme)}); true;`;
}

function settleRequest(id: number, entry: MermaidSvgEntry | null): void {
  const pending = pendingById.get(id);
  if (!pending) {
    return;
  }
  pendingById.delete(id);
  clearTimeout(pending.timer);
  pending.resolve(entry);
}

function dispatchRequest(id: number, source: string, theme: string): void {
  if (!serviceWebView) {
    perfLog("mermaid.dispatch.noWebview", { id });
  }
  serviceWebView?.injectJavaScript(buildRenderScript(id, source, theme));
}

interface ServiceMessage {
  type?: string;
  id?: number;
  ok?: boolean;
  svg?: string;
  png?: string;
  width?: number;
  height?: number;
  error?: string;
  phase?: string;
}

function flushQueuedRequests(): void {
  // Re-dispatch everything still pending: requests sent to a shell that died
  // mid-flight are simply rendered again (duplicate responses are ignored).
  const all = [...queuedBeforeReady.splice(0)];
  for (const [id, pending] of pendingById) {
    all.push({ id, source: pending.source, theme: pending.theme });
  }
  const seen = new Set<number>();
  for (const request of all) {
    if (seen.has(request.id)) continue;
    seen.add(request.id);
    dispatchRequest(request.id, request.source, request.theme);
  }
}

function handleRenderResult(data: ServiceMessage): void {
  if (typeof data.id !== "number") {
    return;
  }
  const pending = pendingById.get(data.id);
  if (!pending) {
    return;
  }
  if (data.ok && (typeof data.png === "string" || typeof data.svg === "string") && data.width && data.height) {
    const entry: MermaidSvgEntry = {
      ...(typeof data.png === "string" ? { pngDataUrl: data.png } : {}),
      ...(typeof data.svg === "string" ? { xml: data.svg } : {}),
      widthPx: data.width,
      heightPx: data.height,
    };
    setCachedMermaidSvg(pending.theme, pending.source, entry);
    perfLog("mermaid.svg.render", {
      ms: Math.round(perfNow() - pending.enqueuedAt),
      bytes: entry.pngDataUrl?.length ?? entry.xml?.length ?? 0,
      kind: entry.pngDataUrl ? "png" : "svg",
      w: entry.widthPx,
      h: entry.heightPx,
    });
    settleRequest(data.id, entry);
    return;
  }
  // Cache failures too — a broken diagram should not retry on every mount.
  setCachedMermaidSvg(pending.theme, pending.source, null);
  perfLog("mermaid.svg.error", { error: data.error ?? "unknown" });
  settleRequest(data.id, null);
}

function handleServiceMessage(event: WebViewMessageEvent): void {
  let data: ServiceMessage;
  try {
    data = JSON.parse(event.nativeEvent.data);
  } catch {
    return;
  }
  if (data.type === "mermaid:service-ready" || data.type === "mermaid:pong") {
    if (!shellAlive) {
      perfLog("mermaid.service.ready");
    }
    shellAlive = true;
    flushQueuedRequests();
    return;
  }
  if (data.type === "mermaid:debug") {
    perfLog("mermaid.shell.debug", { id: data.id, phase: data.phase });
    return;
  }
  if (data.type !== "mermaid:svg") {
    return;
  }
  handleRenderResult(data);
}

function handleServiceLost(): void {
  shellAlive = false;
  serviceWebView = null;
  // Re-queue in-flight requests: a remounted host re-dispatches them from
  // here; the per-request timeout remains the failure backstop if no host
  // ever comes back.
  for (const [id, pending] of pendingById) {
    queuedBeforeReady.push({ id, source: pending.source, theme: pending.theme });
  }
}

/**
 * Render a mermaid diagram to display-ready SVG data. Cache hits resolve
 * synchronously via `peekMermaidSvg`; misses stream through the shared
 * service WebView.
 */
export function renderMermaidSvg(
  source: string,
  theme: string,
): Promise<MermaidSvgEntry | null> {
  const cached = getCachedMermaidSvg(theme, source);
  if (cached !== undefined) {
    return Promise.resolve(cached);
  }
  const cacheKey = mermaidCacheKey(theme, source);
  const inFlight = inFlightByKey.get(cacheKey);
  if (inFlight) {
    return inFlight;
  }
  const request = new Promise<MermaidSvgEntry | null>((resolve) => {
    const id = nextRequestId++;
    const timer = setTimeout(() => {
      perfLog("mermaid.svg.timeout", { id });
      settleRequest(id, null);
    }, REQUEST_TIMEOUT_MS);
    pendingById.set(id, { source, theme, resolve, enqueuedAt: perfNow(), timer });
    perfLog("mermaid.request", { id, ready: shellAlive, queued: !shellAlive });
    if (shellAlive && serviceWebView) {
      dispatchRequest(id, source, theme);
    } else {
      queuedBeforeReady.push({ id, source, theme });
    }
  });
  inFlightByKey.set(cacheKey, request);
  void request.finally(() => {
    inFlightByKey.delete(cacheKey);
  });
  return request;
}

/**
 * Mounts the shared render service. Must be inside the tree (any depth) for
 * `renderMermaidSvg` to make progress; render it once per screen that shows
 * markdown previews.
 */
export function MermaidSvgServiceHost(): React.JSX.Element {
  const webRef = useRef<WebView>(null);
  const mountAtRef = useRef(perfNow());
  const [shellEpoch, setShellEpoch] = React.useState(0);
  const { width: windowWidth } = useWindowDimensions();
  const shell = useMemo(() => {
    const buildStart = perfNow();
    // The multi-MB bundle string parses only when a mermaid diagram actually
    // exists on screen, never on the file pane's import path.
    const { MERMAID_MIN_JS } = require("./mermaid-assets") as typeof import("./mermaid-assets");
    const html = generateMermaidServiceShellHtml(MERMAID_MIN_JS);
    perfLog("mermaid.service.shellBuilt", {
      ms: Math.round(perfNow() - buildStart),
      bytes: html.length,
    });
    return html;
  }, []);

  useEffect(() => {
    serviceWebView = webRef.current;
    requestHostRemount = () => {
      perfLog("mermaid.service.remount");
      shellAlive = false;
      setShellEpoch((epoch) => epoch + 1);
    };
    return () => {
      requestHostRemount = () => undefined;
      handleServiceLost();
    };
  }, [shellEpoch]);

  // Watchdog: onLoadEnd only proves the document loaded, not that the inline
  // mermaid script survived (the renderer can die under memory pressure).
  // Probe the shell; remount the WebView when it stays silent.
  useEffect(() => {
    if (shellAlive) {
      return;
    }
    const probe = setTimeout(() => {
      webRef.current?.injectJavaScript("window.__mermaidPing && window.__mermaidPing(1); true;");
    }, 2000);
    const bail = setTimeout(() => {
      if (!shellAlive) {
        requestHostRemount();
      }
    }, 5000);
    return () => {
      clearTimeout(probe);
      clearTimeout(bail);
    };
  }, [shellEpoch]);

  return (
    <View
      style={[styles.host, { width: Math.max(windowWidth, 360) }]}
      pointerEvents="none"
      testID="mermaid-svg-service"
    >
      <WebView
        key={shellEpoch}
        ref={webRef}
        originWhitelist={["*"]}
        source={{ html: shell }}
        style={styles.webview}
        scrollEnabled={false}
        javaScriptEnabled
        onMessage={handleServiceMessage}
        onLoadEnd={() => {
          perfLog("mermaid.service.loaded", {
            ms: Math.round(perfNow() - mountAtRef.current),
            epoch: shellEpoch,
          });
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  host: {
    position: "absolute",
    left: 0,
    top: 0,
    height: 2,
    opacity: 0.01,
    overflow: "hidden",
  },
  webview: {
    flex: 1,
    backgroundColor: "transparent",
  },
});
