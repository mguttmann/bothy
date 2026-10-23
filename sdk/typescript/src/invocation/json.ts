// SPDX-License-Identifier: Apache-2.0
/**
 * Strict decoding of the token segments (private to `invocation`).
 *
 * The check of docs/spec/09-agents.md 4.5 must read a token exactly as M2-06's reference
 * verifier and the Python SDK do, so `JSON.parse` is not enough: it keeps the last of two equal
 * members (a parser keeping the first would read another header), and it cannot tell `1` from
 * `1.0` or `1e0`, which the reference refuses as a non-integer `iat`. This parser follows
 * Python's `json.loads` with a duplicate check on every object: RFC 8259 grammar, no `NaN` or
 * `Infinity`, no control character inside a string, nothing after the value.
 *
 * @module
 */

import { Buffer } from "node:buffer";

const B64URL = /^[A-Za-z0-9_-]+$/;
const NUMBER = /-?(?:0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?/y;
const INT64_MIN = -(2n ** 63n);
const INT64_MAX = 2n ** 63n - 1n;
const HEX32 = /^[0-9a-f]{32}$/;
const UTF8 = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true });

/**
 * A JSON number that is not an integer of the int64 range (a fraction, an exponent form or a
 * larger integer); only its value is kept, so it never passes an integer check.
 */
export class NonInteger {
  /**
   * Wraps the value of a number that is not an int64 integer.
   *
   * @param value - The value as JavaScript reads it.
   */
  constructor(readonly value: number) {}
}

/**
 * Decodes strict base64url without padding: the alphabet only, no impossible length and zero
 * trailing bits (encoding the bytes again must give the segment back).
 *
 * @param segment - One segment of a compact JWS.
 * @returns The decoded bytes.
 * @throws `RangeError` when the segment is not canonical base64url.
 */
export function decodeSegment(segment: string): Uint8Array {
  if (!B64URL.test(segment) || segment.length % 4 === 1)
    throw new RangeError("not base64url");
  const data = Buffer.from(segment, "base64url");
  if (data.toString("base64url") !== segment)
    throw new RangeError("not canonical base64url");
  return new Uint8Array(data);
}

/**
 * Returns the canonical form of a UUID the way Python's `uuid.UUID` reads it (`urn:uuid:` and
 * braces dropped, hyphens optional, any case), so a claim compares like in the Python SDK.
 *
 * @param text - The text.
 * @returns The lower-case hyphenated UUID, or `undefined` when the text is not one.
 */
export function canonicalUuid(text: string): string | undefined {
  const hex = text
    .replaceAll("urn:", "")
    .replaceAll("uuid:", "")
    .replace(/^[{}]+|[{}]+$/g, "")
    .replaceAll("-", "")
    .toLowerCase();
  if (!HEX32.test(hex)) return undefined;
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/** A JSON object of the strict parser: own members only, no prototype. */
export type StrictObject = Record<string, unknown>;

/**
 * Returns whether a value is a JSON object of {@link parseObject} or any plain record.
 *
 * @param value - The value.
 * @returns Whether it is a non-null object that is not an array.
 */
export function isRecord(value: unknown): value is StrictObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Parses UTF-8 bytes as exactly one JSON object with unique members on every level.
 *
 * Integers of the int64 range become numbers; every other number becomes a
 * {@link NonInteger}. Objects have no prototype, so a member named `__proto__` is plain data.
 *
 * @param data - The bytes.
 * @returns The object.
 * @throws `TypeError` or `RangeError` for invalid UTF-8, invalid JSON, a duplicate member,
 *   trailing data or a value that is not an object.
 */
export function parseObject(data: Uint8Array): StrictObject {
  const parser = new Parser(UTF8.decode(data));
  const value = parser.document();
  if (!isRecord(value)) throw new TypeError("not an object");
  return value;
}

class Parser {
  private pos = 0;

  constructor(private readonly text: string) {}

  document(): unknown {
    this.space();
    const value = this.value();
    this.space();
    if (this.pos !== this.text.length) throw new SyntaxError("trailing data");
    return value;
  }

  private fail(what: string): never {
    throw new SyntaxError(`${what} at ${String(this.pos)}`);
  }

  private space(): void {
    while (this.pos < this.text.length) {
      const c = this.text.charCodeAt(this.pos);
      // JSON whitespace only: space, tab, line feed, carriage return.
      if (c !== 0x20 && c !== 0x09 && c !== 0x0a && c !== 0x0d) return;
      this.pos++;
    }
  }

  private value(): unknown {
    const c = this.text[this.pos];
    if (c === "{") return this.object();
    if (c === "[") return this.array();
    if (c === '"') return this.string();
    if (c === "-" || (c !== undefined && c >= "0" && c <= "9"))
      return this.number();
    for (const [word, value] of [
      ["true", true],
      ["false", false],
      ["null", null],
    ] as const) {
      if (this.text.startsWith(word, this.pos)) {
        this.pos += word.length;
        return value;
      }
    }
    return this.fail("unexpected character");
  }

  private object(): StrictObject {
    const out = Object.create(null) as StrictObject;
    this.pos++;
    this.space();
    if (this.text[this.pos] === "}") {
      this.pos++;
      return out;
    }
    for (;;) {
      this.space();
      if (this.text[this.pos] !== '"') this.fail("expected a member name");
      const name = this.string();
      if (Object.hasOwn(out, name)) this.fail("duplicate member");
      this.space();
      if (this.text[this.pos] !== ":") this.fail("expected a colon");
      this.pos++;
      this.space();
      Object.defineProperty(out, name, {
        value: this.value(),
        enumerable: true,
        writable: true,
        configurable: true,
      });
      this.space();
      const next = this.text[this.pos++];
      if (next === "}") return out;
      if (next !== ",") this.fail("expected a comma");
    }
  }

  private array(): unknown[] {
    const out: unknown[] = [];
    this.pos++;
    this.space();
    if (this.text[this.pos] === "]") {
      this.pos++;
      return out;
    }
    for (;;) {
      this.space();
      out.push(this.value());
      this.space();
      const next = this.text[this.pos++];
      if (next === "]") return out;
      if (next !== ",") this.fail("expected a comma");
    }
  }

  private string(): string {
    let out = "";
    this.pos++;
    for (;;) {
      const c = this.text[this.pos++];
      if (c === undefined) return this.fail("unterminated string");
      if (c === '"') return out;
      if (c.charCodeAt(0) < 0x20) this.fail("control character in a string");
      if (c !== "\\") {
        out += c;
        continue;
      }
      const e = this.text[this.pos++];
      const simple: Record<string, string> = {
        '"': '"',
        "\\": "\\",
        "/": "/",
        b: "\b",
        f: "\f",
        n: "\n",
        r: "\r",
        t: "\t",
      };
      if (e !== undefined && e in simple) out += simple[e];
      else if (e === "u") {
        const hex = this.text.slice(this.pos, this.pos + 4);
        if (!/^[0-9a-fA-F]{4}$/.test(hex)) this.fail("invalid escape");
        out += String.fromCharCode(parseInt(hex, 16));
        this.pos += 4;
      } else this.fail("invalid escape");
    }
  }

  private number(): number | NonInteger {
    NUMBER.lastIndex = this.pos;
    const match = NUMBER.exec(this.text);
    if (match === null) return this.fail("invalid number");
    const [text, fraction, exponent] = match;
    this.pos += text.length;
    const value = Number(text);
    if (fraction !== undefined || exponent !== undefined)
      return new NonInteger(value);
    const exact = BigInt(text);
    return exact < INT64_MIN || exact > INT64_MAX
      ? new NonInteger(value)
      : value;
  }
}
