import { defineConfig } from "vitest/config"
import react from "@vitejs/plugin-react"
import path from "path"

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./test/setup.ts"],
    exclude: ["**/node_modules/**", "**/e2e/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      // The whole reusable surface is measured — components, hooks, libs,
      // contexts. Pages under app/ are exercised by Playwright instead.
      include: [
        "components/**/*.tsx",
        "hooks/**/*.ts",
        "lib/**/*.ts",
        "contexts/**/*.tsx",
      ],
      // Ratchet thresholds: set at the current floor so coverage can only
      // move up. Raise them as the remaining untested components gain tests.
      thresholds: { lines: 61, functions: 57, branches: 51, statements: 58 },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
      "next/image": path.resolve(__dirname, "./test/__mocks__/next-image.tsx"),
      "next/navigation": path.resolve(__dirname, "./test/__mocks__/next-navigation.ts"),
    },
  },
})
