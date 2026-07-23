import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  js.configs.recommended,
  tseslint.configs.recommended,
  {
    rules: {
      "no-empty": ["error", { allowEmptyCatch: false }],
      "no-empty-function": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
        },
      ],
      "@typescript-eslint/no-empty-object-type": [
        "error",
        { allowInterfaces: "with-single-extends" },
      ],
      "prefer-const": "off",
      complexity: ["error", 20],
      "max-lines": ["error", 800],
    },
  },
  {
    // Grandfathered long files (> 800 lines). Tracked for incremental splits
    // (see messages.ts domain split and ConnectionManager extraction); remove a
    // file from this list once it is brought under the cap.
    files: [
      "src/client/agent-rpc.ts",
      "src/client/connection-manager.ts",
      "src/client/daemon-client.ts",
      "src/shared/messages.ts",
      "src/shared/messages-agent.ts",
    ],
    rules: { "max-lines": "off" },
  },
  {
    ignores: ["dist/**/*"],
  }
);
