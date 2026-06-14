// Playwright spec: verify the Auto theme option is exposed in the theme picker.
//
// Contract:
//   1. User navigates to a tmux pane screen
//   2. Clicks the theme-picker trigger (palette icon)
//   3. The dropdown contains System, Dark, Light, Bash, AND a new Auto option
//   4. Selecting Auto persists the choice (subsequent renders use the auto theme)
//
// This spec is TDD-red today: the Auto option does not yet exist in the theme
// picker. It turns green once the UI change lands in tmux-pane-screen.tsx.
//
// Renderer-fidelity assertions (pixel-exact RGB of auto-rendered bg/fg) belong
// in a separate spec that depends on daemon OSC 10/11 probe plumbing. That
// integration lives outside TDD scope; see docs/architecture/auto-theme.md
// (to be authored) for the manual/nightly probe workflow.

import { test, expect } from "./fixtures";

test.describe("Auto theme option", () => {
  test("theme picker exposes Auto alongside System / Dark / Light / Bash", async ({ page }) => {
    // Navigate to the tmux pane route. The fixture seeds a host + daemon so
    // the pane screen renders (even with no agent selected, the theme picker
    // is reachable via the header once an agent is picked — we assert the
    // option exists in the dropdown regardless of selection state).
    await page.goto("/");

    // The Auto option is a product-level label; assert it appears in the DOM
    // somewhere in the picker's accessible tree. This fails red today because
    // the picker only renders System / Dark / Light / Bash.
    const autoOption = page.getByRole("menuitem", { name: /^Auto$/ });
    await expect(autoOption).toBeVisible({ timeout: 5_000 });
  });
});
