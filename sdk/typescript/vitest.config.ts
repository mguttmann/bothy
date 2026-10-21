// SPDX-License-Identifier: Apache-2.0
//
// Test runner of the TypeScript SDK (component contract: docs/spec/23-quality.md section 3).
// Coverage counts hand-written code only; generated code is held by its generator and gate G4
// (docs/DECISIONS.md M2-23/7). The lcov report is the path sonar-project.properties reads.
//
// Two projects: `unit` and `live`. The live files run against the real server of
// services/server/test/sdkserver (brief M2-30 Design note 12); their globalSetup builds it once.
// Vitest starts a project's globalSetup only when that project has files in the run, so a run
// of unit files alone needs neither the Go tool chain nor Docker, while every run of a live
// file builds the binary and fails without them (like the session fixture of
// sdk/python/tests/realserver/conftest.py).

import { configDefaults, defineConfig } from "vitest/config";

// The real server starts a PostgreSQL container and waits real time (agent.invocation_token_ttl
// plus auth.clock_leeway in AG-28), far beyond the default hook and test timeouts.
const LIVE_TIMEOUT_MS = 600_000;

export default defineConfig({
  test: {
    // Each project names its files: with `extends: true` an `include` here would be joined to
    // the project's own list, so the live project would run every unit file again.
    projects: [
      {
        extends: true,
        test: {
          name: "unit",
          include: ["test/**/*.test.ts"],
          exclude: [...configDefaults.exclude, "test/live/**"],
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
      exclude: ["src/generated/**", "test/live/**"],
      reporter: ["text", "lcov"],
      reportsDirectory: "coverage",
      // Named value `quality.coverage.typescript_min_percent` (docs/spec/23-quality.md
      // section 11); tools/docs/tests/test_config_values.py holds all four equal to it.
      thresholds: {
        lines: 80,
        functions: 80,
        branches: 80,
        statements: 80,
      },
    },
  },
});
