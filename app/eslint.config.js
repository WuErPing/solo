// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require("eslint/config");
const expoConfig = require("eslint-config-expo/flat");

module.exports = defineConfig([
  expoConfig,
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
    ignores: ["dist/*"],
  },
]);
