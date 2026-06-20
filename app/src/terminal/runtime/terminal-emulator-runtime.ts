import { ClipboardAddon } from "@xterm/addon-clipboard";
import { FitAddon } from "@xterm/addon-fit";
import { ImageAddon } from "@xterm/addon-image";
import { SearchAddon } from "@xterm/addon-search";
import { Unicode11Addon } from "@xterm/addon-unicode11";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import { LigaturesAddon } from "@xterm/addon-ligatures/lib/addon-ligatures.mjs";
import { Terminal, type ITheme } from "@xterm/xterm";
import type { TerminalState } from "@server/shared/messages";
import {
  type PendingTerminalModifiers,
  isTerminalModifierDomKey,
  mergeTerminalModifiers,
  normalizeDomTerminalKey,
  normalizeTerminalTransportKey,
  shouldInterceptDomTerminalKey,
} from "@/utils/terminal-keys";
import { renderTerminalSnapshotToAnsi } from "./terminal-snapshot";

export interface TerminalEmulatorRuntimeMountInput {
  root: HTMLDivElement;
  host: HTMLDivElement;
  initialSnapshot: TerminalState | null;
  theme: ITheme;
  convertEol?: boolean;
  forceCols?: number;
  allowHorizontalScroll?: boolean;
}

export interface TerminalEmulatorRuntimeCallbacks {
  onInput?: (data: string) => Promise<void> | void;
  onResize?: (input: { rows: number; cols: number }) => Promise<void> | void;
  onTerminalKey?: (input: {
    key: string;
    ctrl: boolean;
    shift: boolean;
    alt: boolean;
    meta: boolean;
  }) => Promise<void> | void;
  onPendingModifiersConsumed?: () => Promise<void> | void;
  onOpenExternalUrl?: (url: string) => Promise<void> | void;
}

interface TerminalEmulatorRuntimeDisposables {
  disposeInput: () => void;
  disconnectResizeObserver: () => void;
  removeWindowResize: () => void;
  removeWindowFocus: () => void;
  removeDocumentVisibilityChange: () => void;
  removeVisualViewportResize: () => void;
  clearFitInterval: () => void;
  clearFitTimeouts: () => void;
  removeFontListeners: () => void;
  removeTouchListeners: () => void;
  restoreDocumentStyles: () => void;
  restoreViewportStyles: () => void;
  disposeFitAddon: () => void;
  disposeWebglAddon: () => void;
  disposeTerminal: () => void;
}

interface TerminalOutputOperation {
  type: "write" | "clear" | "snapshot";
  text: string;
  rows?: number;
  cols?: number;
  suppressInput?: boolean;
  onCommitted?: () => void;
}

declare global {
  interface Window {
    __soloTerminal?: Terminal;
  }
}

const isMac =
  typeof navigator !== "undefined" &&
  (/Macintosh|Mac OS/i.test(navigator.userAgent ?? "") ||
    /Mac/i.test((navigator as Navigator & { platform?: string }).platform ?? ""));

const DEFAULT_TOUCH_SCROLL_LINE_HEIGHT_PX = 18;
const FIT_TIMEOUT_DELAYS_MS = [0, 16, 48, 120, 250, 500, 1_000, 2_000];
const OUTPUT_OPERATION_TIMEOUT_MS = 5_000;
const RESET_TERMINAL_ANSI = "\u001bc";

const DEFAULT_TERMINAL_FONT_FAMILY = [
  // Prefer common developer fonts, with Nerd Font variants for prompt/TUI glyphs.
  "JetBrains Mono",
  "JetBrainsMono Nerd Font",
  "JetBrainsMono NF",
  "MesloLGM Nerd Font",
  "MesloLGM NF",
  "Hack Nerd Font",
  "FiraCode Nerd Font",
  // PUA-only fallback (many Nerd glyphs live here on some systems).
  "Symbols Nerd Font",
  // System fallbacks.
  "SF Mono",
  "Menlo",
  "Monaco",
  "Consolas",
  "'Liberation Mono'",
  "monospace",
].join(", ");

// Approximate average glyph advance widths used to pre-fit the terminal on
// mobile before xterm has measured its actual cell size. Values are in px for
// a 1px font-size and must stay conservative (slightly wider than real glyphs)
// so that capture-pane width does not exceed the visible columns.
const AVG_GLYPH_ADVANCE_X = 0.78;
const AVG_CJK_GLYPH_ADVANCE_X = 1.15;

function computeAdaptiveFontSize(containerWidthPx: number): number {
  // Keep the terminal readable on tablets/desktop, but shrink on phones so
  // that ASCII box-drawing tables fit within the viewport.
  if (containerWidthPx >= 900) return 14;
  if (containerWidthPx >= 600) return 13;
  if (containerWidthPx >= 400) return 11;
  return 10;
}

function estimateCols(containerWidthPx: number, fontSize: number): number {
  // Blend average advances: assume ~15% wide glyphs (CJK/box) and the rest
  // latin/monospace. Reserve generous space for the scrollbar and safety
  // margin so the captured width never exceeds the visible columns.
  const blendedAdvance = AVG_GLYPH_ADVANCE_X * 0.85 + AVG_CJK_GLYPH_ADVANCE_X * 0.15;
  const cellWidth = fontSize * blendedAdvance;
  const usableWidth = Math.max(1, containerWidthPx - 32);
  return Math.max(1, Math.floor(usableWidth / cellWidth));
}

function withOverviewRulerBorderHidden(theme: ITheme): ITheme {
  return {
    ...theme,
    overviewRulerBorder: theme.background ?? "transparent",
  };
}

export class TerminalEmulatorRuntime {
  private callbacks: TerminalEmulatorRuntimeCallbacks = {};
  private pendingModifiers: PendingTerminalModifiers = {
    ctrl: false,
    shift: false,
    alt: false,
  };
  private terminal: Terminal | null = null;
  private fitAddon: FitAddon | null = null;
  private fitAndEmitResize: ((force: boolean) => void) | null = null;
  private lastSize: { rows: number; cols: number } | null = null;
  private cleanup: (() => void) | null = null;
  private outputOperations: TerminalOutputOperation[] = [];
  private inFlightOutputOperation: TerminalOutputOperation | null = null;
  private inFlightOutputOperationTimeout: ReturnType<typeof setTimeout> | null = null;
  private suppressInput = false;
  private forceCols: number | undefined = undefined;
  private allowHorizontalScroll = false;
  private restoreViewportStyles: (() => void) | null = null;

  private handleVisibilityRestore = (): void => {
    if (typeof document !== "undefined" && document.visibilityState !== "visible") {
      return;
    }

    this.fitAndEmitResize?.(true);
    if (typeof window.requestAnimationFrame === "function") {
      window.requestAnimationFrame(() => {
        this.fitAndEmitResize?.(true);
      });
    }
  };

  setCallbacks(input: { callbacks: TerminalEmulatorRuntimeCallbacks }): void {
    this.callbacks = input.callbacks;
  }

  setPendingModifiers(input: { pendingModifiers: PendingTerminalModifiers }): void {
    this.pendingModifiers = input.pendingModifiers;
  }

  setForceCols(input: { forceCols?: number }): void {
    const next = typeof input.forceCols === "number" && input.forceCols > 0 ? input.forceCols : undefined;
    if (this.forceCols === next) {
      return;
    }
    this.forceCols = next;
    this.fitAndEmitResize?.(true);
  }

  setAllowHorizontalScroll(input: { allowHorizontalScroll: boolean }): void {
    if (this.allowHorizontalScroll === input.allowHorizontalScroll) {
      return;
    }
    this.allowHorizontalScroll = input.allowHorizontalScroll;
    this.restoreViewportStyles?.();
    const host = this.terminal?.element?.parentElement as HTMLDivElement | null;
    if (host) {
      this.restoreViewportStyles = this.applyViewportTouchStyles({ host, allowHorizontalScroll: this.allowHorizontalScroll });
    }
    this.fitAndEmitResize?.(true);
  }

  mount(input: TerminalEmulatorRuntimeMountInput): void {
    this.unmount();

    input.host.innerHTML = "";
    this.lastSize = null;

    const rootWidth = input.root.offsetWidth;
    const initialFontSize = computeAdaptiveFontSize(rootWidth);
    const terminal = new Terminal({
      allowProposedApi: true,
      convertEol: input.convertEol ?? false,
      cursorBlink: true,
      cursorStyle: "bar",
      customGlyphs: true,
      fontFamily: DEFAULT_TERMINAL_FONT_FAMILY,
      fontSize: initialFontSize,
      lineHeight: 1.0,
      macOptionIsMeta: true,
      minimumContrastRatio: 1,
      rescaleOverlappingGlyphs: true,
      overviewRuler: {
        width: 8,
      },
      scrollback: 10_000,
      theme: withOverviewRulerBorderHidden(input.theme),
    });
    const fitAddon = new FitAddon();
    const unicode11Addon = new Unicode11Addon();
    let webglAddon: WebglAddon | null = null;
    let imageAddon: ImageAddon | null = null;
    terminal.loadAddon(fitAddon);
    terminal.loadAddon(unicode11Addon);
    terminal.loadAddon(
      new WebLinksAddon((event, uri) => {
        event.preventDefault();
        void this.callbacks.onOpenExternalUrl?.(uri);
      }),
    );
    terminal.loadAddon(new SearchAddon({ highlightLimit: 20_000 }));
    terminal.loadAddon(new ClipboardAddon());
    try {
      terminal.loadAddon(new LigaturesAddon());
    } catch {
      // Ligatures require Font Access API or compatible environment
    }
    terminal.open(input.host);
    try {
      terminal.unicode.activeVersion = "11";
    } catch {
      // Ignore if unicode API isn't available in this build/runtime.
    }

    const disposeImageAddon = (): void => {
      try {
        imageAddon?.dispose();
      } catch {
        // ignore
      }
      imageAddon = null;
    };
    const disposeWebglRenderer = (): void => {
      if (!webglAddon) {
        return;
      }
      try {
        webglAddon.dispose();
      } catch {
        // ignore
      }
      webglAddon = null;
      disposeImageAddon();
      // WebGL and DOM renderers can have different cell dimensions.
      this.fitAndEmitResize?.(true);
    };

    // Browser xterm is a renderer only; it never replies to terminal protocol queries.
    // Replies live on the daemon (one process boundary from the PTY) so they arrive
    // before the foreground app exits, instead of racing back over the websocket.
    // Re-registered after the image addon loads so our handlers stay last in the
    // LIFO dispatch (the image addon registers its own {final:"c"} for sixel DA1).
    const registerProtocolQuerySuppression = (): void => {
      terminal.parser.registerCsiHandler({ final: "c" }, () => true);
      terminal.parser.registerCsiHandler({ prefix: ">", final: "c" }, () => true);
      terminal.parser.registerCsiHandler({ prefix: "=", final: "c" }, () => true);
      terminal.parser.registerCsiHandler({ final: "n" }, () => true);
      terminal.parser.registerCsiHandler({ prefix: "?", final: "n" }, () => true);
      terminal.parser.registerCsiHandler({ final: "R" }, () => true);
      terminal.parser.registerCsiHandler({ intermediates: "$", final: "p" }, () => true);
      terminal.parser.registerCsiHandler(
        { prefix: "?", intermediates: "$", final: "p" },
        () => true,
      );
    };
    registerProtocolQuerySuppression();

    const isLikelyMobile = input.root.offsetWidth > 0 && input.root.offsetWidth < 600;
    let webglAddonRaf: number | null = null;
    if (!isLikelyMobile) {
      webglAddonRaf = requestAnimationFrame(() => {
        webglAddonRaf = null;
        try {
          disposeWebglRenderer();
          webglAddon = new WebglAddon();
          webglAddon.onContextLoss(() => {
            disposeWebglRenderer();
          });
          terminal.loadAddon(webglAddon);
          imageAddon = new ImageAddon();
          terminal.loadAddon(imageAddon);
          registerProtocolQuerySuppression();
          this.fitAndEmitResize?.(true);
        } catch {
          disposeWebglRenderer();
        }
      });
    }

    const restoreDocumentStyles = this.applyDocumentBoundsStyles({
      root: input.root,
    });
    this.allowHorizontalScroll = input.allowHorizontalScroll ?? false;
    this.forceCols = typeof input.forceCols === "number" && input.forceCols > 0 ? input.forceCols : undefined;
    this.restoreViewportStyles = this.applyViewportTouchStyles({
      host: input.host,
      allowHorizontalScroll: this.allowHorizontalScroll,
    });

    this.terminal = terminal;
    this.fitAddon = fitAddon;
    window.__soloTerminal = terminal;

    const fitAndEmitResize = (force: boolean): void => {
      const currentTerminal = this.terminal;
      const currentFitAddon = this.fitAddon;
      if (!currentTerminal || !currentFitAddon) {
        return;
      }

      const currentRootWidth = input.root.offsetWidth;
      const currentRootHeight = input.root.offsetHeight;
      if (currentRootWidth === 0 || currentRootHeight === 0) {
        return;
      }

      const nextFontSize = computeAdaptiveFontSize(currentRootWidth);
      if (currentTerminal.options.fontSize !== nextFontSize) {
        try {
          currentTerminal.options.fontSize = nextFontSize;
        } catch {
          // ignore
        }
      }

      let nextRows = currentTerminal.rows;
      let nextCols = currentTerminal.cols;
      const forcedCols = this.forceCols;
      if (typeof forcedCols === "number" && forcedCols > 0) {
        // First fit to the container so we can measure the real cell width at
        // the current font size, then expand the host to the desired width.
        try {
          currentFitAddon.fit();
        } catch {
          return;
        }
        const fittedCols = currentTerminal.cols;
        const fittedElementWidth = currentTerminal.element?.getBoundingClientRect().width ?? 0;
        const cellWidth = fittedCols > 0 && fittedElementWidth > 0 ? fittedElementWidth / fittedCols : currentRootWidth / Math.max(1, fittedCols);
        const desiredHostWidth = Math.max(1, Math.ceil(cellWidth * forcedCols));
        input.host.style.width = `${desiredHostWidth}px`;
        input.host.style.flex = "0 0 auto";
        input.root.style.minWidth = `${desiredHostWidth}px`;
        input.root.style.width = "auto";
        try {
          currentTerminal.resize(forcedCols, currentTerminal.rows);
          nextRows = currentTerminal.rows;
          nextCols = currentTerminal.cols;
        } catch {
          // fall through to fit()
          try {
            currentFitAddon.fit();
          } catch {
            return;
          }
          nextRows = currentTerminal.rows;
          nextCols = currentTerminal.cols;
        }
      } else {
        input.host.style.width = "100%";
        input.host.style.flex = "1";
        input.root.style.minWidth = "";
        input.root.style.width = "100%";
        try {
          currentFitAddon.fit();
        } catch {
          return;
        }
        nextRows = currentTerminal.rows;
        nextCols = currentTerminal.cols;
      }

      const previous = this.lastSize;
      if (!force && previous && previous.rows === nextRows && previous.cols === nextCols) {
        return;
      }

      this.lastSize = { rows: nextRows, cols: nextCols };
      this.refreshVisibleRows();
      this.callbacks.onResize?.({
        rows: nextRows,
        cols: nextCols,
      });
    };
    this.fitAndEmitResize = fitAndEmitResize;

    fitAndEmitResize(true);

    const inputDisposable = terminal.onData((data) => {
      if (this.suppressInput) {
        return;
      }
      this.callbacks.onInput?.(data);
    });

    terminal.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown" || event.isComposing) {
        return true;
      }

      if (!isMac && event.ctrlKey && !event.shiftKey && !event.altKey && !event.metaKey) {
        const key = event.key.toLowerCase();

        // Ctrl+C: copy selection to clipboard if text is selected, otherwise let xterm send SIGINT
        if (key === "c" && terminal.hasSelection()) {
          void navigator.clipboard.writeText(terminal.getSelection());
          return false;
        }

        // Ctrl+V: paste from clipboard into terminal
        if (key === "v") {
          event.preventDefault();
          void navigator.clipboard.readText().then((text) => {
            if (text) {
              terminal.paste(text);
            }
            return;
          });
          return false;
        }

        return true;
      }

      const normalizedKey = normalizeDomTerminalKey(event.key);
      if (!normalizedKey || isTerminalModifierDomKey(event.key)) {
        return true;
      }

      if (
        !shouldInterceptDomTerminalKey({
          key: normalizedKey,
          ctrlKey: event.ctrlKey,
          shiftKey: event.shiftKey,
          altKey: event.altKey,
          metaKey: event.metaKey,
          pendingModifiers: this.pendingModifiers,
        })
      ) {
        return true;
      }

      const modifiers = mergeTerminalModifiers({
        pendingModifiers: this.pendingModifiers,
        ctrlKey: event.ctrlKey,
        shiftKey: event.shiftKey,
        altKey: event.altKey,
        metaKey: event.metaKey,
      });
      this.callbacks.onTerminalKey?.({
        key: normalizeTerminalTransportKey(normalizedKey),
        ...modifiers,
      });

      if (this.pendingModifiers.ctrl || this.pendingModifiers.shift || this.pendingModifiers.alt) {
        this.callbacks.onPendingModifiersConsumed?.();
      }

      event.preventDefault();
      event.stopPropagation();
      return false;
    });

    const removeTouchListeners = this.setupTouchScrollHandlers({
      root: input.root,
      host: input.host,
      terminal,
    });
    const resizeObserver = new ResizeObserver(() => {
      fitAndEmitResize(false);
    });
    resizeObserver.observe(input.root);
    resizeObserver.observe(input.host);

    const windowResizeHandler = () => fitAndEmitResize(false);
    window.addEventListener("resize", windowResizeHandler);
    const windowFocusHandler = () => {
      this.handleVisibilityRestore();
    };
    window.addEventListener("focus", windowFocusHandler);

    const documentVisibilityChangeHandler = () => {
      this.handleVisibilityRestore();
    };
    document.addEventListener("visibilitychange", documentVisibilityChangeHandler);

    const visualViewport = window.visualViewport;
    const visualViewportResizeHandler = () => fitAndEmitResize(false);
    visualViewport?.addEventListener("resize", visualViewportResizeHandler);

    const fitInterval = window.setInterval(() => {
      fitAndEmitResize(false);
    }, 250);
    const fitTimeouts = FIT_TIMEOUT_DELAYS_MS.map((delayMs) =>
      window.setTimeout(() => {
        fitAndEmitResize(true);
      }, delayMs),
    );

    const fontSet = document.fonts;
    const fontReadyHandler = () => {
      fitAndEmitResize(true);
    };
    fontSet?.addEventListener?.("loadingdone", fontReadyHandler);
    void fontSet?.ready
      .then(() => {
        fitAndEmitResize(true);
        return;
      })
      .catch(() => {
        // no-op
      });

    window.setTimeout(() => {
      fitAndEmitResize(true);
    }, 0);

    if (input.initialSnapshot) {
      this.renderSnapshot({ state: input.initialSnapshot });
    }

    this.processOutputQueue();

    const disposables: TerminalEmulatorRuntimeDisposables = {
      disposeInput: () => {
        inputDisposable.dispose();
      },
      disconnectResizeObserver: () => {
        resizeObserver.disconnect();
      },
      removeWindowResize: () => {
        window.removeEventListener("resize", windowResizeHandler);
      },
      removeWindowFocus: () => {
        window.removeEventListener("focus", windowFocusHandler);
      },
      removeDocumentVisibilityChange: () => {
        document.removeEventListener("visibilitychange", documentVisibilityChangeHandler);
      },
      removeVisualViewportResize: () => {
        visualViewport?.removeEventListener("resize", visualViewportResizeHandler);
      },
      clearFitInterval: () => {
        window.clearInterval(fitInterval);
      },
      clearFitTimeouts: () => {
        for (const handle of fitTimeouts) {
          window.clearTimeout(handle);
        }
      },
      removeFontListeners: () => {
        fontSet?.removeEventListener?.("loadingdone", fontReadyHandler);
      },
      removeTouchListeners,
      restoreDocumentStyles,
      restoreViewportStyles: () => {
        this.restoreViewportStyles?.();
        this.restoreViewportStyles = null;
      },
      disposeFitAddon: () => {
        fitAddon.dispose();
      },
      disposeWebglAddon: () => {
        if (webglAddonRaf !== null) {
          cancelAnimationFrame(webglAddonRaf);
          webglAddonRaf = null;
        }
        disposeWebglRenderer();
        disposeImageAddon();
      },
      disposeTerminal: () => {
        terminal.dispose();
      },
    };

    this.cleanup = () => {
      disposables.disposeInput();
      disposables.disconnectResizeObserver();
      disposables.removeWindowResize();
      disposables.removeWindowFocus();
      disposables.removeDocumentVisibilityChange();
      disposables.removeVisualViewportResize();
      disposables.clearFitInterval();
      disposables.clearFitTimeouts();
      disposables.removeFontListeners();
      disposables.removeTouchListeners();
      disposables.disposeFitAddon();
      disposables.disposeWebglAddon();
      disposables.disposeTerminal();
      disposables.restoreDocumentStyles();
      disposables.restoreViewportStyles();
    };
  }

  write(input: { text: string; suppressInput?: boolean; onCommitted?: () => void }): void {
    if (input.text.length === 0) {
      input.onCommitted?.();
      return;
    }
    this.outputOperations.push({
      type: "write",
      text: input.text,
      suppressInput: input.suppressInput ?? false,
      ...(input.onCommitted ? { onCommitted: input.onCommitted } : {}),
    });
    this.processOutputQueue();
  }

  clear(input?: { onCommitted?: () => void }): void {
    this.outputOperations.push({
      type: "clear",
      text: "",
      suppressInput: false,
      ...(input?.onCommitted ? { onCommitted: input.onCommitted } : {}),
    });
    this.processOutputQueue();
  }

  renderSnapshot(input: { state: TerminalState | null; onCommitted?: () => void }): void {
    if (!input.state) {
      this.clear(input);
      return;
    }
    this.outputOperations.push({
      type: "snapshot",
      text: `${RESET_TERMINAL_ANSI}${renderTerminalSnapshotToAnsi(input.state)}`,
      rows: input.state.rows,
      cols: input.state.cols,
      suppressInput: true,
      ...(input.onCommitted ? { onCommitted: input.onCommitted } : {}),
    });
    this.processOutputQueue();
  }

  resize(input?: { force?: boolean }): void {
    this.fitAndEmitResize?.(input?.force ?? false);
  }

  setTheme(input: { theme: ITheme }): void {
    const terminal = this.terminal;
    if (!terminal) {
      return;
    }

    try {
      terminal.options.theme = withOverviewRulerBorderHidden(input.theme);
    } catch {
      // ignore
      return;
    }

    this.refreshVisibleRows();
  }

  focus(): void {
    this.terminal?.focus();
  }

  private refreshVisibleRows(): void {
    const terminal = this.terminal;
    if (!terminal || terminal.rows <= 0) {
      return;
    }

    try {
      terminal.refresh(0, terminal.rows - 1);
    } catch {
      // ignore
    }
  }

  unmount(): void {
    this.clearInFlightOutputTimeout();
    const inFlightOperation = this.inFlightOutputOperation;
    this.inFlightOutputOperation = null;
    if (inFlightOperation?.onCommitted) {
      inFlightOperation.onCommitted();
    }
    const pendingOperations = this.outputOperations.splice(0, this.outputOperations.length);
    for (const operation of pendingOperations) {
      operation.onCommitted?.();
    }

    this.cleanup?.();
    this.cleanup = null;
    if (window.__soloTerminal === this.terminal) {
      window.__soloTerminal = undefined;
    }
    this.terminal = null;
    this.fitAddon = null;
    this.fitAndEmitResize = null;
    this.lastSize = null;
    this.suppressInput = false;
    this.forceCols = undefined;
    this.allowHorizontalScroll = false;
    this.restoreViewportStyles = null;
  }

  private processOutputQueue(): void {
    if (this.inFlightOutputOperation) {
      return;
    }

    const terminal = this.terminal;
    if (!terminal) {
      return;
    }

    const operation = this.outputOperations.shift();
    if (!operation) {
      return;
    }

    this.inFlightOutputOperation = operation;
    const previousSuppressInput = this.suppressInput;
    if (operation.type === "write") {
      this.suppressInput = Boolean(operation.suppressInput);
    }
    const finalizeOperation = (expectedOperation: TerminalOutputOperation) => {
      if (this.inFlightOutputOperation !== expectedOperation) {
        return;
      }
      this.inFlightOutputOperation = null;
      this.clearInFlightOutputTimeout();
      this.suppressInput = previousSuppressInput;
      expectedOperation.onCommitted?.();
      this.processOutputQueue();
    };

    if (operation.type === "clear") {
      terminal.reset();
      finalizeOperation(operation);
      return;
    }

    if (operation.type === "snapshot") {
      try {
        if (
          typeof operation.cols === "number" &&
          typeof operation.rows === "number" &&
          (terminal.cols !== operation.cols || terminal.rows !== operation.rows)
        ) {
          terminal.resize(operation.cols, operation.rows);
        }
      } catch {
        finalizeOperation(operation);
        return;
      }
    }

    const text = operation.text;
    this.inFlightOutputOperationTimeout = setTimeout(() => {
      finalizeOperation(operation);
    }, OUTPUT_OPERATION_TIMEOUT_MS);

    try {
      terminal.write(text, () => {
        finalizeOperation(operation);
      });
    } catch {
      finalizeOperation(operation);
    }
  }

  private clearInFlightOutputTimeout(): void {
    if (!this.inFlightOutputOperationTimeout) {
      return;
    }
    clearTimeout(this.inFlightOutputOperationTimeout);
    this.inFlightOutputOperationTimeout = null;
  }

  estimateCols(input: { containerWidthPx: number }): number {
    const fontSize = computeAdaptiveFontSize(input.containerWidthPx);
    return estimateCols(input.containerWidthPx, fontSize);
  }

  private applyDocumentBoundsStyles(input: { root: HTMLDivElement }): () => void {
    const documentElement = document.documentElement;
    const body = document.body;
    const rootContainer = input.root.parentElement;

    const previousDocumentElementOverflow = documentElement.style.overflow;
    const previousDocumentElementWidth = documentElement.style.width;
    const previousDocumentElementHeight = documentElement.style.height;
    const previousDocumentElementTextSizeAdjust =
      documentElement.style.getPropertyValue("text-size-adjust");
    const previousDocumentElementWebkitTextSizeAdjust = documentElement.style.getPropertyValue(
      "-webkit-text-size-adjust",
    );

    const previousBodyOverflow = body.style.overflow;
    const previousBodyWidth = body.style.width;
    const previousBodyHeight = body.style.height;
    const previousBodyMargin = body.style.margin;
    const previousBodyPadding = body.style.padding;
    const previousBodyTextSizeAdjust = body.style.getPropertyValue("text-size-adjust");
    const previousBodyWebkitTextSizeAdjust = body.style.getPropertyValue(
      "-webkit-text-size-adjust",
    );

    const previousRootOverflow = rootContainer?.style.overflow ?? "";
    const previousRootWidth = rootContainer?.style.width ?? "";
    const previousRootHeight = rootContainer?.style.height ?? "";

    documentElement.style.overflow = "hidden";
    documentElement.style.width = "100%";
    documentElement.style.height = "100%";
    documentElement.style.setProperty("text-size-adjust", "100%");
    documentElement.style.setProperty("-webkit-text-size-adjust", "100%");

    body.style.overflow = "hidden";
    body.style.width = "100%";
    body.style.height = "100%";
    body.style.margin = "0";
    body.style.padding = "0";
    body.style.setProperty("text-size-adjust", "100%");
    body.style.setProperty("-webkit-text-size-adjust", "100%");

    if (rootContainer) {
      rootContainer.style.overflow = "hidden";
      rootContainer.style.width = "100%";
      rootContainer.style.height = "100%";
    }

    return () => {
      documentElement.style.overflow = previousDocumentElementOverflow;
      documentElement.style.width = previousDocumentElementWidth;
      documentElement.style.height = previousDocumentElementHeight;
      documentElement.style.setProperty("text-size-adjust", previousDocumentElementTextSizeAdjust);
      documentElement.style.setProperty(
        "-webkit-text-size-adjust",
        previousDocumentElementWebkitTextSizeAdjust,
      );

      body.style.overflow = previousBodyOverflow;
      body.style.width = previousBodyWidth;
      body.style.height = previousBodyHeight;
      body.style.margin = previousBodyMargin;
      body.style.padding = previousBodyPadding;
      body.style.setProperty("text-size-adjust", previousBodyTextSizeAdjust);
      body.style.setProperty("-webkit-text-size-adjust", previousBodyWebkitTextSizeAdjust);

      if (rootContainer) {
        rootContainer.style.overflow = previousRootOverflow;
        rootContainer.style.width = previousRootWidth;
        rootContainer.style.height = previousRootHeight;
      }
    };
  }

  private applyViewportTouchStyles(input: { host: HTMLDivElement; allowHorizontalScroll?: boolean }): () => void {
    const viewportElement = input.host.querySelector<HTMLElement>(".xterm-viewport");
    const screenElement = input.host.querySelector<HTMLElement>(".xterm-screen");

    const previousViewportOverscroll = viewportElement?.style.overscrollBehavior ?? "";
    const previousViewportTouchAction = viewportElement?.style.touchAction ?? "";
    const previousViewportOverflowY = viewportElement?.style.overflowY ?? "";
    const previousViewportOverflowX = viewportElement?.style.overflowX ?? "";
    const previousViewportPointerEvents = viewportElement?.style.pointerEvents ?? "";
    const previousViewportWebkitOverflowScrolling =
      viewportElement?.style.getPropertyValue("-webkit-overflow-scrolling") ?? "";
    const previousScreenMinWidth = screenElement?.style.minWidth ?? "";
    const allowHorizontalScroll = input.allowHorizontalScroll ?? false;
    if (viewportElement) {
      viewportElement.style.overscrollBehavior = "none";
      viewportElement.style.touchAction = allowHorizontalScroll ? "pan-x pan-y" : "pan-y";
      viewportElement.style.overflowY = "auto";
      viewportElement.style.overflowX = allowHorizontalScroll ? "auto" : "hidden";
      viewportElement.style.pointerEvents = "auto";
      viewportElement.style.setProperty("-webkit-overflow-scrolling", "touch");
    }
    if (screenElement && allowHorizontalScroll) {
      // Prevent xterm from collapsing the screen to the viewport width when
      // horizontal scrolling is enabled; the screen should stay at its
      // intrinsic cols*cellWidth size.
      screenElement.style.minWidth = "max-content";
    }

    return () => {
      if (viewportElement) {
        viewportElement.style.overscrollBehavior = previousViewportOverscroll;
        viewportElement.style.touchAction = previousViewportTouchAction;
        viewportElement.style.overflowY = previousViewportOverflowY;
        viewportElement.style.overflowX = previousViewportOverflowX;
        viewportElement.style.pointerEvents = previousViewportPointerEvents;
        viewportElement.style.setProperty(
          "-webkit-overflow-scrolling",
          previousViewportWebkitOverflowScrolling,
        );
      }
      if (screenElement) {
        screenElement.style.minWidth = previousScreenMinWidth;
      }
    };
  }

  private setupTouchScrollHandlers(input: {
    root: HTMLDivElement;
    host: HTMLDivElement;
    terminal: Terminal;
  }): () => void {
    let touchScrollRemainderPx = 0;
    const measuredLineHeight =
      input.host.querySelector<HTMLElement>(".xterm-rows > div")?.getBoundingClientRect().height ??
      0;
    const touchScrollLineHeightPx =
      measuredLineHeight > 0 ? measuredLineHeight : DEFAULT_TOUCH_SCROLL_LINE_HEIGHT_PX;

    const activeTouch = {
      identifier: -1,
      startX: 0,
      startY: 0,
      lastX: 0,
      lastY: 0,
      mode: null as "vertical" | "horizontal" | null,
    };

    const touchStartHandler = (event: TouchEvent) => {
      if (event.touches.length !== 1) {
        touchScrollRemainderPx = 0;
        activeTouch.identifier = -1;
        activeTouch.mode = null;
        return;
      }

      const touch = event.touches[0];
      if (!touch) {
        touchScrollRemainderPx = 0;
        activeTouch.identifier = -1;
        activeTouch.mode = null;
        return;
      }

      activeTouch.identifier = touch.identifier;
      activeTouch.startX = touch.clientX;
      activeTouch.startY = touch.clientY;
      activeTouch.lastX = touch.clientX;
      activeTouch.lastY = touch.clientY;
      activeTouch.mode = null;
      touchScrollRemainderPx = 0;
    };

    const touchMoveHandler = (event: TouchEvent) => {
      if (event.touches.length !== 1) {
        return;
      }

      const touch = Array.from(event.touches).find(
        (candidate) => candidate.identifier === activeTouch.identifier,
      );
      if (!touch) {
        return;
      }

      const totalDeltaX = touch.clientX - activeTouch.startX;
      const totalDeltaY = touch.clientY - activeTouch.startY;
      if (activeTouch.mode === null) {
        const absX = Math.abs(totalDeltaX);
        const absY = Math.abs(totalDeltaY);
        if (absX > 8 || absY > 8) {
          activeTouch.mode = absY >= absX ? "vertical" : "horizontal";
        }
      }

      activeTouch.lastX = touch.clientX;
      activeTouch.lastY = touch.clientY;

      if (activeTouch.mode === "horizontal") {
        // In original-width mode let the browser/xterm viewport handle
        // horizontal panning natively.
        return;
      }

      if (activeTouch.mode !== "vertical") {
        return;
      }

      const deltaY = touch.clientY - activeTouch.lastY;
      touchScrollRemainderPx += deltaY;
      const lineDelta = Math.trunc(touchScrollRemainderPx / touchScrollLineHeightPx);
      if (lineDelta !== 0) {
        input.terminal.scrollLines(-lineDelta);
        touchScrollRemainderPx -= lineDelta * touchScrollLineHeightPx;
      }

      event.preventDefault();
    };

    const touchEndHandler = (event: TouchEvent) => {
      const activeTouchEnded = Array.from(event.changedTouches).some(
        (touch) => touch.identifier === activeTouch.identifier,
      );
      if (activeTouchEnded || event.touches.length === 0) {
        touchScrollRemainderPx = 0;
        activeTouch.identifier = -1;
        activeTouch.mode = null;
      }
    };

    const touchCancelHandler = () => {
      touchScrollRemainderPx = 0;
      activeTouch.identifier = -1;
      activeTouch.mode = null;
    };

    input.root.addEventListener("touchstart", touchStartHandler, { passive: true });
    input.root.addEventListener("touchmove", touchMoveHandler, { passive: false });
    input.root.addEventListener("touchend", touchEndHandler, { passive: true });
    input.root.addEventListener("touchcancel", touchCancelHandler, { passive: true });

    return () => {
      input.root.removeEventListener("touchstart", touchStartHandler);
      input.root.removeEventListener("touchmove", touchMoveHandler);
      input.root.removeEventListener("touchend", touchEndHandler);
      input.root.removeEventListener("touchcancel", touchCancelHandler);
    };
  }
}
