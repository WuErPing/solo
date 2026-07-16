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
      "react-hooks/refs": "off",
      "react-hooks/set-state-in-effect": "off",
      "react-hooks/preserve-manual-memoization": "off",
      "react-hooks/immutability": "off",
      "react-hooks/static-components": "off",
      "react-hooks/purity": "off",
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
    ignores: ["dist/*"],
  },
]);
