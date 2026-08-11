import { defineConfig } from "@playwright/test";

// TARGET_URL lets this same suite run against either a dev server this
// config starts itself (the default, for day-to-day local iteration) or an
// already-deployed instance — e.g. TARGET_URL=http://localhost:3005 to
// verify the real docker-compose-built container, the same "verify by hand
// against the real running dockerized stack, on top of the automated
// tests" step every other phase in this project has gone through.
const targetUrl = process.env.TARGET_URL ?? "http://localhost:3006";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: targetUrl,
  },
  // Reuses an already-running dev server if one's up (e.g. started by hand
  // for manual verification); otherwise starts one itself. Either way this
  // is testing the real Next.js server, real Auth.js wiring, and a real
  // Postgres — no mocked auth layer, same "test the real thing" rule every
  // other integration test in this project follows. Skipped entirely when
  // TARGET_URL points elsewhere — nothing to spawn against a container
  // compose already started.
  webServer: process.env.TARGET_URL
    ? undefined
    : {
        command: "npm run dev -- -p 3006",
        url: targetUrl,
        reuseExistingServer: true,
        timeout: 60_000,
      },
});
