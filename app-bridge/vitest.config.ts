import { defineConfig, configDefaults } from "vitest/config";
import path from "path";

export default defineConfig({
  test: {
    environment: "node",
    globals: true,
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html", "lcov"],
      exclude: [...configDefaults.exclude, "**/*.config.*"],
    },
  },
  resolve: {
    alias: {
      "@server": path.resolve(__dirname, "./src"),
    },
  },
});
