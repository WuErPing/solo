import type { Page } from "@playwright/test";
import { test, expect } from "./fixtures";
import { TerminalE2EHarness, withTerminalInApp } from "./helpers/terminal-dsl";
import { getTabTestIds } from "./helpers/launcher";
import {
  installTerminalRenderProbe,
  readTerminalRenderProbe,
  resetTerminalRenderProbe,
  startTerminalFrameSampling,
  summarizeTerminalRenderProbe,
} from "./helpers/terminal-probes";
import { getTerminalBufferText, waitForTerminalContent } from "./helpers/terminal-perf";

interface TerminalLayoutMetrics {
  visibleSurfaceCount: number;
  surfaceHeight: number;
  rows: number | null;
  cols: number | null;
  /** Non-empty lines in the active xterm buffer (renderer-agnostic). */
  nonEmptyRowCount: number;
  tabIds: string[];
}

async function readTerminalLayoutMetrics(page: Page): Promise<TerminalLayoutMetrics> {
  const tabIds = await getTabTestIds(page);
  return page.evaluate((currentTabIds) => {
    const visibleSurfaces = Array.from(
      document.querySelectorAll<HTMLElement>('[data-testid="terminal-surface"]'),
    ).filter((candidate) => {
      const rect = candidate.getBoundingClientRect();
      return rect.width > 0 && rect.height > 0;
    });
    const surface = visibleSurfaces[0] ?? null;
    const surfaceRect = surface?.getBoundingClientRect() ?? null;
    const term = (
      window as Window & {
        __soloTerminal?: {
          rows?: number;
          cols?: number;
          buffer?: {
            active: {
              length: number;
              getLine: (i: number) => { translateToString: (trim: boolean) => string } | null;
            };
          };
        };
      }
    ).__soloTerminal;

    // The app renders terminals with the WebGL addon, so .xterm-rows DOM nodes
    // do not exist; measure content coverage through the xterm buffer instead.
    let nonEmptyRowCount = 0;
    const buffer = term?.buffer?.active ?? null;
    if (buffer) {
      for (let i = 0; i < buffer.length; i += 1) {
        const line = buffer.getLine(i);
        if (line && line.translateToString(true).trim().length > 0) {
          nonEmptyRowCount += 1;
        }
      }
    }

    return {
      visibleSurfaceCount: visibleSurfaces.length,
      surfaceHeight: surfaceRect?.height ?? 0,
      rows: typeof term?.rows === "number" ? term.rows : null,
      cols: typeof term?.cols === "number" ? term.cols : null,
      nonEmptyRowCount,
      tabIds: currentTabIds,
    };
  }, tabIds);
}

async function waitForAlternateScreenExit(page: Page, afterAlt: string, timeout: number) {
  let lastBufferText = "";
  let lastProbe = await readTerminalRenderProbe(page);

  try {
    await expect
      .poll(
        async () => {
          lastBufferText = await getTerminalBufferText(page);
          lastProbe = await readTerminalRenderProbe(page);
          return (
            lastProbe.altEnterWrites > 0 &&
            lastProbe.altExitWrites > 0 &&
            lastBufferText.includes(afterAlt)
          );
        },
        {
          intervals: [50],
          message: `wait for alternate-screen exit and ${afterAlt} output`,
          timeout,
        },
      )
      .toBe(true);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(
      `Timed out waiting for alternate-screen exit: ${message}\n${JSON.stringify(
        {
          afterAlt,
          probe: summarizeTerminalRenderProbe(lastProbe),
          bufferTextTail: lastBufferText.slice(-500),
        },
        null,
        2,
      )}`,
      { cause: error },
    );
  }

  return lastProbe;
}

test.describe("Terminal alternate-screen transitions", () => {
  test.describe.configure({ timeout: 120_000 });

  let harness: TerminalE2EHarness;

  test.beforeAll(async () => {
    harness = await TerminalE2EHarness.create({ tempPrefix: "terminal-alt-" });
  });

  test.afterAll(async () => {
    await harness?.cleanup();
  });

  test("restores the normal screen after full-screen alternate buffer exit without remounting", async ({
    page,
  }, testInfo) => {
    test.setTimeout(60_000);

    await installTerminalRenderProbe(page);

    await withTerminalInApp(page, harness, { name: "alternate-screen" }, async () => {
      await harness.setupPrompt(page);

      const terminal = harness.terminalSurface(page);
      const historyReady = `HISTORY_READY_${Date.now()}`;
      await terminal.pressSequentially(
        `for i in $(seq 1 80); do echo HISTORY_$i; done; echo ${historyReady}\n`,
        { delay: 0 },
      );
      function hasHistoryReady(text: string): boolean {
        return text.includes(historyReady);
      }
      await waitForTerminalContent(page, hasHistoryReady, 10_000);

      await resetTerminalRenderProbe(page);
      await page.waitForTimeout(500);
      const settledProbe = await readTerminalRenderProbe(page);
      expect(settledProbe.resetWrites, "terminal should be idle before alternate-screen act").toBe(
        0,
      );
      await resetTerminalRenderProbe(page);

      const afterAlt = `AFTER_ALT_${Date.now()}`;
      await startTerminalFrameSampling(page);
      await terminal.pressSequentially(
        `printf '\\033[?1049h\\033[2J\\033[HALT_SCREEN_TOP\\n'; sleep 0.25; printf '\\033[?1049l'; echo ${afterAlt}\n`,
        { delay: 0 },
      );
      const probe = await waitForAlternateScreenExit(page, afterAlt, 10_000);
      const probeSummary = summarizeTerminalRenderProbe(probe);

      await testInfo.attach("alternate-screen-probe", {
        body: JSON.stringify({ summary: probeSummary, probe }, null, 2),
        contentType: "application/json",
      });

      expect(probe.setCount, "terminal instance should not be replaced after attach").toBe(0);
      expect(probe.unsetCount, "terminal instance should not be unset after attach").toBe(0);
      expect(
        probe.altEnterWrites,
        "test command should enter the alternate screen",
      ).toBeGreaterThan(0);
      expect(probe.altExitWrites, "test command should exit the alternate screen").toBeGreaterThan(
        0,
      );
      expect(probe.resetWrites, "alternate-screen exit should not replay a snapshot reset").toBe(0);

      const finalBufferText = await getTerminalBufferText(page);
      expect(finalBufferText).toContain(historyReady);
      expect(finalBufferText).toContain(afterAlt);

      function isSuspiciousFrame(frame: (typeof probe.frames)[number]): boolean {
        return (
          frame.text.includes("$") &&
          !frame.text.includes(historyReady) &&
          !frame.text.includes(afterAlt) &&
          frame.nonEmptyRows <= 2 &&
          (frame.firstNonEmptyRow ?? Number.POSITIVE_INFINITY) <= 1
        );
      }
      const suspiciousFrames = probe.frames.filter(isSuspiciousFrame);

      expect(
        suspiciousFrames,
        "normal-screen restore should not flash to a mostly blank prompt-at-top frame",
      ).toEqual([]);
    });
  });

  test("opening vim in a new terminal fills the terminal surface", async ({ page }, testInfo) => {
    test.setTimeout(60_000);

    await page.setViewportSize({ width: 1280, height: 900 });
    await installTerminalRenderProbe(page);

    await withTerminalInApp(page, harness, { name: "vim-layout" }, async () => {
      await harness.setupPrompt(page);
      const terminalSurface = harness.terminalSurface(page).first();
      await expect(terminalSurface).toBeVisible({ timeout: 15_000 });
      await terminalSurface.click();

      const beforeVim = await readTerminalLayoutMetrics(page);

      await startTerminalFrameSampling(page, 2_000);
      await terminalSurface.pressSequentially("vim -Nu NONE -n\n", { delay: 0 });

      // Wait until vim has actually drawn its full-screen UI (tilde rows on
      // the alternate buffer) instead of sampling on a fixed timer.
      let vimMetrics = beforeVim;
      await expect
        .poll(
          async () => {
            vimMetrics = await readTerminalLayoutMetrics(page);
            const rows = vimMetrics.rows ?? 0;
            return rows > 0 && vimMetrics.nonEmptyRowCount >= Math.floor(rows * 0.75);
          },
          { timeout: 15_000, intervals: [100, 250, 500, 1000] },
        )
        .toBe(true);

      const probe = await readTerminalRenderProbe(page);
      const samples: TerminalLayoutMetrics[] = [];
      for (let index = 0; index < 12; index += 1) {
        samples.push(await readTerminalLayoutMetrics(page));
        await page.waitForTimeout(50);
      }

      await testInfo.attach("reused-terminal-layout-metrics", {
        body: JSON.stringify(
          {
            beforeVim,
            vimMetrics,
            probe: summarizeTerminalRenderProbe(probe),
            samples,
          },
          null,
          2,
        ),
        contentType: "application/json",
      });

      const firstSample = samples[0];
      const finalSample = samples.at(-1);
      expect(firstSample, "expected an initial layout sample").toBeTruthy();
      expect(finalSample, "expected a final layout sample").toBeTruthy();

      expect(
        finalSample?.visibleSurfaceCount ?? 0,
        "opening vim should leave exactly one visible terminal surface",
      ).toBe(1);
      const finalRows = finalSample?.rows ?? 0;
      expect(finalRows, "terminal dimensions should be readable via the xterm buffer").toBeGreaterThan(
        0,
      );
      expect(
        finalSample?.nonEmptyRowCount ?? 0,
        "vim should fill most of the terminal rows with content",
      ).toBeGreaterThanOrEqual(Math.floor(finalRows * 0.75));
      expect(
        finalSample?.nonEmptyRowCount ?? 0,
        "vim should render substantially more content than the shell prompt",
      ).toBeGreaterThan(Math.max(10, beforeVim.nonEmptyRowCount));
    });
  });
});
