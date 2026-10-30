// SPDX-License-Identifier: Apache-2.0
//
// The scenario functions against the real server of this tree (ADR 0026 "Confirmation"; port of
// the scenario tests of sdk/python/tests/realserver/test_realserver_scenarios.py). One RealServer
// serves every scenario: each scenario connects its own consumers, so a run one scenario leaves
// queued never reaches another. It needs the Go tool chain and a Docker-compatible runtime;
// without them it fails, it never skips (brief M2-30 Design note 12).

import { afterAll, beforeAll, describe, inject, it } from "vitest";

import {
  ag08InvalidTokens,
  ag09DoubleDelivery,
  ag12PatchRules,
  ag28SlowStartAndRestart,
  azp6AdapterNeverStarts,
  rag19AllowlistFromServer,
} from "../../src/testing/index.js";
import { type RealServer, startRealServer } from "./sdkserver.js";

describe("the scenarios against the real server", () => {
  let real: RealServer;

  beforeAll(async () => {
    real = await startRealServer(inject("sdkserverBinary"));
  });

  afterAll(async () => {
    await real?.stop();
  });

  it("AG-08 adapter side", () => ag08InvalidTokens(real));

  it("AZ-P6 for the SDK", () => azp6AdapterNeverStarts(real));

  it("AG-09", () => ag09DoubleDelivery(real));

  it("AG-12", () => ag12PatchRules(real));

  it("AG-28", () => ag28SlowStartAndRestart(real));

  it("R-AG-19", () => rag19AllowlistFromServer(real));
});
