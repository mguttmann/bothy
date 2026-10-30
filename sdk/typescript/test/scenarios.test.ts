// SPDX-License-Identifier: Apache-2.0
//
// AG-08, AG-09, AG-12, AG-28, AZ-P6 and R-AG-19 against the FakeServer (port of
// sdk/python/tests/test_scenarios_fake.py; the same scenario functions run against the real
// server in test/live/scenarios.live.test.ts), plus the check that the scenario functions of
// `testing` equal the shared list of sdk/testdata/scenarios/README.md in camelCase (port of
// sdk/python/tests/test_scenario_list.py).

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import * as testing from "../src/testing/index.js";
import {
  FakeServer,
  ag08InvalidTokens,
  ag09DoubleDelivery,
  ag12PatchRules,
  ag28SlowStartAndRestart,
  azp6AdapterNeverStarts,
  rag19AllowlistFromServer,
} from "../src/testing/index.js";

const README = join(
  import.meta.dirname,
  "..",
  "..",
  "testdata",
  "scenarios",
  "README.md",
);
const HEADING = /^### (\S+)\s*$/gm;
// A scenario function name: the test ID (`ag08`, `azp6`) and the camelCase rest.
const SCENARIO_NAME = /^[a-z]+\d+[A-Z]\w*$/;
const TEST_TIMEOUT = 120_000;

/** `ag08_invalid_tokens` becomes `ag08InvalidTokens`. */
function camelCase(name: string): string {
  return name.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase());
}

/** The scenario names of the shared README in camelCase, in their order. */
function listed(): string[] {
  const text = readFileSync(README, "utf8");
  return [...text.matchAll(HEADING)].map((m) => camelCase(m[1] ?? ""));
}

/** The scenario functions exported by `testing`. */
function implemented(): string[] {
  return Object.entries(testing)
    .filter(
      ([name, value]) =>
        typeof value === "function" && SCENARIO_NAME.test(name),
    )
    .map(([name]) => name);
}

describe("the shared scenario list", () => {
  it("lists each scenario once", () => {
    const names = listed();
    expect(names.length).toBeGreaterThan(0);
    expect(new Set(names).size).toBe(names.length);
  });

  it("equals the scenario functions of testing in camelCase", () => {
    expect(new Set(implemented())).toEqual(new Set(listed()));
  });
});

describe("the scenarios against the FakeServer", () => {
  let fake: FakeServer;

  beforeEach(async () => {
    fake = new FakeServer();
    await fake.start();
  });

  afterEach(async () => {
    await fake.stop();
  });

  it("AG-08 adapter side", () => ag08InvalidTokens(fake), TEST_TIMEOUT);

  it("AZ-P6 for the SDK", () => azp6AdapterNeverStarts(fake), TEST_TIMEOUT);

  it("AG-09", () => ag09DoubleDelivery(fake), TEST_TIMEOUT);

  it("AG-12", () => ag12PatchRules(fake), TEST_TIMEOUT);

  it("AG-28", () => ag28SlowStartAndRestart(fake), TEST_TIMEOUT);

  it("R-AG-19", () => rag19AllowlistFromServer(fake), TEST_TIMEOUT);
});
