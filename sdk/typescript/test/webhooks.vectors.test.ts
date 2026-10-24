// SPDX-License-Identifier: Apache-2.0
//
// The verifier against the contract file of every verifier, docs/api/webhook-test-vectors.json
// (docs/DECISIONS.md D-M2-26-12; PLAN row M2-30: "signature verification against the test
// vectors from M2-26"). Derived secrets are built in memory and never written or printed.

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { signWebhook } from "../src/testing/webhooks.js";
import {
  WebhookVerificationError,
  parseWebhookPublicKey,
  parseWebhookSecret,
  verifyWebhook,
} from "../src/webhooks/index.js";

interface DerivationCase {
  master_hex: string;
  subscription_id: string;
  generation: number;
  derived_hex: string;
}

interface SignatureCase {
  name: string;
  webhook_id: string;
  timestamp: number;
  body: string;
  ed25519_seed_base64: string[];
  public_key: string[];
  hmac_secrets: string[];
  signature_header: string;
}

interface Vectors {
  version: number;
  derivation: DerivationCase[];
  signatures: SignatureCase[];
}

const vectors = JSON.parse(
  readFileSync(
    new URL("../../../docs/api/webhook-test-vectors.json", import.meta.url),
    "utf8",
  ),
) as Vectors;

function headersOf(c: SignatureCase, overrides: Record<string, string> = {}) {
  return {
    "webhook-id": c.webhook_id,
    "webhook-timestamp": String(c.timestamp),
    "webhook-signature": c.signature_header,
    ...overrides,
  };
}

function codeOf(fn: () => unknown): string {
  try {
    fn();
  } catch (err) {
    if (err instanceof WebhookVerificationError) return err.code;
    throw err;
  }
  return "accepted";
}

const atCase = (c: SignatureCase) => () => c.timestamp * 1000;

describe("webhook test vectors", () => {
  it("has signature and derivation cases", () => {
    expect(vectors.version).toBe(1);
    expect(vectors.signatures.length).toBeGreaterThan(0);
    expect(vectors.derivation.length).toBeGreaterThan(0);
  });

  describe.each(vectors.signatures.map((c) => [c.name, c] as const))(
    "case %s",
    (_name, c) => {
      it("verifies with its keys and secrets", () => {
        const verified = verifyWebhook({
          body: c.body,
          headers: headersOf(c),
          keys: c.public_key,
          secrets: c.hmac_secrets,
          now: atCase(c),
        });
        expect(verified.id).toBe(c.webhook_id);
        expect(verified.timestamp).toBe(c.timestamp);
      });

      it("verifies with the body as bytes", () => {
        const verified = verifyWebhook({
          body: new TextEncoder().encode(c.body),
          headers: new Headers(headersOf(c)),
          keys: c.public_key.map(parseWebhookPublicKey),
          secrets: c.hmac_secrets.map(parseWebhookSecret),
          now: atCase(c),
        });
        expect(verified.id).toBe(c.webhook_id);
      });

      it("verifies with the keys alone (v1a)", () => {
        const verified = verifyWebhook({
          body: c.body,
          headers: headersOf(c),
          keys: c.public_key,
          now: atCase(c),
        });
        expect(verified.scheme).toBe("v1a");
      });

      it.runIf(c.hmac_secrets.length > 0)(
        "verifies with the secrets alone (v1)",
        () => {
          const verified = verifyWebhook({
            body: c.body,
            headers: headersOf(c),
            secrets: c.hmac_secrets,
            now: atCase(c),
          });
          expect(verified.scheme).toBe("v1");
        },
      );

      it.runIf(c.hmac_secrets.length > 1)(
        "verifies with each secret generation alone",
        () => {
          for (const secret of c.hmac_secrets) {
            const verified = verifyWebhook({
              body: c.body,
              headers: headersOf(c),
              secrets: [secret],
              now: atCase(c),
            });
            expect(verified.scheme).toBe("v1");
          }
        },
      );

      it.runIf(c.public_key.length > 1)(
        "verifies with each signer key alone",
        () => {
          for (const key of c.public_key) {
            const verified = verifyWebhook({
              body: c.body,
              headers: headersOf(c),
              keys: [key],
              now: atCase(c),
            });
            expect(verified.scheme).toBe("v1a");
          }
        },
      );

      it("rejects the body with one byte changed", () => {
        const bytes = new TextEncoder().encode(c.body);
        for (const index of [
          0,
          Math.floor(bytes.length / 2),
          bytes.length - 1,
        ]) {
          const changed = Uint8Array.from(bytes);
          changed[index] = (changed[index] ?? 0) ^ 0x01;
          expect(
            codeOf(() =>
              verifyWebhook({
                body: changed,
                headers: headersOf(c),
                keys: c.public_key,
                secrets: c.hmac_secrets,
                now: atCase(c),
              }),
            ),
          ).toBe("signature_mismatch");
        }
      });

      it("rejects another webhook-id", () => {
        expect(
          codeOf(() =>
            verifyWebhook({
              body: c.body,
              headers: headersOf(c, {
                "webhook-id": "0192f4a0-7b1c-757c-87c8-267db4b2c8a0",
              }),
              keys: c.public_key,
              secrets: c.hmac_secrets,
              now: atCase(c),
            }),
          ),
        ).toBe("signature_mismatch");
      });

      it("rejects another timestamp", () => {
        expect(
          codeOf(() =>
            verifyWebhook({
              body: c.body,
              headers: headersOf(c, {
                "webhook-timestamp": String(c.timestamp + 1),
              }),
              keys: c.public_key,
              secrets: c.hmac_secrets,
              now: atCase(c),
            }),
          ),
        ).toBe("signature_mismatch");
      });

      it("signWebhook reproduces the header byte for byte", () => {
        const header = signWebhook({
          id: c.webhook_id,
          timestamp: c.timestamp,
          body: c.body,
          seeds: c.ed25519_seed_base64.map((s) => Buffer.from(s, "base64")),
          secrets: c.hmac_secrets,
        });
        expect(header).toBe(c.signature_header);
      });

      it("every hmac secret parses", () => {
        for (const secret of c.hmac_secrets) {
          expect(parseWebhookSecret(secret).length).toBeGreaterThan(0);
        }
      });
    },
  );

  it("parses the whsec_ form of every derived secret to its bytes", () => {
    for (const d of vectors.derivation) {
      const bytes = Buffer.from(d.derived_hex, "hex");
      const parsed = parseWebhookSecret("whsec_" + bytes.toString("base64"));
      // Compared as hex of the bytes, which the file holds anyway; the whsec_ string is not
      // printed on failure.
      expect(Buffer.from(parsed).toString("hex")).toBe(d.derived_hex);
    }
  });
});
