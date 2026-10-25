// SPDX-License-Identifier: Apache-2.0
// Tests wait for a state, never a fixed time (the rules of deterministic tests; the SDK's
// test/no-fixed-waits.test.ts holds the same rule): a numeric time bound in a test file is either
// the patience or the bound of an answer that never comes, marked with `never-answers` on its
// line or the line before. A short bound for a request that does get an answer fails on a slow
// or loaded machine (red team of M2-49).

import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

const ROOT = import.meta.dirname;
const BOUND =
  /\b(?:timeoutMs|httpTimeoutMs|testTimeout|hookTimeout)\s*[:=?]+\s*[\d_]+|\bsetTimeout\([^,]*,\s*[\d_]+\s*\)|\bsleep\(\s*[\d_]+\s*\)|\bpause\(\s*[\d_]{3,}\s*\)/;

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return files(path);
    return entry.name.endsWith(".ts") ? [path] : [];
  });
}

/** The lines of a source with a numeric time bound that is not marked. */
export function unmarkedBounds(source: string): string[] {
  const lines = source.split("\n");
  return lines.filter((line, i) => {
    if (!BOUND.test(line)) return false;
    return !`${lines[i - 1] ?? ""}\n${line}`.includes("never-answers");
  });
}

describe("no fixed waits", () => {
  it("finds an unmarked bound and accepts a marked one", () => {
    expect(unmarkedBounds("client({ timeoutMs: 50 })")).toHaveLength(1);
    expect(
      unmarkedBounds("// never-answers: x\nclient({ timeoutMs: 50 })"),
    ).toHaveLength(0);
    expect(unmarkedBounds("await sleep(1000);")).toHaveLength(1);
    expect(unmarkedBounds("timeoutMs: PATIENCE_MS")).toHaveLength(0);
  });

  it("holds every test file of the app", () => {
    const own = join(ROOT, "no-fixed-waits.test.ts");
    const found = files(ROOT)
      .filter(
        (path) => path !== own && !path.includes(`${join("test", "live")}`),
      )
      .flatMap((path) =>
        unmarkedBounds(readFileSync(path, "utf8")).map(
          (line) => `${path}: ${line.trim()}`,
        ),
      );
    expect(found).toEqual([]);
  });
});
