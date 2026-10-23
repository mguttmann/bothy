// SPDX-License-Identifier: Apache-2.0
/**
 * The defaults of the SDK equal the configuration reference (docs/decisions.d/M2-30.md D-M2-30-14).
 *
 * Every member of `DEFAULTS` names its key in `SETTING_KEYS`; the "Default" column
 * (`Standard` in the German tables) of `docs/operations/configuration.md` holds the value of
 * that key. Durations are compared in milliseconds, counts and sizes as integers.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  DEFAULTS,
  SETTING_KEYS,
  parseDuration,
  type SdkSettings,
} from "../src/config.js";

const REFERENCE = fileURLToPath(
  new URL("../../../docs/operations/configuration.md", import.meta.url),
);
const KEY_HEADERS = new Set(["Key"]);
const DEFAULT_HEADERS = new Set(["Default"]);

/** The cells of one Markdown table row, trimmed; `undefined` for a line that is no row. */
function cells(line: string): string[] | undefined {
  const trimmed = line.trim();
  if (!trimmed.startsWith("|") || !trimmed.endsWith("|")) return undefined;
  return trimmed
    .slice(1, -1)
    .split("|")
    .map((cell) => cell.trim());
}

/** Strips the code span around a cell value. */
function unquote(cell: string): string {
  return cell.replace(/^`(.*)`$/, "$1");
}

/**
 * Every key of the reference's tables with a key and a default column, to its default text.
 * A key listed twice with different defaults is an error of the reference.
 */
function referenceDefaults(text: string): Map<string, string> {
  const found = new Map<string, string>();
  let keyAt = -1;
  let defaultAt = -1;
  for (const line of text.split("\n")) {
    const row = cells(line);
    if (row === undefined) {
      keyAt = -1;
      defaultAt = -1;
      continue;
    }
    const headerKey = row.findIndex((cell) => KEY_HEADERS.has(cell));
    const headerDefault = row.findIndex((cell) => DEFAULT_HEADERS.has(cell));
    if (headerKey >= 0 && headerDefault >= 0) {
      keyAt = headerKey;
      defaultAt = headerDefault;
      continue;
    }
    if (keyAt < 0 || /^-+$/.test(row[0]?.replace(/:/g, "") ?? "")) continue;
    const key = unquote(row[keyAt] ?? "");
    const value = unquote(row[defaultAt] ?? "");
    if (!key || !value) continue;
    const earlier = found.get(key);
    if (earlier !== undefined && earlier !== value)
      throw new Error(
        `${key} has two defaults in the reference: ${earlier}, ${value}`,
      );
    found.set(key, value);
  }
  return found;
}

/** The default text of the reference as the number the SDK holds. */
function parsed(field: keyof SdkSettings, text: string): number {
  if (field.endsWith("Ms")) return parseDuration(text);
  if (!/^\d+$/.test(text)) throw new Error(`not an integer: ${text}`);
  return Number(text);
}

const reference = referenceDefaults(readFileSync(REFERENCE, "utf8"));
const fields = Object.keys(SETTING_KEYS) as (keyof SdkSettings)[];

describe("DEFAULTS against docs/operations/configuration.md", () => {
  it("reads the tables of the reference", () => {
    expect(reference.get("sdk.http_timeout")).toBe("30s");
    expect(reference.size).toBeGreaterThan(fields.length);
  });

  it("names a key for every member and a member for every key", () => {
    expect(Object.keys(DEFAULTS).sort()).toEqual([...fields].sort());
    expect(new Set(Object.values(SETTING_KEYS)).size).toBe(fields.length);
  });

  it.each(fields)("%s equals the default of its key", (field) => {
    const key = SETTING_KEYS[field];
    const text = reference.get(key);
    expect(text, `${key} is not in the reference`).toBeDefined();
    expect(DEFAULTS[field]).toBe(parsed(field, text ?? ""));
  });
});
