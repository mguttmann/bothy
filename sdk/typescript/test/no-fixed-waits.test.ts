// SPDX-License-Identifier: Apache-2.0
/**
 * No test of the SDK waits for something that must happen with a fixed time or a fixed number of
 * loop turns as its bound (docs/decisions.d/M2-30.md D-M2-30-15).
 *
 * A bound of a few seconds or of a few hundred turns fails a correct program on a loaded machine
 * or a slow CI runner (the flaky "runs over REST polling" gave up after 400 turns). Tests wait
 * with `src/testing/patience.ts`, whose only bound is `quality.test.patience`; the same check as
 * `sdk/python/tests/test_no_fixed_waits.py`.
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import ts from "typescript";
import { describe, expect, it } from "vitest";

import { PATIENCE_MS } from "../src/testing/patience.js";

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return files(path);
    return path.endsWith(".ts") ? [path] : [];
  });
}

const SOURCES = [
  ...files(join(ROOT, "test")),
  ...files(join(ROOT, "src/testing")),
].filter((p) => !p.endsWith("src/testing/patience.ts"));

function has(node: ts.Node, kinds: readonly ts.SyntaxKind[]): boolean {
  let found = false;
  const visit = (n: ts.Node): void => {
    if (found) return;
    if (kinds.includes(n.kind)) found = true;
    else ts.forEachChild(n, visit);
  };
  visit(node);
  return found;
}

function isDateNow(node: ts.Node): boolean {
  return (
    ts.isCallExpression(node) &&
    ts.isPropertyAccessExpression(node.expression) &&
    node.expression.getText() === "Date.now"
  );
}

const LONG_MS = 1000;

function isLong(node: ts.Node | undefined): boolean {
  return (
    node !== undefined &&
    ts.isNumericLiteral(node) &&
    Number(node.text.replaceAll("_", "")) >= LONG_MS
  );
}

/**
 * A timer delay of a second or more: a literal, or a parameter whose default is one. A test
 * that ends a wait with such a timer (for example a close after 3 s) bounds by wall clock what
 * must happen; short delays stay allowed for checks that something does not happen.
 */
function longDelay(delay: ts.Node | undefined): boolean {
  if (delay === undefined) return false;
  if (isLong(delay)) return true;
  if (!ts.isIdentifier(delay)) return false;
  for (let scope: ts.Node | undefined = delay; scope; scope = scope.parent) {
    if (!ts.isFunctionLike(scope)) continue;
    const param = scope.parameters.find(
      (p) => ts.isIdentifier(p.name) && p.name.text === delay.text,
    );
    if (param !== undefined) return isLong(param.initializer);
  }
  return false;
}

/** The fixed waits of one source text: a deadline from `Date.now()` or a bounded polling loop. */
function fixedWaits(name: string, text: string): string[] {
  const file = ts.createSourceFile(name, text, ts.ScriptTarget.Latest, true);
  const found: string[] = [];
  const line = (n: ts.Node): number =>
    file.getLineAndCharacterOfPosition(n.getStart()).line + 1;
  const visit = (node: ts.Node): void => {
    if (
      ts.isBinaryExpression(node) &&
      node.operatorToken.kind === ts.SyntaxKind.PlusToken &&
      isDateNow(node.left)
    )
      found.push(`${line(node)}: a deadline from Date.now()`);
    if (
      ts.isForStatement(node) &&
      node.condition !== undefined &&
      has(node.statement, [ts.SyntaxKind.AwaitExpression]) &&
      (has(node.statement, [
        ts.SyntaxKind.ReturnStatement,
        ts.SyntaxKind.BreakStatement,
      ]) ||
        node.condition.getText().includes("&&"))
    )
      found.push(`${line(node)}: a polling loop of fixed turns`);
    if (
      ts.isCallExpression(node) &&
      node.expression.getText() === "setTimeout" &&
      longDelay(node.arguments[1])
    )
      found.push(`${line(node)}: a real timer of a second or more`);
    if (
      ts.isCallExpression(node) &&
      ["it", "test"].includes(node.expression.getText()) &&
      node.arguments[2] !== undefined &&
      ts.isNumericLiteral(node.arguments[2])
    )
      found.push(`${line(node)}: a fixed test timeout`);
    ts.forEachChild(node, visit);
  };
  visit(file);
  return found;
}

describe("waits of the tests", () => {
  it("no test waits with a fixed bound", () => {
    const offenders = SOURCES.flatMap((path) =>
      fixedWaits(path, readFileSync(path, "utf8")).map(
        (what) => `${relative(ROOT, path)}:${what}`,
      ),
    );
    expect(offenders).toEqual([]);
  });

  it("finds both forms", () => {
    const sample = [
      "async function f(c) {",
      "  const end = Date.now() + 3000;",
      "  for (let i = 0; i < 100; i++) {",
      "    if (c.done()) return;",
      "    await c.turn();",
      "  }",
      "  for (let i = 0; i < 100 && !c.done(); i++) await c.turn();",
      "  for (let i = 0; i < 3; i++) await c();",
      "}",
      'it("t", async () => {}, 30000);',
      "function g(gw, ms = 3000) { setTimeout(() => gw.close(), ms); }",
      "setTimeout(() => undefined, 2_000);",
      "setTimeout(() => undefined, 50);",
    ].join("\n");
    expect(fixedWaits("sample.ts", sample)).toEqual([
      "2: a deadline from Date.now()",
      "3: a polling loop of fixed turns",
      "7: a polling loop of fixed turns",
      "10: a fixed test timeout",
      "11: a real timer of a second or more",
      "12: a real timer of a second or more",
    ]);
  });

  it("gives the unit tests the patience as the runner's limit", () => {
    const config = readFileSync(join(ROOT, "vitest.config.ts"), "utf8");
    const value = /const UNIT_TIMEOUT_MS = ([\d_]+);/.exec(config)?.[1];
    expect(Number(value?.replaceAll("_", ""))).toBe(PATIENCE_MS);
    expect(config).toContain("testTimeout: UNIT_TIMEOUT_MS");
    expect(config).toContain("hookTimeout: UNIT_TIMEOUT_MS");
  });
});
