import { defineConfig } from "vitest/config"

// Unit tests run without the React Router plugin: they cover plain modules.
export default defineConfig({
  resolve: {
    tsconfigPaths: true,
  },
  test: {
    include: ["app/**/*.test.ts"],
  },
})
