// SPDX-License-Identifier: Apache-2.0
//
// Extends the Jest configuration of the scaffold (.config/jest.config.js, kept as generated).
// Additions: tooling tests under test/, the font files of the design tokens as stubs, and the
// coverage threshold of the TypeScript components.

// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';

const base = require('./.config/jest.config');

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...base,
  testMatch: [...base.testMatch, '<rootDir>/test/**/*.test.{ts,tsx}'],
  moduleNameMapper: {
    ...base.moduleNameMapper,
    // Webpack turns a font file into its URL (asset/resource); the tests need only a string.
    '\\.woff2$': '<rootDir>/test/file-stub.js',
  },
  collectCoverageFrom: ['src/**/*.{ts,tsx}', '!src/**/*.test.{ts,tsx}'],
  coverageReporters: ['text', 'lcov'],
  // Named value `quality.coverage.typescript_min_percent` (docs/spec/23-quality.md section 11).
  coverageThreshold: {
    global: { lines: 80, functions: 80, branches: 80, statements: 80 },
  },
};
