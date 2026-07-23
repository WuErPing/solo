// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require("eslint/config");
const expoConfig = require("eslint-config-expo/flat");

module.exports = defineConfig([
  expoConfig,
  {
    settings: {
      "import/resolver": {
        typescript: { alwaysTryTypes: true },
        node: true,
      },
    },
  },
  {
    files: ["**/*.test.{ts,tsx}"],
    rules: {
      "react/display-name": "off",
      "import/first": "off",
    },
  },
  {
    rules: {
      "@typescript-eslint/no-unused-vars": [
        "warn",
        {
          vars: "all",
          args: "none",
          ignoreRestSiblings: true,
          caughtErrors: "all",
          varsIgnorePattern: "^_",
          argsIgnorePattern: "^_",
        },
      ],
      "@typescript-eslint/no-empty-object-type": [
        "warn",
        {
          allowInterfaces: "with-single-extends",
        },
      ],
    },
  },
  {
    files: ["**/*.d.ts"],
    rules: {
      "import/no-unresolved": "off",
    },
  },
  {
    rules: {
      "react-hooks/refs": "error",
      "react-hooks/set-state-in-effect": "error",
      "react-hooks/preserve-manual-memoization": "off",
      "react-hooks/immutability": "off",
      "react-hooks/static-components": "off",
      "react-hooks/purity": "error",
      "react-hooks/use-memo": "off",
      "react-hooks/incompatible-library": "off",
      "react-hooks/globals": "off",
    },
  },
  {
    rules: {
      "import/no-unresolved": [
        "error",
        { ignore: ["@getsolo/highlight"] },
      ],
    },
  },
  {
    rules: {
      complexity: ["error", 20],
      "max-lines": ["error", 800],
    },
  },
  {
    // Grandfathered complexity hotspots (CC > 20). Tracked for incremental splits;
    // remove a file from this list once its hotspots are refactored so the cap applies.
    files: [
      "src/components/sidebar-workspace-list.tsx",
      "src/components/terminal-emulator.tsx",
      "src/hooks/use-tmux-agents.ts",
      "src/runtime/host-runtime.ts",
      "src/screens/loop-detail-screen.tsx",
      "src/screens/settings/host-page.tsx",
      "src/screens/tmux-dashboard/tmux-dashboard-screen.tsx",
      "src/screens/tmux-pane-screen.tsx",
      "src/screens/tmux-pane-xterm-screen.tsx",
      "src/terminal/runtime/terminal-emulator-runtime.ts",
      "src/utils/ansi-parser.ts",
      "src/utils/detect-ansi-colors.ts",
    ],
    rules: { complexity: "off" },
  },
  {
    // Grandfathered long files (> 800 lines). Tracked for incremental splits;
    // remove a file from this list once it is brought under the cap.
    files: [
      "src/app/_layout.tsx",
      "src/components/agent-status-bar.tsx",
      "src/components/agent-stream-view.tsx",
      "src/components/combined-model-selector.tsx",
      "src/components/composer.test.tsx",
      "src/components/composer.tsx",
      "src/components/file-explorer-pane.tsx",
      "src/components/git-diff-pane.tsx",
      "src/components/left-sidebar.tsx",
      "src/components/message-input.tsx",
      "src/components/message.tsx",
      "src/components/sidebar-workspace-list.tsx",
      "src/components/split-container.tsx",
      "src/components/stream-strategy-web.tsx",
      "src/components/terminal-emulator.tsx",
      "src/components/terminal-pane.tsx",
      "src/components/ui/combobox.tsx",
      "src/components/ui/context-menu.tsx",
      "src/components/ui/dropdown-menu.tsx",
      "src/contexts/session-stream-reducers.test.ts",
      "src/contexts/session-stream-reducers.ts",
      "src/hooks/use-agent-form-state.ts",
      "src/hooks/use-tmux-capture-pane.test.ts",
      "src/keyboard/keyboard-shortcuts.ts",
      "src/panels/agent-panel.tsx",
      "src/runtime/host-runtime.test.ts",
      "src/runtime/host-runtime.ts",
      "src/screens/loop-detail-screen.tsx",
      "src/screens/new-workspace-screen.test.tsx",
      "src/screens/new-workspace-screen.tsx",
      "src/screens/project-settings-screen.test.tsx",
      "src/screens/project-settings-screen.tsx",
      "src/screens/settings-screen.tsx",
      "src/screens/tmux-dashboard/tmux-dashboard-screen.tsx",
      "src/screens/tmux-pane-screen.tsx",
      "src/screens/workspace/workspace-desktop-tabs-row.tsx",
      "src/screens/workspace/workspace-screen.tsx",
      "src/stores/session-store.ts",
      "src/stores/workspace-layout-actions.ts",
      "src/stores/workspace-layout-store.test.ts",
      "src/terminal/runtime/terminal-emulator-runtime.ts",
      "src/types/stream.test.ts",
      "src/types/stream.ts",
    ],
    rules: { "max-lines": "off" },
  },
  {
    ignores: ["dist/*"],
  },
]);
