/**
 * Quick Playwright verification script: check that the web app at
 * http://localhost:19000 loads without JS errors and renders the
 * main UI surfaces (sidebar + workspace area). Targets the already-running
 * dev server, not a fresh stack.
 *
 * Run: node e2e/voice-removal-regression.mjs
 */
import { chromium } from "playwright";

const BASE_URL = "http://localhost:19000";

async function main() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();

  const consoleErrors = [];
  const uncaughtExceptions = [];
  const wsFrames = { sent: [], received: [] };
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text());
  });
  page.on("pageerror", (err) => uncaughtExceptions.push(err.message));
  page.on("websocket", (ws) => {
    ws.on("framesent", (ev) => {
      const s = typeof ev.payload === "string" ? ev.payload : "<binary>";
      if (s.includes("timeline") || s.includes("agent_list") || s.includes("hello")) {
        wsFrames.sent.push(s.slice(0, 400));
      }
    });
    ws.on("framereceived", (ev) => {
      const s = typeof ev.payload === "string" ? ev.payload : "<binary>";
      if (s.includes("timeline") || s.includes("agent_list") || s.includes("server_info") || s.includes("agent_created")) {
        wsFrames.received.push(s.slice(0, 400));
      }
    });
  });

  console.log(`[1] navigating to ${BASE_URL}`);
  const start = Date.now();
  await page.goto(BASE_URL, { waitUntil: "networkidle", timeout: 30_000 });
  console.log(`    loaded in ${Date.now() - start}ms`);

  // Take a screenshot at landing
  await page.screenshot({ path: "/tmp/voice-removal-01-landing.png", fullPage: true });

  console.log("[2] waiting for React to mount and render");
  // The app renders a sidebar with hosts/workspaces; wait for anything that looks like the shell.
  await page.waitForFunction(
    () => document.body && document.body.children.length > 0,
    { timeout: 15_000 },
  );
  await page.waitForTimeout(1500); // let deferred effects settle

  console.log("[3] checking for obvious stuck states");
  const bodyText = await page.locator("body").innerText();
  const snapshot = {
    hasContent: bodyText.trim().length > 0,
    textLength: bodyText.length,
    consoleErrors: consoleErrors.length,
    uncaughtExceptions: uncaughtExceptions.length,
  };
  console.log("    snapshot:", JSON.stringify(snapshot, null, 2));

  if (snapshot.consoleErrors > 0) {
    console.log("    first 5 console errors:");
    for (const e of consoleErrors.slice(0, 5)) console.log("      -", e.slice(0, 240));
  }
  if (snapshot.uncaughtExceptions > 0) {
    console.log("    uncaught exceptions:");
    for (const e of uncaughtExceptions.slice(0, 5)) console.log("      -", e.slice(0, 240));
  }

  // Try to find and click the first workspace in the sidebar (if any) to trigger timeline loading.
  console.log("[4] looking for a workspace entry to exercise timeline loading");
  const workspaceLink = page.locator('[data-testid*="workspace"], a[href*="workspace"]').first();
  if (await workspaceLink.count()) {
    await workspaceLink.click().catch(() => {});
    await page.waitForTimeout(3000);
    await page.screenshot({ path: "/tmp/voice-removal-02-workspace.png", fullPage: true });
    console.log("    after workspace click, console errors:", consoleErrors.length);
    if (consoleErrors.length > snapshot.consoleErrors) {
      console.log("    new errors:");
      for (const e of consoleErrors.slice(snapshot.consoleErrors).slice(0, 5)) {
        console.log("      -", e.slice(0, 240));
      }
    }
  } else {
    console.log("    no workspace link found (first-run state); skipping timeline exercise");
  }

  // Try to navigate into a project with sessions — click the "solo" sidebar entry, then look for
  // an existing session in the Sessions tab. This exercises timeline loading.
  console.log("[5] trying to open an existing session to verify timeline rendering");
  const errorsBeforeSession = consoleErrors.length;
  // Click on "Sessions" tab in sidebar
  const sessionsNav = page.locator("text=Sessions").first();
  if (await sessionsNav.count()) {
    await sessionsNav.click().catch(() => {});
    await page.waitForTimeout(1500);
    await page.screenshot({ path: "/tmp/voice-removal-03-sessions-list.png", fullPage: true });

    // Click the first session row — look for the "a2a demo" title (most recent, the one the user
    // was just using) or fall back to the first list item.
    const firstSession = page.locator("text=a2a demo").first();
    const anySession = page.getByRole("link").filter({ hasText: /~/ }).first();
    const target = (await firstSession.count()) ? firstSession : anySession;
    if (await target.count()) {
      await target.click({ timeout: 5000 }).catch(() => {});
      console.log("    url after click =", page.url());

      // Periodic snapshots to detect stuck-spinner vs slow-load
      for (let i = 1; i <= 6; i++) {
        await page.waitForTimeout(3000);
        await page.screenshot({ path: `/tmp/voice-removal-04-session-timeline-${i}.png`, fullPage: true });
        const spinnerVisible = await page.locator('[data-testid*="spinner"], [role="progressbar"]').count();
        const textNow = await page.locator("main, [data-pane], [class*='Pane']").first().innerText().catch(() => "");
        console.log(`    t+${i * 3}s: spinner=${spinnerVisible > 0}, textLen=${textNow.length}, errors=${consoleErrors.length}`);
        if (spinnerVisible === 0 && textNow.length > 50) break;
      }

      const newErrors = consoleErrors.length - errorsBeforeSession;
      console.log("    final: new errors =", newErrors);
      console.log("    final url =", page.url());
      if (newErrors > 0) {
        console.log("    errors after session open:");
        for (const e of consoleErrors.slice(errorsBeforeSession).slice(0, 8)) {
          console.log("      -", e.slice(0, 240));
        }
      }
      // Check for the "thinks" block (often rendered with "Thinking", "Thought", or a details element)
      const thinksBlocks = await page.locator('text=/think|thought/i').count();
      const assistantMessages = await page.locator('[data-testid*="assistant"], [data-role="assistant"]').count();
      console.log("    thinks blocks visible:", thinksBlocks, "assistant messages:", assistantMessages);
    } else {
      console.log("    no session found in list; skipping");
    }
  } else {
    console.log("    no Sessions nav found; skipping");
  }

  console.log("\n[6] WebSocket frame summary (timeline / agent_list / hello)");
  console.log("    sent    :", wsFrames.sent.length);
  for (const f of wsFrames.sent) console.log("      >>", f);
  console.log("    received:", wsFrames.received.length);
  for (const f of wsFrames.received) console.log("      <<", f);

  await browser.close();

  const failed = snapshot.uncaughtExceptions > 0;
  console.log(failed ? "\nFAILED — uncaught exceptions detected" : "\nPASSED — no uncaught exceptions");
  process.exit(failed ? 1 : 0);
}

main().catch((err) => {
  console.error("script failed:", err);
  process.exit(2);
});
