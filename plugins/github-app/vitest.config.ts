// SPDX-License-Identifier: Apache-2.0
//
// Test runner of the GitHub app, following sdk/typescript/vitest.config.ts. Two projects: `unit`
// (fakes in the test process) and `live` (the real server of services/server/test/sdkserver,
// brief M2-49 Design note 19; its globalSetup builds the binary once). Both load the guard of
// test/setup/no-network.ts, so no test reaches a host other than the loopback host (Design note
// 18). Coverage counts src/ only; test/live/ and test/fakes/ are outside it.

import { configDefaults, defineConfig } from "vitest/config";

// The real server starts a PostgreSQL container and waits real time, far beyond the default
// hook and test timeouts (the value of the SDK's live project).
const LIVE_TIMEOUT_MS = 600_000;
// A unit test waits for a condition with test/helpers/patience.ts; its only bound is
// `quality.test.patience`, so the runner's own limits are the same value.
const UNIT_TIMEOUT_MS = 180_000;

export default defineConfig({
  test: {
    setupFiles: ["test/setup/no-network.ts"],
    projects: [
      {
        extends: true,
        test: {
          name: "unit",
          include: ["test/**/*.test.ts"],
          exclude: [...configDefaults.exclude, "test/live/**"],
          testTimeout: UNIT_TIMEOUT_MS,
          hookTimeout: UNIT_TIMEOUT_MS,
        },
      },
      {
        extends: true,
        test: {
          name: "live",
          include: ["test/live/**/*.test.ts"],
          globalSetup: ["test/live/global-setup.ts"],
          testTimeout: LIVE_TIMEOUT_MS,
          hookTimeout: LIVE_TIMEOUT_MS,
        },
      },
    ],
    coverage: {
      provider: "v8",
      include: ["src/**/*.ts"],
      exclude: ["test/live/**", "test/fakes/**"],
      reporter: ["text", "lcov"],
      reportsDirectory: "coverage",
      // Named value `quality.coverage.typescript_min_percent` (docs/spec/23-quality.md
      // section 11), the same thresholds as the SDK.
      thresholds: {
        lines: 80,
        functions: 80,
        branches: 80,
        statements: 80,
      },
    },
  },
});
