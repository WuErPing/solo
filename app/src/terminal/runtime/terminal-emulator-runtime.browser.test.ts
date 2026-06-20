import { page } from "@vitest/browser/context";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TerminalEmulatorRuntime } from "./terminal-emulator-runtime";

vi.mock("@xterm/addon-webgl", () => ({
  WebglAddon: class WebglAddon {
    activate(): void {}
    dispose(): void {}
    onContextLoss(): void {}
  },
}));

interface TerminalSize {
  rows: number;
  cols: number;
}

type BrowserTerminal = TerminalSize & {
  refresh: (start: number, end: number) => void;
  reset: () => void;
};

interface MountedTerminal {
  host: HTMLDivElement;
  root: HTMLDivElement;
  runtime: TerminalEmulatorRuntime;
  inputs: string[];
  sizes: TerminalSize[];
}

const mountedTerminals: MountedTerminal[] = [];

function nextFrame(): Promise<void> {
  return new Promise((resolve) => {
    requestAnimationFrame(() => {
      resolve();
    });
  });
}

async function waitFor(input: { predicate: () => boolean; timeoutMs?: number }): Promise<void> {
  const startedAt = performance.now();
  const timeoutMs = input.timeoutMs ?? 2_000;

  while (!input.predicate()) {
    if (performance.now() - startedAt > timeoutMs) {
      throw new Error("Timed out waiting for terminal browser condition");
    }
    await nextFrame();
  }
}

function createTerminalHost(input: {
  width: number;
  height: number;
  forceCols?: number;
  allowHorizontalScroll?: boolean;
}): MountedTerminal {
  const root = document.createElement("div");
  root.style.width = `${input.width}px`;
  root.style.height = `${input.height}px`;
  root.style.position = "fixed";
  root.style.left = "0";
  root.style.top = "0";
  root.style.overflow = "hidden";

  const host = document.createElement("div");
  host.style.width = "100%";
  host.style.height = "100%";
  root.appendChild(host);
  document.body.appendChild(root);

  const sizes: TerminalSize[] = [];
  const inputs: string[] = [];
  const runtime = new TerminalEmulatorRuntime();
  runtime.setCallbacks({
    callbacks: {
      onInput: (data) => {
        inputs.push(data);
      },
      onResize: (size) => {
        sizes.push(size);
      },
    },
  });
  runtime.mount({
    root,
    host,
    initialSnapshot: null,
    theme: {
      background: "#0b0b0b",
      foreground: "#e6e6e6",
      cursor: "#e6e6e6",
    },
    ...(input.forceCols != null ? { forceCols: input.forceCols } : {}),
    ...(input.allowHorizontalScroll != null ? { allowHorizontalScroll: input.allowHorizontalScroll } : {}),
  });

  const mounted = { host, root, runtime, inputs, sizes };
  mountedTerminals.push(mounted);
  return mounted;
}

function latestSize(sizes: TerminalSize[]): TerminalSize {
  const size = sizes.at(-1);
  if (!size) {
    throw new Error("Terminal did not report a size");
  }
  return size;
}

function getBrowserTerminal(): BrowserTerminal {
  const terminal = window.__soloTerminal as BrowserTerminal | undefined;
  if (!terminal) {
    throw new Error("Expected xterm to be exposed for browser test inspection");
  }
  return terminal;
}

afterEach(() => {
  for (const mounted of mountedTerminals.splice(0)) {
    mounted.runtime.unmount();
    mounted.root.remove();
  }
});

describe("terminal emulator runtime in a real browser", () => {
  it("reports a larger PTY size when the terminal container grows", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 360, height: 180 });

    await waitFor({ predicate: () => mounted.sizes.length > 0 });
    const initialSize = latestSize(mounted.sizes);

    mounted.root.style.width = "720px";
    mounted.root.style.height = "360px";
    await nextFrame();
    mounted.runtime.resize({ force: true });

    await waitFor({
      predicate: () => {
        const size = latestSize(mounted.sizes);
        return size.cols > initialSize.cols && size.rows > initialSize.rows;
      },
    });

    const grownSize = latestSize(mounted.sizes);
    expect(grownSize.cols).toBeGreaterThan(initialSize.cols);
    expect(grownSize.rows).toBeGreaterThan(initialSize.rows);
  });

  it("refreshes visible rows on a forced same-size resize", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 720, height: 360 });

    await waitFor({ predicate: () => mounted.sizes.length > 0 });

    const terminal = getBrowserTerminal();
    const refreshCalls: [number, number][] = [];
    const originalRefresh = terminal.refresh.bind(terminal);
    terminal.refresh = (start, end) => {
      refreshCalls.push([start, end]);
      originalRefresh(start, end);
    };

    mounted.runtime.resize({ force: true });

    await waitFor({ predicate: () => refreshCalls.length > 0 });
    expect(refreshCalls.at(-1)).toEqual([0, terminal.rows - 1]);
  });

  it.each([
    { name: "DA1", bytes: "\x1b[c" },
    { name: "DA1-zero", bytes: "\x1b[0c" },
    { name: "DA2", bytes: "\x1b[>c" },
    { name: "DA3", bytes: "\x1b[=c" },
    { name: "DSR-5", bytes: "\x1b[5n" },
    { name: "DSR-6", bytes: "\x1b[6n" },
    { name: "DSR-?6", bytes: "\x1b[?6n" },
    { name: "DECRQM", bytes: "\x1b[1$p" },
    { name: "DECRQM-?", bytes: "\x1b[?1$p" },
  ])("does not emit a PTY input reply for $name", async ({ bytes }) => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 720, height: 360 });

    await waitFor({ predicate: () => mounted.sizes.length > 0 });

    mounted.runtime.write({ text: bytes });
    await nextFrame();
    await nextFrame();

    expect(mounted.inputs).toEqual([]);
  });

  it("replays snapshots without synchronously resetting the visible terminal", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 720, height: 360 });

    await waitFor({ predicate: () => mounted.sizes.length > 0 });

    const terminal = getBrowserTerminal();
    const originalReset = terminal.reset.bind(terminal);
    const reset = vi.fn(originalReset);
    terminal.reset = reset;

    mounted.runtime.renderSnapshot({
      state: {
        rows: terminal.rows,
        cols: terminal.cols,
        scrollback: [],
        grid: [
          [
            { char: "p" },
            { char: "r" },
            { char: "o" },
            { char: "m" },
            { char: "p" },
            { char: "t" },
          ],
        ],
        cursor: {
          row: 0,
          col: 6,
        },
      },
    });
    await nextFrame();

    expect(reset).not.toHaveBeenCalled();
  });

  it("applies horizontal scroll viewport styles when allowHorizontalScroll is true", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 720, height: 360, allowHorizontalScroll: true });

    await waitFor({ predicate: () => mounted.sizes.length > 0 });

    const viewport = mounted.host.querySelector<HTMLElement>(".xterm-viewport");
    const screen = mounted.host.querySelector<HTMLElement>(".xterm-screen");

    expect(viewport).not.toBeNull();
    expect(screen).not.toBeNull();
    expect(viewport!.style.overflowX).toBe("auto");
    expect(viewport!.style.touchAction).toBe("pan-x pan-y");
    expect(screen!.style.minWidth).toBe("max-content");
  });

  it("restores viewport styles when allowHorizontalScroll is toggled off", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 720, height: 360, allowHorizontalScroll: true });

    await waitFor({ predicate: () => mounted.sizes.length > 0 });

    const viewport = mounted.host.querySelector<HTMLElement>(".xterm-viewport");
    const screen = mounted.host.querySelector<HTMLElement>(".xterm-screen");
    expect(viewport!.style.overflowX).toBe("auto");

    mounted.runtime.setAllowHorizontalScroll({ allowHorizontalScroll: false });
    await nextFrame();

    expect(viewport!.style.overflowX).toBe("hidden");
    expect(viewport!.style.touchAction).toBe("pan-y");
    expect(screen!.style.minWidth).toBe("");
  });

  it("resizes terminal to forced column count", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 360, height: 180, forceCols: 120 });

    await waitFor({
      predicate: () => {
        const size = latestSize(mounted.sizes);
        return size.cols === 120;
      },
    });

    const terminal = window.__soloTerminal as { cols: number } | undefined;
    expect(terminal).toBeDefined();
    expect(terminal!.cols).toBe(120);
  });

  it("expands host width proportionally when forceCols is set", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 360, height: 180, forceCols: 120 });

    await waitFor({
      predicate: () => {
        const size = latestSize(mounted.sizes);
        return size.cols === 120;
      },
    });

    const hostWidth = parseInt(mounted.host.style.width, 10);
    expect(hostWidth).toBeGreaterThan(360);
  });

  it("restores normal fit when forceCols is cleared", async () => {
    await page.viewport(900, 600);
    const mounted = createTerminalHost({ width: 360, height: 180, forceCols: 120 });

    await waitFor({
      predicate: () => {
        const size = latestSize(mounted.sizes);
        return size.cols === 120;
      },
    });

    expect(mounted.host.style.width).not.toBe("100%");

    mounted.runtime.setForceCols({});
    await nextFrame();

    expect(mounted.host.style.width).toBe("100%");
  });

});
