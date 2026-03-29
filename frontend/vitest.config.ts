import { defineConfig } from "vitest/config"
import react from "@vitejs/plugin-react"
import path from "path"

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./test/setup.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      include: [
        "components/ui/**/*.tsx",
        "hooks/useUpload.ts",
        "hooks/useHeartbeat.ts",
        "hooks/usePresence.ts",
        "lib/chatHelpers.ts",
        "lib/validation.ts",
      ],
      thresholds: { lines: 98, functions: 98, branches: 98, statements: 98 },
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
