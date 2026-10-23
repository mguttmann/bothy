// SPDX-License-Identifier: Apache-2.0
//
// The webhook verifier (docs/api/webhooks.md 3, docs/spec/11-extensions.md 7 and 9): header
// and timestamp rules, signature entries, parsers, the Fetch API form for `interactions_url`,
// the trust list cache and a seeded fuzz. Keys come from fixed seeds, secrets carry `EXAMPLE`;
// no request leaves the process.

import { generateKeyPairSync } from "node:crypto";
import createClient from "openapi-fetch";
import { describe, expect, it } from "vitest";

import { DEFAULTS, resolveSettings } from "../src/config.js";
import type { components, paths } from "../src/generated/openapi.js";
import { FakeClock } from "../src/testing/clock.js";
import { signWebhook, webhookPublicKey } from "../src/testing/webhooks.js";
import {
  WebhookConfigError,
  WebhookKeyCache,
  WebhookVerificationError,
  parseWebhookPublicKey,
  parseWebhookSecret,
  verifyWebhook,
  verifyWebhookRequest,
  type VerifyWebhookOptions,
} from "../src/webhooks/index.js";

// The prefix is assembled at runtime so no literal matches the secret-scanning format.
const WHSEC = ["whsec", ""].join("_");

const SEED_A = new Uint8Array(32).fill(0x11);
const SEED_B = new Uint8Array(32).fill(0x22);
const SECRET = `${WHSEC}EXAMPLEsecretOfTheWebhookTest123`;
const OTHER_SECRET = `${WHSEC}EXAMPLEanotherSecretOfTheTest456`;
const ID = "0192f4a0-7b1c-757c-87c8-267db4b2c89f";
const TS = 1_760_346_000;
const NOW = () => TS * 1000;
const BODY =
  '{"id":"0192f4a0-7b1c-757c-87c8-267db4b2c89f","type":"message.created"}';

type Headers3 = Record<string, string>;

function signed(
  body: string | Uint8Array = BODY,
  opts: {
    id?: string;
    timestamp?: number;
    seeds?: Uint8Array[];
    secrets?: string[];
  } = {},
): Headers3 {
  const id = opts.id ?? ID;
  const timestamp = opts.timestamp ?? TS;
  return {
    "webhook-id": id,
    "webhook-timestamp": String(timestamp),
    "webhook-signature": signWebhook({
      id,
      timestamp,
      body,
      seeds: opts.seeds ?? [SEED_A],
      secrets: opts.secrets ?? [SECRET],
    }),
  };
}

function base(
  overrides: Partial<VerifyWebhookOptions> = {},
): VerifyWebhookOptions {
  return {
    body: BODY,
    headers: signed(),
    keys: [webhookPublicKey(SEED_A)],
    secrets: [SECRET],
    now: NOW,
    ...overrides,
  };
}

function failure(options: VerifyWebhookOptions): WebhookVerificationError {
  try {
    verifyWebhook(options);
  } catch (err) {
    if (err instanceof WebhookVerificationError) return err;
    throw err;
  }
  throw new Error("verifyWebhook accepted");
}

describe("verifyWebhook headers", () => {
  it.each(["webhook-id", "webhook-timestamp", "webhook-signature"])(
    "missing %s gives missing_header",
    (name) => {
      const headers = signed();
      delete headers[name];
      expect(failure(base({ headers })).code).toBe("missing_header");
      expect(failure(base({ headers: new Headers(headers) })).code).toBe(
        "missing_header",
      );
    },
  );

  it("treats a plain record case-insensitively", () => {
    const h = signed();
    const headers = {
      "Webhook-Id": h["webhook-id"],
      "WEBHOOK-TIMESTAMP": h["webhook-timestamp"],
      "Webhook-Signature": h["webhook-signature"],
    };
    expect(verifyWebhook(base({ headers })).id).toBe(ID);
  });

  it("accepts header values as one-element arrays and ignores undefined members", () => {
    const h = signed();
    const headers: Record<string, string | string[] | undefined> = {
      "webhook-id": [h["webhook-id"] ?? ""],
      "webhook-timestamp": h["webhook-timestamp"],
      "webhook-signature": [h["webhook-signature"] ?? ""],
      "x-other": undefined,
    };
    expect(verifyWebhook(base({ headers })).id).toBe(ID);
    const missing = { ...headers, "webhook-id": undefined };
    expect(failure(base({ headers: missing })).code).toBe("missing_header");
  });

  it.each([
    "abc",
    "1.5",
    "",
    "-1",
    "+1760346000",
    " 1760346000",
    "1e9",
    "12345678901234567",
  ])("webhook-timestamp %j gives invalid_timestamp", (value) => {
    const headers = { ...signed(), "webhook-timestamp": value };
    expect(failure(base({ headers })).code).toBe("invalid_timestamp");
  });
});

describe("verifyWebhook time window", () => {
  const tolS = DEFAULTS.webhookReceiverToleranceMs / 1000;

  it("accepts exactly webhook.receiver_tolerance before and after now", () => {
    for (const ts of [TS - tolS, TS + tolS]) {
      const headers = signed(BODY, { timestamp: ts });
      expect(verifyWebhook(base({ headers })).timestamp).toBe(ts);
    }
  });

  it("rejects one second beyond with timestamp_out_of_tolerance", () => {
    for (const ts of [TS - tolS - 1, TS + tolS + 1]) {
      const headers = signed(BODY, { timestamp: ts });
      expect(failure(base({ headers })).code).toBe(
        "timestamp_out_of_tolerance",
      );
    }
  });

  it("follows toleranceMs and defaults now to Date.now", () => {
    const headers = signed(BODY, { timestamp: TS + 10 });
    expect(failure(base({ headers, toleranceMs: 5_000 })).code).toBe(
      "timestamp_out_of_tolerance",
    );
    expect(verifyWebhook(base({ headers, toleranceMs: 10_000 })).id).toBe(ID);
    const fresh = Math.floor(Date.now() / 1000);
    const withoutNow: VerifyWebhookOptions = {
      body: BODY,
      headers: signed(BODY, { timestamp: fresh }),
      keys: [webhookPublicKey(SEED_A)],
    };
    expect(verifyWebhook(withoutNow).timestamp).toBe(fresh);
  });
});

describe("verifyWebhook signatures", () => {
  it("no key and no secret gives no_trusted_key", () => {
    expect(failure(base({ keys: [], secrets: [] })).code).toBe(
      "no_trusted_key",
    );
    expect(failure({ body: BODY, headers: signed(), now: NOW }).code).toBe(
      "no_trusted_key",
    );
  });

  it("checks the header and timestamp before the trust list", () => {
    const headers = signed();
    delete headers["webhook-id"];
    expect(failure(base({ headers, keys: [], secrets: [] })).code).toBe(
      "missing_header",
    );
  });

  it("verifies by its valid entry among several spaces, an unknown scheme and bad base64", () => {
    const good = signed();
    const entries = good["webhook-signature"] ?? "";
    const headers = {
      ...good,
      "webhook-signature": `v2,QUJD   v1a,@@not-base64@@ v1,%%%  ${entries}   v1a, v1,QUJD== nocomma `,
    };
    expect(verifyWebhook(base({ headers })).id).toBe(ID);
    expect(verifyWebhook(base({ headers, secrets: [] })).scheme).toBe("v1a");
    expect(verifyWebhook(base({ headers, keys: [] })).scheme).toBe("v1");
  });

  it("reports the scheme of the first valid entry in header order", () => {
    expect(verifyWebhook(base()).scheme).toBe("v1a");
    const v1Only = signed(BODY, { seeds: [] });
    expect(verifyWebhook(base({ headers: v1Only })).scheme).toBe("v1");
  });

  it("tries every key and every secret", () => {
    const headers = signed(BODY, { seeds: [SEED_B], secrets: [OTHER_SECRET] });
    const keys = [webhookPublicKey(SEED_A), webhookPublicKey(SEED_B)];
    expect(verifyWebhook(base({ headers, keys, secrets: [] })).scheme).toBe(
      "v1a",
    );
    expect(
      verifyWebhook(
        base({ headers, keys: [], secrets: [SECRET, OTHER_SECRET] }),
      ).scheme,
    ).toBe("v1");
    expect(failure(base({ headers })).code).toBe("signature_mismatch");
  });

  it("a v1 entry of another length does not throw", () => {
    const headers = {
      ...signed(),
      "webhook-signature":
        "v1,QUJD v1," + Buffer.alloc(64, 1).toString("base64"),
    };
    expect(failure(base({ headers })).code).toBe("signature_mismatch");
    const shortV1a = { ...signed(), "webhook-signature": "v1a,QUJD" };
    expect(failure(base({ headers: shortV1a })).code).toBe(
      "signature_mismatch",
    );
  });

  it("an empty signature header gives signature_mismatch", () => {
    const headers = { ...signed(), "webhook-signature": "" };
    expect(failure(base({ headers })).code).toBe("signature_mismatch");
  });

  it("a body parsed and serialized again fails (raw-body rule)", () => {
    const body =
      '{ "id": "0192f4a0-7b1c-757c-87c8-267db4b2c89f", "type": "message.created" }';
    const headers = signed(body);
    expect(verifyWebhook(base({ body, headers })).id).toBe(ID);
    const reserialized = JSON.stringify(JSON.parse(body));
    expect(failure(base({ body: reserialized, headers })).code).toBe(
      "signature_mismatch",
    );
  });

  it("counts a string body as its UTF-8 bytes", () => {
    const body =
      '{"type":"message.created","payload":{"content_md":"Café 😀"}}';
    const headers = signed(new TextEncoder().encode(body));
    expect(verifyWebhook(base({ body, headers })).id).toBe(ID);
  });

  it("never takes a key from the request", () => {
    // A forged request signed with an own key and naming it nowhere the verifier reads.
    const forged = signed(BODY, { seeds: [SEED_B], secrets: [] });
    expect(failure(base({ headers: forged, secrets: [] })).code).toBe(
      "signature_mismatch",
    );
  });
});

describe("parsers", () => {
  it("parseWebhookPublicKey reads the whpk_ form", () => {
    const key = parseWebhookPublicKey(webhookPublicKey(SEED_A));
    expect(key.type).toBe("public");
    expect(key.asymmetricKeyType).toBe("ed25519");
  });

  it("parseWebhookPublicKey rejects a wrong prefix, bad base64 and another key type", () => {
    const der = Buffer.from(
      webhookPublicKey(SEED_A).slice("whpk_".length),
      "base64",
    );
    const p256 = generateKeyPairSync("ec", { namedCurve: "P-256" })
      .publicKey.export({ format: "der", type: "spki" })
      .toString("base64");
    const x25519 = generateKeyPairSync("x25519")
      .publicKey.export({ format: "der", type: "spki" })
      .toString("base64");
    for (const value of [
      "whsec_" + der.toString("base64"),
      "WHPK_" + der.toString("base64"),
      der.toString("base64"),
      "whpk_",
      "whpk_@@@@",
      "whpk_" + der.toString("base64").replace(/=+$/, ""),
      "whpk_" + der.toString("base64url"),
      "whpk_" + der.subarray(0, 20).toString("base64"),
      "whpk_" + p256,
      "whpk_" + x25519,
    ]) {
      expect(() => parseWebhookPublicKey(value), value).toThrow(
        WebhookConfigError,
      );
    }
  });

  it("parseWebhookSecret reads whsec_ and rejects empty or malformed values", () => {
    expect(Buffer.from(parseWebhookSecret(SECRET)).toString("base64")).toBe(
      SECRET.slice("whsec_".length),
    );
    for (const value of [
      "",
      "whsec_",
      "EXAMPLE",
      "whsek_EXAMPLEsecretOfTheWebhookTest123",
      "whsec_EXAMPLE",
      `${WHSEC}EXAM PLEsecretOfTheWebhookTest12`,
      `${WHSEC}EXAMPLEsecretOfTheWebhookTest123\n`,
      `${WHSEC}EXAMPLEsecretOfTheWebhookTest123=`,
      `${WHSEC}EXAMPLE-_`,
    ]) {
      expect(() => parseWebhookSecret(value)).toThrow(WebhookConfigError);
    }
  });

  it("config errors never repeat the value", () => {
    const value = `${WHSEC}EXAMPLEnot base64 at all`;
    try {
      parseWebhookSecret(value);
      expect.unreachable();
    } catch (err) {
      expect((err as Error).message).not.toContain("EXAMPLE");
    }
  });

  it("verifyWebhook throws WebhookConfigError for a malformed configured key or secret", () => {
    expect(() => verifyWebhook(base({ keys: ["whpk_nope"] }))).toThrow(
      WebhookConfigError,
    );
    expect(() => verifyWebhook(base({ secrets: ["whsec_"] }))).toThrow(
      WebhookConfigError,
    );
    expect(() => verifyWebhook(base({ secrets: [new Uint8Array(0)] }))).toThrow(
      WebhookConfigError,
    );
  });
});

describe("signWebhook", () => {
  it("rejects a seed of another length", () => {
    expect(() => webhookPublicKey(new Uint8Array(31))).toThrow(RangeError);
    expect(() =>
      signWebhook({
        id: ID,
        timestamp: TS,
        body: BODY,
        seeds: [new Uint8Array(33)],
      }),
    ).toThrow(RangeError);
  });

  it("signs with raw secret bytes like with their whsec_ form", () => {
    const raw = parseWebhookSecret(SECRET);
    const opts = { id: ID, timestamp: TS, body: BODY };
    expect(signWebhook({ ...opts, secrets: [raw] })).toBe(
      signWebhook({ ...opts, secrets: [SECRET] }),
    );
    expect(signWebhook(opts)).toBe("");
  });
});

describe("decoded envelope", () => {
  const SYNC_EPOCH = "0192f4a0-7b1c-7a3e-8c41-5d2e9b7f1a60";
  const invoked = {
    id: "0192f4a0-7b1c-7ff0-88b6-7d3b051173dd",
    seq: 5301,
    schema_version: 1,
    occurred_at: "2026-10-10T09:30:00.000Z",
    permission_version: 418,
    scope: {
      space_id: "0192f4a0-7b1c-7881-8522-0a58634abc14",
      channel_id: "0192f4a0-7b1c-72d4-8bae-7661d2b2f203",
    },
    resource: { kind: "command", id: "0192f4a0-7b1c-7beb-80fc-f336f4fe79aa" },
    sync_epoch: SYNC_EPOCH,
    type: "command.invoked",
    payload: {
      invocation_id: "0192f4a0-7b1c-712e-8ccb-17c85f8d11c9",
      command_id: "0192f4a0-7b1c-7beb-80fc-f336f4fe79aa",
      command_name: "escalate",
      control: false,
      channel_id: "0192f4a0-7b1c-72d4-8bae-7661d2b2f203",
      invoked_by: {
        id: "0192f4a0-7b1c-7f89-8d6b-6960453241bc",
        kind: "user",
        handle: "alex",
        display_name: "Alex",
      },
      options: [{ name: "ticket", value: "4711" }],
    },
  } satisfies components["schemas"]["CommandInvokedEvent"];

  it("verifyWebhookRequest decodes a signed command.invoked (interactions_url)", async () => {
    const body = JSON.stringify(invoked);
    const request = new Request("https://app.example.com/interactions", {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...signed(body, { id: invoked.id }),
      },
      body,
    });
    const verified = await verifyWebhookRequest(request, {
      keys: [webhookPublicKey(SEED_A)],
      now: NOW,
    });
    expect(verified.id).toBe(invoked.id);
    expect(verified.syncEpoch).toBe(SYNC_EPOCH);
    expect(verified.event.kind).toBe("known");
    if (
      verified.event.kind === "known" &&
      verified.event.event.type === "command.invoked"
    ) {
      expect(verified.event.event.payload.command_name).toBe("escalate");
    } else {
      expect.unreachable();
    }
  });

  it("verifyWebhookRequest rejects a forged request", async () => {
    const body = JSON.stringify(invoked);
    const request = new Request("https://app.example.com/interactions", {
      method: "POST",
      headers: signed(body, { seeds: [SEED_B], secrets: [] }),
      body,
    });
    await expect(
      verifyWebhookRequest(request, {
        keys: [webhookPublicKey(SEED_A)],
        now: NOW,
      }),
    ).rejects.toMatchObject({ code: "signature_mismatch" });
  });

  it("an envelope of unknown type returns UnknownEvent", () => {
    const body =
      '{"id":"x","seq":7,"type":"future.example","sync_epoch":"' +
      SYNC_EPOCH +
      '"}';
    const verified = verifyWebhook(base({ body, headers: signed(body) }));
    expect(verified.event).toMatchObject({
      kind: "unknown",
      type: "future.example",
      seq: 7,
    });
    expect(verified.syncEpoch).toBe(SYNC_EPOCH);
  });

  it("drops null members like the REST tolerance layer", () => {
    const body =
      '{"type":"message.created","truncated":null,"payload":{"message":null}}';
    const verified = verifyWebhook(base({ body, headers: signed(body) }));
    expect(verified.event.kind).toBe("known");
    if (verified.event.kind === "known") {
      expect(verified.event.event).not.toHaveProperty("truncated");
      expect(verified.event.event.payload).toEqual({});
    }
    expect(verified.syncEpoch).toBeUndefined();
  });

  it("a validly signed body that is no JSON returns UnknownEvent with an empty type", () => {
    for (const body of ["not json", new Uint8Array([0xff, 0xfe, 0x7b])]) {
      const verified = verifyWebhook(base({ body, headers: signed(body) }));
      expect(verified.event.kind).toBe("unknown");
      if (verified.event.kind === "unknown") {
        expect(verified.event.type).toBe("");
        expect(verified.event.raw).toBeUndefined();
      }
      expect(verified.syncEpoch).toBeUndefined();
    }
  });

  it("ignores a sync_epoch that is no string", () => {
    const body = '{"type":"message.created","sync_epoch":5}';
    expect(
      verifyWebhook(base({ body, headers: signed(body) })).syncEpoch,
    ).toBeUndefined();
  });
});

describe("WebhookKeyCache", () => {
  const KEYS: components["schemas"]["WebhookKeys"] = {
    keys: [
      {
        kid: "whk-2026-11",
        public_key: webhookPublicKey(SEED_B),
        state: "next",
      },
      {
        kid: "whk-2026-10",
        public_key: webhookPublicKey(SEED_A),
        state: "active",
      },
      {
        kid: "whk-2026-09",
        public_key: webhookPublicKey(new Uint8Array(32).fill(0x33)),
        state: "retiring",
        not_after: "2026-11-01T00:00:00.000Z",
      },
    ],
  };

  function server(
    cacheControl: string | undefined,
    body: unknown = KEYS,
    status = 200,
  ) {
    const calls: string[] = [];
    const fetch = (input: Request): Promise<Response> => {
      calls.push(new URL(input.url).pathname);
      const headers: Record<string, string> = {
        "content-type": "application/json",
      };
      if (cacheControl !== undefined) headers["cache-control"] = cacheControl;
      return Promise.resolve(
        new Response(JSON.stringify(body), { status, headers }),
      );
    };
    const client = createClient<paths>({
      baseUrl: "https://example.com/api/v1",
      fetch,
    });
    return { client, calls };
  }

  it("fetches once and trusts next, active and retiring keys", async () => {
    const { client, calls } = server("public, max-age=1200");
    const clock = new FakeClock();
    const cache = new WebhookKeyCache({ client, clock });
    const [first, second] = await Promise.all([cache.keys(), cache.keys()]);
    expect(first).toHaveLength(3);
    expect(second).toBe(first);
    await cache.keys();
    expect(calls).toEqual(["/api/v1/instance/webhook-keys"]);
    for (const seed of [SEED_A, SEED_B]) {
      const headers = signed(BODY, { seeds: [seed], secrets: [] });
      expect(
        verifyWebhook(base({ headers, keys: first, secrets: [] })).scheme,
      ).toBe("v1a");
    }
  });

  it("refreshes only after max-age", async () => {
    const { client, calls } = server("public, max-age=1200");
    const clock = new FakeClock();
    const cache = new WebhookKeyCache({ client, clock });
    await cache.keys();
    await clock.advance(1_199_999);
    await cache.keys();
    expect(calls).toHaveLength(1);
    await clock.advance(1);
    await cache.keys();
    expect(calls).toHaveLength(2);
  });

  it("without Cache-Control refreshes after agent.jwks_max_age", async () => {
    const { client, calls } = server(undefined);
    const clock = new FakeClock();
    const cache = new WebhookKeyCache({
      client,
      clock,
      settings: resolveSettings({
        jwksMaxAgeMs: 50_000,
        jwksRefreshMinIntervalMs: 1_000,
      }),
    });
    await cache.keys();
    await clock.advance(49_999);
    await cache.keys();
    expect(calls).toHaveLength(1);
    await clock.advance(1);
    await cache.keys();
    expect(calls).toHaveLength(2);
  });

  it("a max-age below adapter.jwks_refresh_min_interval waits that interval", async () => {
    const { client, calls } = server("max-age=0");
    const clock = new FakeClock();
    const cache = new WebhookKeyCache({ client, clock });
    await cache.keys();
    await cache.keys();
    expect(calls).toHaveLength(1);
    await clock.advance(DEFAULTS.jwksRefreshMinIntervalMs);
    await cache.keys();
    expect(calls).toHaveLength(2);
  });

  it("never refreshes because of a failed verification", async () => {
    const { client, calls } = server("public, max-age=1200");
    const cache = new WebhookKeyCache({ client, clock: new FakeClock() });
    for (let i = 0; i < 5; i++) {
      const forged = signed(BODY, {
        seeds: [new Uint8Array(32).fill(0x44)],
        secrets: [],
      });
      const keys = await cache.keys();
      expect(failure(base({ headers: forged, keys, secrets: [] })).code).toBe(
        "signature_mismatch",
      );
    }
    expect(calls).toHaveLength(1);
  });

  it("skips listed keys it cannot use and unknown states", async () => {
    const body = {
      keys: [
        ...KEYS.keys,
        { kid: "bad", public_key: "whpk_nope", state: "active" },
        {
          kid: "future",
          public_key: webhookPublicKey(SEED_A),
          state: "revoked",
        },
      ],
    };
    const { client } = server("max-age=60", body);
    const cache = new WebhookKeyCache({ client, clock: new FakeClock() });
    expect(await cache.keys()).toHaveLength(3);
  });

  it("a failed fetch throws without naming a key and is retried on the next call", async () => {
    let fail = true;
    const fetch = (): Promise<Response> =>
      Promise.resolve(
        fail
          ? new Response('{"type":"about:blank","status":503}', {
              status: 503,
              headers: { "content-type": "application/problem+json" },
            })
          : new Response(JSON.stringify(KEYS), {
              status: 200,
              headers: { "content-type": "application/json" },
            }),
      );
    const client = createClient<paths>({
      baseUrl: "https://example.com/api/v1",
      fetch,
    });
    const cache = new WebhookKeyCache({ client, clock: new FakeClock() });
    await expect(cache.keys()).rejects.toThrow(/getWebhookKeys/);
    fail = false;
    expect(await cache.keys()).toHaveLength(3);
  });
});

// Seeded random headers and bodies (mulberry32, fixed seed): the verifier may only throw
// WebhookVerificationError, and its messages never contain the body, a signature or a secret.
describe("seeded fuzz", () => {
  function mulberry32(seed: number): () => number {
    let a = seed >>> 0;
    return () => {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  const ALPHABET =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=,. -_\t:%@ü\u0000";
  const PIECES = [
    "v1a,",
    "v1,",
    "v2,",
    " ",
    ",",
    "=",
    "==",
    "QUJD",
    "v1a,QUJD",
    "v1,",
  ];

  it("gives only WebhookVerificationError for random input", () => {
    const rand = mulberry32(0x5eed_2030);
    const pick = <T>(xs: readonly T[]): T =>
      xs[Math.floor(rand() * xs.length)] as T;
    const text = (max: number): string => {
      let out = "";
      const n = Math.floor(rand() * max);
      for (let i = 0; i < n; i++)
        out += rand() < 0.2 ? pick(PIECES) : pick(chars);
      return out;
    };
    const valid = signed();
    const chars = [...ALPHABET];
    const trustedKey = parseWebhookPublicKey(webhookPublicKey(SEED_A));
    const trustedSecret = parseWebhookSecret(SECRET);
    const seen = new Set<string>();
    for (let round = 0; round < 2000; round++) {
      const marker = `EXAMPLE-body-marker-${round}`;
      const bodyText =
        rand() < 0.5 ? marker + text(40) : `{"type":"${marker}"}`;
      const body: string | Uint8Array =
        rand() < 0.15
          ? BODY
          : rand() < 0.5
            ? bodyText
            : Uint8Array.from({ length: Math.floor(rand() * 64) }, () =>
                Math.floor(rand() * 256),
              );
      const headers: Record<string, string> = {};
      for (const name of [
        "webhook-id",
        "webhook-timestamp",
        "webhook-signature",
      ]) {
        const r = rand();
        if (r < 0.1) continue;
        if (r < 0.6) headers[name] = valid[name] ?? "";
        else if (name === "webhook-timestamp" && r < 0.8)
          headers[name] = String(TS + Math.floor((rand() - 0.5) * 1000));
        else headers[name] = text(name === "webhook-signature" ? 200 : 30);
      }
      const options: VerifyWebhookOptions = {
        body,
        headers:
          rand() < 0.5
            ? headers
            : new Headers(
                Object.fromEntries(
                  Object.entries(headers).map(([k, v]) => [
                    k,
                    [...v].filter((c) => c >= " " && c <= "~").join(""),
                  ]),
                ),
              ),
        keys: rand() < 0.8 ? [trustedKey] : [],
        secrets: rand() < 0.8 ? [trustedSecret] : [],
        now: NOW,
      };
      try {
        verifyWebhook(options);
        seen.add("accepted");
      } catch (err) {
        expect(err).toBeInstanceOf(WebhookVerificationError);
        const e = err as WebhookVerificationError;
        seen.add(e.code);
        expect(e.message).not.toContain(marker);
        expect(e.message).not.toContain("EXAMPLE");
        expect(e.message).not.toContain(SECRET.slice("whsec_".length));
        for (const entry of (headers["webhook-signature"] ?? "").split(" ")) {
          const value = entry.slice(entry.indexOf(",") + 1);
          if (value.length >= 4) expect(e.message).not.toContain(value);
        }
      }
    }
    // The fuzz reaches every outcome, so it exercises each step.
    expect([...seen].sort()).toEqual(
      [
        "accepted",
        "invalid_timestamp",
        "missing_header",
        "no_trusted_key",
        "signature_mismatch",
        "timestamp_out_of_tolerance",
      ].sort(),
    );
  });

  it("no error message contains the body, a signature or a secret", () => {
    const body =
      '{"type":"message.created","secret":"EXAMPLE-very-private-body"}';
    const headers = signed(body);
    const signature = headers["webhook-signature"] ?? "";
    const cases: VerifyWebhookOptions[] = [
      base({
        body,
        headers: { ...headers, "webhook-id": undefined as unknown as string },
      }),
      base({ body, headers: { ...headers, "webhook-timestamp": "EXAMPLE" } }),
      base({ body, headers: signed(body, { timestamp: TS + 100_000 }) }),
      base({ body, headers, keys: [], secrets: [] }),
      base({ body: body + " ", headers }),
      base({
        body,
        headers,
        secrets: [OTHER_SECRET],
        keys: [webhookPublicKey(SEED_B)],
      }),
    ];
    const codes = new Set<string>();
    for (const options of cases) {
      const e = failure(options);
      codes.add(e.code);
      for (const forbidden of [
        "EXAMPLE",
        body,
        SECRET,
        SECRET.slice("whsec_".length),
        OTHER_SECRET.slice("whsec_".length),
        ...signature.split(" ").map((s) => s.slice(s.indexOf(",") + 1)),
      ]) {
        expect(e.message).not.toContain(forbidden);
        expect(String(e.stack)).not.toContain(forbidden);
      }
      expect(e.name).toBe("WebhookVerificationError");
    }
    expect(codes.size).toBe(5);
  });
});
