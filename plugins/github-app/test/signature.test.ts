// SPDX-License-Identifier: Apache-2.0
// Signature check of GitHub deliveries (brief M2-49 Design note 5, PLAN row "signature check
// X-Hub-Signature-256 with the test values of the GitHub documentation"). The documented values
// come from https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries,
// section "Testing the webhook payload validation", retrieved 2026-10-10 (the URL and date of
// docs/spec/11-extensions.md 10.1).

import * as crypto from "node:crypto";

import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("node:crypto", async (importOriginal) => {
  const actual = await importOriginal<typeof import("node:crypto")>();
  return { ...actual, timingSafeEqual: vi.fn(actual.timingSafeEqual) };
});

const { verifyGitHubSignature } = await import("../src/github/signature.js");

const SECRET = "It's a Secret to Everybody";
const PAYLOAD = new TextEncoder().encode("Hello, World!");
const DOCUMENTED =
  "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17";

const actual =
  await vi.importActual<typeof import("node:crypto")>("node:crypto");
const mocked = vi.mocked(crypto.timingSafeEqual);

afterEach(() => {
  mocked.mockReset();
  mocked.mockImplementation(actual.timingSafeEqual);
});

describe("verifyGitHubSignature", () => {
  it("accepts the documented test values", () => {
    expect(verifyGitHubSignature(SECRET, PAYLOAD, DOCUMENTED)).toEqual({
      ok: true,
    });
  });

  it.each([
    [
      "the last hex digit changed",
      SECRET,
      PAYLOAD,
      `${DOCUMENTED.slice(0, -1)}8`,
    ],
    [
      "the body changed by one byte",
      SECRET,
      new TextEncoder().encode("Hello, World?"),
      DOCUMENTED,
    ],
    ["another secret", "It's a Secret to Somebody", PAYLOAD, DOCUMENTED],
  ])("refuses %s as mismatch", (_name, secret, body, header) => {
    expect(verifyGitHubSignature(secret, body, header)).toEqual({
      ok: false,
      reason: "mismatch",
    });
  });

  it("treats a missing header as missing, also next to a lone X-Hub-Signature", () => {
    expect(verifyGitHubSignature(SECRET, PAYLOAD, undefined)).toEqual({
      ok: false,
      reason: "missing",
    });
    // The SHA-1 header is never read: the caller passes only X-Hub-Signature-256.
    const headers: Record<string, string> = {
      "x-hub-signature": "sha1=0123456789abcdef0123456789abcdef01234567",
    };
    expect(
      verifyGitHubSignature(SECRET, PAYLOAD, headers["x-hub-signature-256"]),
    ).toEqual({ ok: false, reason: "missing" });
  });

  it.each([
    ["sha256=757107ea"],
    ["sha256="],
    [DOCUMENTED.slice("sha256=".length)],
    [DOCUMENTED.toUpperCase().replace("SHA256=", "sha256=")],
    [`${DOCUMENTED}0`],
    [` ${DOCUMENTED}`],
    ["sha1=757107ea0eb2509fc211221cce984b8a37570b6d"],
  ])("refuses %s as malformed without throwing", (header) => {
    expect(() => verifyGitHubSignature(SECRET, PAYLOAD, header)).not.toThrow();
    expect(verifyGitHubSignature(SECRET, PAYLOAD, header)).toEqual({
      ok: false,
      reason: "malformed",
    });
    expect(mocked).not.toHaveBeenCalled();
  });

  it("refuses a repeated header as malformed", () => {
    expect(
      verifyGitHubSignature(SECRET, PAYLOAD, [DOCUMENTED, DOCUMENTED]),
    ).toEqual({ ok: false, reason: "malformed" });
  });

  it("compares through timingSafeEqual exactly once with two 32-byte buffers", () => {
    verifyGitHubSignature(SECRET, PAYLOAD, DOCUMENTED);
    verifyGitHubSignature(SECRET, PAYLOAD, `${DOCUMENTED.slice(0, -1)}8`);
    expect(mocked).toHaveBeenCalledTimes(2);
    for (const [a, b] of mocked.mock.calls) {
      expect(a.byteLength).toBe(32);
      expect(b.byteLength).toBe(32);
    }
  });

  it("decides only through timingSafeEqual", () => {
    mocked.mockImplementation(() => true);
    expect(
      verifyGitHubSignature(SECRET, PAYLOAD, `${DOCUMENTED.slice(0, -1)}8`),
    ).toEqual({ ok: true });
    mocked.mockImplementation(() => false);
    expect(verifyGitHubSignature(SECRET, PAYLOAD, DOCUMENTED)).toEqual({
      ok: false,
      reason: "mismatch",
    });
  });
});
