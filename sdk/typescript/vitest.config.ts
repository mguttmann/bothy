// SPDX-License-Identifier: Apache-2.0
//
// Test runner of the TypeScript SDK (component contract: docs/spec/23-quality.md section 3).
// Coverage counts hand-written code only; generated code is held by its generator and gate G4
// (docs/DECISIONS.md M2-23/7). The lcov report is the path sonar-project.properties reads.

import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.ts"],
      exclude: ["src/generated/**"],
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
