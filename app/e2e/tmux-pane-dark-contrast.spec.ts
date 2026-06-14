import { spawn, execFileSync } from "node:child_process";
import { mkdtemp, writeFile, chmod, rm } from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import { test, expect } from "./fixtures";

function relativeLuminance(rgb: { r: number; g: number; b: number }): number {
  const channel = (v: number) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  };
  return (
    0.2126 * channel(rgb.r) + 0.7152 * channel(rgb.g) + 0.0722 * channel(rgb.b)
  );
}

function parseRgb(color: string): { r: number; g: number; b: number } | null {
  const match = /rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*[\d.]+\s*)?\)/.exec(color);
  if (!match) return null;
  return {
    r: parseInt(match[1]!, 10),
    g: parseInt(match[2]!, 10),
    b: parseInt(match[3]!, 10),
  };
}

function contrastRatio(fg: string, bg: string): number {
  const a = parseRgb(fg);
  const b = parseRgb(bg);
  if (!a || !b) return 0;
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

const MIN_CONTRAST = 3;

test.describe("tmux pane dark theme contrast", () => {
  test.describe.configure({ timeout: 90_000 });

  let fakeBinDir: string | null = null;
  let sessionName: string | null = null;

  test.beforeAll(async () => {
    fakeBinDir = await mkdtemp(path.join(tmpdir(), "solo-fake-qodercli-"));
    const fakeQodercli = path.join(fakeBinDir, "qodercli");
    await writeFile(
      fakeQodercli,
      `#!/usr/bin/env node
// Fake qodercli used for E2E contrast testing.
const text =
  "\\x1b[38;5;59mchange-android-logo\\x1b[0m\\r\\n" +
  "\\x1b[38;5;231mvisible-anchor\\x1b[0m\\r\\n";
process.stdout.write(text);
setInterval(() => {}, 60_000);
`,
    );
    await chmod(fakeQodercli, 0o755);

    sessionName = `solo-contrast-${Date.now()}-${process.pid}`;
    const env: NodeJS.ProcessEnv = {
      ...process.env,
      PATH: `${fakeBinDir}${path.delimiter}${process.env.PATH ?? ""}`,
      TMUX: undefined,
    };

    // Start a detached tmux session running our fake qodercli.
    // The process name remains "qodercli" so the daemon's agent scanner picks it up.
    spawn("tmux", ["new-session", "-d", "-s", sessionName, "-n", "main", "qodercli"], {
      env,
      detached: false,
    });

    // Force the pane title so the daemon's title-based agent detection finds
    // our fake process even though tmux reports the running command as "node".
    execFileSync("tmux", ["select-pane", "-T", "qodercli", "-t", `${sessionName}:0.0`], {
      timeout: 5_000,
    });

    // Give tmux a moment to create the pane before the first dashboard poll.
    await new Promise((resolve) => setTimeout(resolve, 500));
  });

  test.afterAll(async () => {
    if (sessionName) {
      try {
        execFileSync("tmux", ["kill-session", "-t", sessionName], { timeout: 5_000 });
      } catch {
        // Session may already be gone; ignore.
      }
    }
    if (fakeBinDir) {
      await rm(fakeBinDir, { recursive: true, force: true });
    }
  });

  test("dark terminal theme lightens low-contrast ANSI text so it stays readable", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    // Force the terminal theme to "dark" before the app boots.
    await page.addInitScript(() => {
      localStorage.setItem(
        "@solo:app-settings",
        JSON.stringify({
          theme: "dark",
          sendBehavior: "interrupt",
          terminalTheme: "dark",
        }),
      );
    });

    await page.goto("/tmux-dashboard");

    // Find and open the fake qodercli agent card.
    const agentCard = page.getByText(new RegExp(`S:${sessionName}`));
    await expect(agentCard).toBeVisible({ timeout: 15_000 });
    await agentCard.click();

    // Wait for the captured pane content to render.
    const anchor = page.getByText("change-android-logo");
    await expect(anchor).toBeVisible({ timeout: 15_000 });

    // Read the actual rendered foreground and background colors.
    const scroll = page.getByTestId("tmux-pane-scroll");
    await expect(scroll).toBeVisible();

    const fg = await anchor.evaluate((el) => window.getComputedStyle(el).color);
    const bg = await scroll.evaluate((el) => window.getComputedStyle(el).backgroundColor);

    expect(fg, "rendered text should have a computed foreground color").not.toBe("");
    expect(bg, "scroll container should have a computed background color").not.toBe("");

    const ratio = contrastRatio(fg, bg);
    expect(
      ratio,
      `low-contrast ANSI text should be adjusted to be readable (got ${fg} on ${bg}, ratio ${ratio.toFixed(2)})`,
    ).toBeGreaterThanOrEqual(MIN_CONTRAST);
  });
});
