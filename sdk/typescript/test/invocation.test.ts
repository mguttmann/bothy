// SPDX-License-Identifier: Apache-2.0
//
// The invocation check of docs/spec/09-agents.md 4.5 against the cases of M2-06's reference
// verifier, ported from `sdk/python/tests/test_invocation.py` (the SDK verifiers must agree with
// the reference on each case), plus the cases of the M2-30 brief and a cross-check against the
// FakeServer's own issuer. Tokens are never printed: case names identify a failure.

import {
  createHash,
  createHmac,
  generateKeyPairSync,
  sign,
  type KeyObject,
} from "node:crypto";

import { describe, expect, it } from "vitest";

import { Secret } from "../src/auth/secret.js";
import { createApiClient } from "../src/client.js";
import type { components } from "../src/generated/openapi.js";
import { uuidv7 } from "../src/ids.js";
import {
  TOKEN_EVENT_TYPES,
  TokenVerdict,
  InvocationVerifier,
  MemoryInboxStore,
  compareActual,
  expectationFor,
  jwksFetcher,
  storedClaims,
  triggererLoader,
  type Actual,
  type Expectation,
  type JwksFetcher,
  type TriggererLoader,
} from "../src/invocation/index.js";
import type { LogFields, Logger } from "../src/logger.js";
import { FakeClock } from "../src/testing/clock.js";
import { FakeServer } from "../src/testing/fakeserver.js";

type S = components["schemas"];
type Json = Record<string, unknown>;

// The values of the reference tests: agent.invocation_token_ttl, adapter.invocation_token_leeway,
// adapter.jwks_refresh_min_interval and the max-age of the JWKS (agent.jwks_max_age), seconds.
const TTL = 60;
const LEEWAY = 30;
const REFRESH = 300;
const MAX_AGE = 1200;

const SETTINGS = {
  invocationTokenLeewayMs: LEEWAY * 1000,
  jwksRefreshMinIntervalMs: REFRESH * 1000,
};
const ACTUAL: Actual = { billingMode: "organization", toolClass: "scoped" };
const TYP = "agent-invocation+jwt";
const enc = new TextEncoder();

function b64(data: Uint8Array | string): string {
  return Buffer.from(
    typeof data === "string" ? enc.encode(data) : data,
  ).toString("base64url");
}

function unb64(text: string): Uint8Array {
  return new Uint8Array(Buffer.from(text, "base64url"));
}

function publicBytes(key: KeyObject): Uint8Array {
  return unb64(String(key.export({ format: "jwk" }).x));
}

/** A compact JWS of `header` (as given) and `payload` with an Ed25519 signature. */
function signEd(header: string, payload: Uint8Array, key: KeyObject): string {
  const input = `${b64(header)}.${b64(payload)}`;
  return `${input}.${b64(sign(null, Buffer.from(input, "ascii"), key))}`;
}

/** Algorithm confusion: HS256 keyed with the bytes of the published public key. */
function signHs256WithPublicKey(
  header: string,
  payload: Uint8Array,
  key: KeyObject,
): string {
  const input = `${b64(header)}.${b64(payload)}`;
  const mac = createHmac("sha256", publicBytes(key)).update(input).digest();
  return `${input}.${b64(mac)}`;
}

/** A header JSON with `kid`; `null` leaves `alg` or `typ` out. */
function header(
  kid: string,
  alg: string | null = "EdDSA",
  typ: string | null = TYP,
): string {
  const h: Json = { kid };
  if (alg !== null) h.alg = alg;
  if (typ !== null) h.typ = typ;
  return JSON.stringify(h);
}

function payloadOf(token: string): Json {
  return JSON.parse(
    Buffer.from(token.split(".")[1] ?? "", "base64url").toString("utf8"),
  ) as Json;
}

/** The server side: Ed25519 keys by kid, the active kid, a JWKS and grants turned into tokens. */
class Issuer {
  readonly keys = new Map<string, KeyObject>();
  readonly published = new Set<string>();
  active: string;
  readonly grant: Json;

  constructor(
    private readonly now: () => number,
    readonly instanceId: string,
    readonly agentId: string,
    readonly ttl = TTL,
  ) {
    this.active = this.addKey();
    this.grant = {
      sub: crypto.randomUUID(),
      trigger_kind: "message",
      run_id: uuidv7(),
      message_id: uuidv7(),
      thread_id: uuidv7(),
      channel_id: uuidv7(),
      space_id: uuidv7(),
      level: "trigger",
      billing_mode: "organization",
      tool_class: "scoped",
      permission_version: 4,
    };
  }

  addKey({ active = true, published = true } = {}): string {
    const key = generateKeyPairSync("ed25519").privateKey;
    const kid = createHash("sha256").update(publicBytes(key)).digest("hex");
    this.keys.set(kid, key);
    if (published) this.published.add(kid);
    if (active) this.active = kid;
    return kid;
  }

  key(kid = this.active): KeyObject {
    const key = this.keys.get(kid);
    if (key === undefined) throw new Error("no such key");
    return key;
  }

  jwks(): { keys: Json[] } {
    return {
      keys: [...this.published].sort().map((kid) => ({
        kty: "OKP",
        crv: "Ed25519",
        kid,
        alg: "EdDSA",
        use: "sig",
        x: b64(publicBytes(this.key(kid))),
      })),
    };
  }

  /** The claims of a fresh token for the grant with `changes` applied (`null` removes). */
  claims(changes: Json = {}): Json {
    const iat = Math.floor(this.now() / 1000);
    const c: Json = {
      iss: this.instanceId,
      aud: this.agentId,
      jti: uuidv7(),
      iat,
      exp: iat + this.ttl,
      ...this.grant,
    };
    for (const [name, value] of Object.entries(changes)) {
      if (value === null) delete c[name];
      else c[name] = value;
    }
    return c;
  }

  raw(claims?: Json, kid = this.active): string {
    const body = enc.encode(JSON.stringify(claims ?? this.claims()));
    return signEd(header(kid), body, this.key(kid));
  }

  token(changes: Json = {}): Secret {
    return new Secret(this.raw(this.claims(changes)));
  }

  resign(token: string, change: (m: Json) => void): string {
    const claims = payloadOf(token);
    change(claims);
    return this.raw(claims);
  }
}

/** An in-test JWKS fetcher that counts its calls. */
class Jwks {
  calls = 0;
  extra: Json[] = [];

  constructor(
    private readonly issuer: Issuer,
    public maxAgeMs: number | undefined,
  ) {}

  readonly fetch: JwksFetcher = () => {
    this.calls++;
    const body = { keys: [...this.extra, ...this.issuer.jwks().keys] };
    return Promise.resolve([body, this.maxAgeMs] as const);
  };
}

/** A logger that keeps every line with its fields as text. */
class RecordingLogger implements Logger {
  readonly lines: string[] = [];
  private add(msg: string, fields?: LogFields): void {
    this.lines.push(`${msg} ${JSON.stringify(fields ?? {})}`);
  }
  debug(msg: string, fields?: LogFields): void {
    this.add(msg, fields);
  }
  info(msg: string, fields?: LogFields): void {
    this.add(msg, fields);
  }
  warn(msg: string, fields?: LogFields): void {
    this.add(msg, fields);
  }
  error(msg: string, fields?: LogFields): void {
    this.add(msg, fields);
  }
}

/** A clock, an issuer, a JWKS fetcher and a verifier of the issuer's agent. */
class Env {
  readonly clock = new FakeClock(Date.UTC(2026, 9, 20, 9, 0));
  readonly issuer = new Issuer(() => this.clock.now(), uuidv7(), uuidv7());
  readonly jwks: Jwks;
  readonly store = new MemoryInboxStore();
  readonly logger = new RecordingLogger();
  readonly verifier: InvocationVerifier;

  constructor(
    options: {
      maxAgeMs?: number | undefined;
      allowlist?: TriggererLoader;
    } = {},
  ) {
    const { allowlist } = options;
    // An explicit undefined means "no max-age", so no default parameter here.
    const maxAgeMs = Object.hasOwn(options, "maxAgeMs")
      ? options.maxAgeMs
      : MAX_AGE * 1000;
    this.jwks = new Jwks(this.issuer, maxAgeMs);
    this.verifier = new InvocationVerifier({
      instanceId: this.issuer.instanceId,
      agentId: this.issuer.agentId,
      jwks: this.jwks.fetch,
      replay: this.store,
      settings: SETTINGS,
      clock: this.clock,
      ...(allowlist === undefined ? {} : { allowlist }),
      logger: this.logger,
    });
  }

  full(): Expectation {
    const g = this.issuer.grant;
    return {
      threadId: String(g.thread_id),
      runId: String(g.run_id),
      triggerClaim: "message_id",
      triggerId: String(g.message_id),
    };
  }

  async verdict(
    token: Secret | string,
    expect?: Expectation,
    actual: Actual = ACTUAL,
  ): Promise<TokenVerdict> {
    const secret = token instanceof Secret ? token : new Secret(token);
    const check = await this.verifier.check(
      secret,
      expect ?? this.full(),
      actual,
    );
    return check.verdict;
  }
}

/** The header cases of TestParseInvocationToken_HeaderRules and RejectsAlgAndTyp. */
function badHeaders(kid: string): Record<string, string> {
  const h: Record<string, string> = {
    "typ missing": header(kid, "EdDSA", null),
    "kid empty": header("", "EdDSA", TYP),
    "kid missing": JSON.stringify({ alg: "EdDSA", typ: TYP }),
    crit: JSON.stringify({ alg: "EdDSA", kid, typ: TYP, crit: ["exp"] }),
    "alg missing": header(kid, null),
    "kid number": JSON.stringify({ alg: "EdDSA", kid: 1, typ: TYP }),
    "duplicate alg": `{"alg":"none","alg":"EdDSA","kid":"${kid}","typ":"${TYP}"}`,
    "duplicate typ": `{"alg":"EdDSA","kid":"${kid}","typ":"JWT","typ":"${TYP}"}`,
    "header not an object": "[1]",
    "header trailing data": `${header(kid)}{}`,
    "header with a byte order mark": `\ufeff${header(kid)}`,
  };
  for (const alg of ["none", "HS256", "ES256", "RS256", "Ed25519", "eddsa"])
    h[`alg ${alg}`] = header(kid, alg);
  for (const typ of [
    "JWT",
    "at+jwt",
    "terminal-session+jwt",
    "application/agent-invocation+jwt",
    "Agent-Invocation+JWT",
    "agent-invocation+jwt ",
  ])
    h[`typ ${typ}`] = header(kid, "EdDSA", typ);
  return h;
}

/** The INVALID cases of the reference tests: tokens and, where it differs, the expectation. */
function invalidCases(
  env: Env,
  valid: string,
): [Record<string, string>, Record<string, Expectation>] {
  const { issuer } = env;
  const key = issuer.key();
  const [head, body, sig] = valid.split(".") as [string, string, string];
  const payload = unb64(body);

  // Points 1 and 2 and the token form: every variant is INVALID.
  const invalid: Record<string, string> = {};
  for (const [name, h] of Object.entries(badHeaders(issuer.active)))
    invalid[name] = signEd(h, payload, key);
  invalid["HS256 with the public key"] = signHs256WithPublicKey(
    header(issuer.active, "HS256"),
    payload,
    key,
  );
  invalid["none with an empty signature"] =
    `${b64(header(issuer.active, "none"))}.${body}.`;
  invalid["padded signature"] = `${head}.${body}.${sig}=`;
  invalid["padded payload"] = `${head}.${body}=.${sig}`;
  invalid["character outside base64url"] = `${head}.${body}.${sig}+`;
  invalid["two segments"] = `${head}.${body}`;
  invalid["four segments"] = `${valid}.${sig}`;
  invalid["empty header segment"] = `.${body}.${sig}`;
  invalid["signature of another key"] = signEd(
    header(issuer.active),
    payload,
    generateKeyPairSync("ed25519").privateKey,
  );
  // Payload shape (decodeClaims of the reference).
  const shapes: Record<string, (m: Json) => void> = {
    "aud array": (m) => {
      m.aud = [m.aud];
    },
    "iss missing": (m) => {
      delete m.iss;
    },
    "sub empty": (m) => {
      m.sub = "";
    },
    "sub not a UUID": (m) => {
      m.sub = "alex";
    },
    "permission_version missing": (m) => {
      delete m.permission_version;
    },
    "iat string": (m) => {
      m.iat = "1";
    },
    "exp fraction": (m) => {
      m.exp = 1.5;
    },
    "iat boolean": (m) => {
      m.iat = true;
    },
    "unknown tool_class": (m) => {
      m.tool_class = "root";
    },
    // Names every JavaScript object inherits are no tool classes either.
    "tool_class constructor": (m) => {
      m.tool_class = "constructor";
    },
    "tool_class __proto__": (m) => {
      m.tool_class = "__proto__";
    },
    "run_id null": (m) => {
      m.run_id = null;
    },
    "message_id number": (m) => {
      m.message_id = 5;
    },
  };
  for (const [name, change] of Object.entries(shapes))
    invalid[name] = issuer.resign(valid, change);
  const text = new TextDecoder().decode(payload);
  // A whole number written as a fraction or with an exponent is no integer for the reference.
  invalid["iat written as a fraction"] = signEd(
    header(issuer.active),
    enc.encode(text.replace(/"iat":(\d+)/, '"iat":$1.0')),
    key,
  );
  invalid["exp written with an exponent"] = signEd(
    header(issuer.active),
    enc.encode(text.replace(/"exp":(\d+)/, '"exp":$1e0')),
    key,
  );
  invalid["iat beyond int64"] = signEd(
    header(issuer.active),
    enc.encode(text.replace(/"iat":(\d+)/, `"iat":${(2n ** 63n).toString()}`)),
    key,
  );
  invalid["duplicate payload member"] = signEd(
    header(issuer.active),
    enc.encode(`${text.slice(0, -1)},"aud":"${uuidv7()}"}`),
    key,
  );
  invalid["payload trailing data"] = signEd(
    header(issuer.active),
    enc.encode(`${text}{}`),
    key,
  );
  invalid["payload not UTF-8"] = signEd(
    header(issuer.active),
    new Uint8Array([...payload.slice(0, -1), 0xff, 0x7d]),
    key,
  );

  // Points 4 and 6, and an iat in the future (point 5).
  const full = env.full();
  const expectFor: Record<string, Expectation> = {};
  invalid["foreign iss"] = issuer.resign(valid, (m) => {
    m.iss = uuidv7();
  });
  invalid["foreign aud"] = issuer.resign(valid, (m) => {
    m.aud = uuidv7();
  });
  invalid["upper-case aud"] = issuer.resign(valid, (m) => {
    m.aud = String(m.aud).toUpperCase();
  });
  const changed = text.replace('"level":"trigger"', '"level":"triggex"');
  invalid["changed payload byte"] = `${head}.${b64(changed)}.${sig}`;
  for (const [name, member] of [
    ["run_id", "runId"],
    ["thread_id", "threadId"],
    ["triggering object", "triggerId"],
  ] as const) {
    invalid[name] = issuer.raw();
    expectFor[name] = { ...full, [member]: uuidv7() };
  }
  invalid["iat in the future"] = issuer.resign(valid, (m) => {
    const iat = Number(m.iat);
    m.iat = iat + LEEWAY + 1;
    m.exp = iat + LEEWAY + 1 + TTL;
  });
  invalid["exp not after iat"] = issuer.resign(valid, (m) => {
    m.exp = m.iat;
  });
  return [invalid, expectFor];
}

describe("InvocationVerifier", () => {
  it("agrees with the reference cases (09 4.5 points 1 to 10)", async () => {
    const env = new Env();
    const { issuer } = env;
    const valid = issuer.raw();
    const [invalid, expectFor] = invalidCases(env, valid);
    const got: Record<string, string> = {};
    for (const [name, tok] of Object.entries(invalid))
      got[name] = await env.verdict(tok, expectFor[name]);
    expect(got).toEqual(
      Object.fromEntries(Object.keys(invalid).map((n) => [n, "invalid"])),
    );

    // The jti store is unchanged by the rejections: the valid token passes once, then REPLAY.
    expect(await env.verdict(valid)).toBe(TokenVerdict.VALID);
    expect(await env.verdict(valid)).toBe(TokenVerdict.REPLAY);

    // Point 8: a differing and an unknown actual mode never match.
    for (const mode of ["personal", "unknown", ""])
      expect(
        await env.verdict(issuer.token(), undefined, {
          billingMode: mode,
          toolClass: "scoped",
        }),
        mode,
      ).toBe(TokenVerdict.BILLING_MODE_MISMATCH);
    // Point 10: host above scoped is a mismatch, scoped above chat too, chat under scoped passes.
    expect(
      await env.verdict(issuer.token(), undefined, {
        billingMode: "organization",
        toolClass: "host",
      }),
    ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);
    expect(
      await env.verdict(issuer.token({ tool_class: "chat" }), undefined, {
        billingMode: "organization",
        toolClass: "scoped",
      }),
    ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);
    expect(
      await env.verdict(issuer.token(), undefined, {
        billingMode: "organization",
        toolClass: "chat",
      }),
    ).toBe(TokenVerdict.VALID);
    // An unknown actual tool class never passes.
    expect(
      await env.verdict(issuer.token(), undefined, {
        billingMode: "organization",
        toolClass: "unknown",
      }),
    ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);

    // Point 5 after 8: expired with a billing mismatch gives the mismatch; expired alone is
    // EXPIRED and carries the claims; the last second before exp plus the leeway passes.
    const old = issuer.token();
    await env.clock.advance((TTL + LEEWAY) * 1000);
    expect(
      await env.verdict(old, undefined, {
        billingMode: "personal",
        toolClass: "scoped",
      }),
    ).toBe(TokenVerdict.BILLING_MODE_MISMATCH);
    const expired = await env.verifier.check(old, env.full(), ACTUAL);
    expect(expired.verdict).toBe(TokenVerdict.EXPIRED);
    expect(expired.claims?.jti).toBe(payloadOf(old.reveal()).jti);
    const env2 = new Env();
    const last = env2.issuer.token();
    await env2.clock.advance((TTL + LEEWAY - 1) * 1000);
    expect(await env2.verdict(last)).toBe(TokenVerdict.VALID);

    // Point 7: the same valid token twice.
    const tok = issuer.token();
    const first = await env.verifier.check(tok, env.full(), ACTUAL);
    expect(first.verdict).toBe(TokenVerdict.VALID);
    expect(first.claims?.runId).toBe(issuer.grant.run_id);
    expect(first.claims?.permissionVersion).toBe(4);
    expect((first.claims?.exp ?? 0) - (first.claims?.iat ?? 0)).toBe(TTL);
    expect(await env.verdict(tok)).toBe(TokenVerdict.REPLAY);

    // A token without run passes against an expectation without run (autocomplete).
    const commandId = uuidv7();
    const auto = issuer.token({
      trigger_kind: "autocomplete",
      run_id: null,
      message_id: null,
      command_id: commandId,
    });
    expect(
      await env.verdict(auto, {
        threadId: String(issuer.grant.thread_id),
        runId: undefined,
        triggerClaim: "command_id",
        triggerId: commandId,
      }),
    ).toBe(TokenVerdict.VALID);
  });

  it("refuses alg none, HS256 with the public key and ES256 before any key lookup", async () => {
    const env = new Env();
    const { issuer } = env;
    const payload = enc.encode(JSON.stringify(issuer.claims()));
    const key = issuer.key();
    const cases = {
      none: `${b64(header(issuer.active, "none"))}.${b64(payload)}.`,
      "none with a signature": signEd(
        header(issuer.active, "none"),
        payload,
        key,
      ),
      HS256: signHs256WithPublicKey(
        header(issuer.active, "HS256"),
        payload,
        key,
      ),
      ES256: signEd(header(issuer.active, "ES256"), payload, key),
      "typ at+jwt": signEd(
        header(issuer.active, "EdDSA", "at+jwt"),
        payload,
        key,
      ),
    };
    for (const [name, tok] of Object.entries(cases)) {
      const check = await env.verifier.check(
        new Secret(tok),
        env.full(),
        ACTUAL,
      );
      expect([name, check.verdict, check.claims]).toEqual([
        name,
        TokenVerdict.INVALID,
        undefined,
      ]);
    }
    expect([env.verifier.fetches, env.jwks.calls]).toEqual([0, 0]);
  });

  it("calls a function for the actual state only after the allow list, with the claims", async () => {
    const env = new Env();
    const seen: string[] = [];
    const resolve = (claims: { jti: string }): Promise<Actual> => {
      seen.push(claims.jti);
      return Promise.resolve(ACTUAL);
    };
    const tok = env.issuer.token();
    const check = await env.verifier.check(tok, env.full(), resolve);
    expect(check.verdict).toBe(TokenVerdict.VALID);
    expect(seen).toEqual([payloadOf(tok.reveal()).jti]);
    // A token refused at point 6 never asks for the actual state.
    const other = await env.verifier.check(
      env.issuer.token(),
      { ...env.full(), threadId: uuidv7() },
      resolve,
    );
    expect(other.verdict).toBe(TokenVerdict.INVALID);
    expect(seen).toHaveLength(1);
  });

  it("remembers a jti until exp plus the leeway", async () => {
    const env = new Env();
    const tok = env.issuer.token();
    expect(await env.verdict(tok)).toBe(TokenVerdict.VALID);
    // The jti is remembered until exp plus the leeway; afterwards the token is expired anyway.
    await env.clock.advance((TTL + LEEWAY - 1) * 1000);
    expect(await env.verdict(tok)).toBe(TokenVerdict.REPLAY);
    await env.clock.advance(1000);
    expect(await env.verdict(tok)).toBe(TokenVerdict.EXPIRED);
  });

  it("caches the JWKS for max-age and reloads once per adapter.jwks_refresh_min_interval", async () => {
    const env = new Env();
    const v = env.verifier;
    expect(await env.verdict(env.issuer.token())).toBe(TokenVerdict.VALID);
    expect([v.fetches, v.forcedReloads]).toEqual([1, 0]);
    await env.clock.advance((MAX_AGE - 1) * 1000);
    expect(await env.verdict(env.issuer.token())).toBe(TokenVerdict.VALID);
    expect(v.fetches).toBe(1);
    await env.clock.advance(1000);
    expect(await env.verdict(env.issuer.token())).toBe(TokenVerdict.VALID);
    expect([v.fetches, v.forcedReloads]).toEqual([2, 0]);

    // An unknown kid: exactly one forced reload, then UNKNOWN_KEY without claims.
    const stranger = generateKeyPairSync("ed25519").privateKey;
    const payload = enc.encode(JSON.stringify(env.issuer.claims()));
    const unknown = signEd(header("unknown-1"), payload, stranger);
    const check = await v.check(new Secret(unknown), env.full(), ACTUAL);
    expect([check.verdict, check.claims]).toEqual([
      TokenVerdict.UNKNOWN_KEY,
      undefined,
    ]);
    expect([v.fetches, v.forcedReloads]).toEqual([3, 1]);
    // A second unknown kid within adapter.jwks_refresh_min_interval fetches nothing.
    await env.clock.advance((REFRESH - 1) * 1000);
    const other = signEd(header("unknown-2"), payload, stranger);
    expect(await env.verdict(other)).toBe(TokenVerdict.UNKNOWN_KEY);
    expect([v.fetches, v.forcedReloads]).toEqual([3, 1]);
    // After the interval a kid published meanwhile is found by one forced reload.
    await env.clock.advance(1000);
    env.issuer.addKey();
    expect(await env.verdict(env.issuer.token())).toBe(TokenVerdict.VALID);
    expect([v.fetches, v.forcedReloads]).toEqual([4, 2]);

    // refresh() fetches now and counts as a fetch, not as a forced reload.
    await v.refresh();
    expect([v.fetches, v.forcedReloads]).toEqual([5, 2]);
  });

  it("never takes a key from the token: a jwk header with an unknown kid stays unknown", async () => {
    const env = new Env();
    const stranger = generateKeyPairSync("ed25519");
    const jwk = stranger.publicKey.export({ format: "jwk" });
    const head = JSON.stringify({
      alg: "EdDSA",
      kid: "stranger",
      typ: TYP,
      jwk,
    });
    const tok = signEd(
      head,
      enc.encode(JSON.stringify(env.issuer.claims())),
      stranger.privateKey,
    );
    expect(await env.verdict(tok)).toBe(TokenVerdict.UNKNOWN_KEY);
    // The key the token named never joined the set.
    await env.clock.advance(REFRESH * 1000);
    expect(await env.verdict(tok)).toBe(TokenVerdict.UNKNOWN_KEY);
  });

  it("fetches a JWKS without max-age again on the next use", async () => {
    const env = new Env({ maxAgeMs: undefined });
    expect(await env.verdict(env.issuer.token())).toBe(TokenVerdict.VALID);
    expect(await env.verdict(env.issuer.token())).toBe(TokenVerdict.VALID);
    expect([env.verifier.fetches, env.verifier.forcedReloads]).toEqual([2, 0]);
  });

  it("skips unusable keys of the JWKS", async () => {
    const env = new Env();
    const { issuer } = env;
    const hidden = issuer.addKey({ active: false, published: false });
    const x = b64(publicBytes(issuer.key(hidden)));
    const base: Json = { kty: "OKP", crv: "Ed25519", kid: hidden, x };
    env.jwks.extra = [
      { ...base, kty: "EC" },
      { ...base, crv: "Ed448" },
      { ...base, alg: "ES256" },
      { ...base, use: "enc" },
      { ...base, x: `${x}AA` },
      { ...base, x: `${x.slice(0, -1)}=` },
    ];
    expect(await env.verdict(issuer.raw(undefined, hidden))).toBe(
      TokenVerdict.UNKNOWN_KEY,
    );
    // The same key with alg and use left out is usable.
    env.jwks.extra = [base];
    await env.clock.advance(REFRESH * 1000);
    expect(await env.verdict(issuer.raw(undefined, hidden))).toBe(
      TokenVerdict.VALID,
    );
  });

  it("propagates a failing JWKS fetch and a body that is not a key set", async () => {
    const env = new Env();
    const failing = new InvocationVerifier({
      instanceId: env.issuer.instanceId,
      agentId: env.issuer.agentId,
      jwks: () => Promise.reject(new Error("jwks unavailable")),
      replay: env.store,
      clock: env.clock,
    });
    await expect(
      failing.check(env.issuer.token(), env.full(), ACTUAL),
    ).rejects.toThrow("jwks unavailable");
    const notASet = new InvocationVerifier({
      instanceId: env.issuer.instanceId,
      agentId: env.issuer.agentId,
      jwks: () => Promise.resolve([{ keys: "none" }, undefined] as const),
      replay: env.store,
      clock: env.clock,
    });
    await expect(notASet.refresh()).rejects.toThrow(/key set/);
    expect(notASet.fetches).toBe(1);
  });

  it("reloads the allow list for a higher permission_version and refuses a missing sub", async () => {
    const [alex, sam, robin] = [uuidv7(), uuidv7(), uuidv7()];
    const loader = {
      ids: [alex],
      version: 4,
      calls: 0,
      load: (): Promise<S["AgentTriggerers"]> => {
        loader.calls++;
        return Promise.resolve({
          principal_ids: [...loader.ids],
          permission_version: loader.version,
        });
      },
    };
    const env = new Env({ allowlist: loader.load });
    const { issuer } = env;

    // Loaded on the first check; a listed sub passes.
    expect(await env.verdict(issuer.token({ sub: alex }))).toBe(
      TokenVerdict.VALID,
    );
    expect(loader.calls).toBe(1);
    // The same permission_version: no reload; an unlisted sub is INVALID.
    expect(await env.verdict(issuer.token({ sub: sam }))).toBe(
      TokenVerdict.INVALID,
    );
    expect(loader.calls).toBe(1);
    // A higher permission_version reloads first; the new list admits sam.
    loader.ids = [alex, sam];
    loader.version = 5;
    expect(
      await env.verdict(issuer.token({ sub: sam, permission_version: 5 })),
    ).toBe(TokenVerdict.VALID);
    expect(loader.calls).toBe(2);
    // Still missing after the reload: INVALID.
    loader.version = 6;
    expect(
      await env.verdict(issuer.token({ sub: robin, permission_version: 6 })),
    ).toBe(TokenVerdict.INVALID);
    expect(loader.calls).toBe(3);
    // accessChanged() reloads before the next check, even without a higher version.
    loader.ids = [alex];
    env.verifier.accessChanged();
    expect(
      await env.verdict(issuer.token({ sub: sam, permission_version: 6 })),
    ).toBe(TokenVerdict.INVALID);
    expect(loader.calls).toBe(4);
    // The allow list comes after point 6 and before point 8: a refused sub with a billing
    // mismatch is INVALID, a token for another thread never loads the list.
    expect(
      await env.verdict(issuer.token({ sub: robin }), undefined, {
        billingMode: "personal",
        toolClass: "scoped",
      }),
    ).toBe(TokenVerdict.INVALID);
    env.verifier.accessChanged();
    const wrong = { ...env.full(), threadId: uuidv7() };
    expect(await env.verdict(issuer.token({ sub: alex }), wrong)).toBe(
      TokenVerdict.INVALID,
    );
    expect(loader.calls).toBe(4);
    // An upper-case sub names the same principal as its canonical form.
    expect(await env.verdict(issuer.token({ sub: alex.toUpperCase() }))).toBe(
      TokenVerdict.VALID,
    );
  });

  it("refuses a command trigger without a counterpart in the payload", async () => {
    const env = new Env();
    const { issuer } = env;
    const commandInvocation = uuidv7();
    const event = {
      type: "agent.invocation",
      payload: {
        run_id: issuer.grant.run_id,
        thread_id: issuer.grant.thread_id,
        channel_id: issuer.grant.channel_id,
        trigger_kind: "command",
        triggered_by: principal(),
        billing_mode: "organization",
        command_invocation_id: commandInvocation,
      },
    };
    const expectation = expectationFor("agent.invocation", event);
    expect([expectation.triggerClaim, expectation.triggerId]).toEqual([
      "command_id",
      undefined,
    ]);
    // Even a token whose command_id equals the payload's only command-like ID is refused.
    const tok = issuer.token({
      trigger_kind: "command",
      message_id: null,
      command_id: commandInvocation,
    });
    const check = await env.verifier.check(tok, expectation, ACTUAL);
    expect(check.verdict).toBe(TokenVerdict.INVALID);
    // Nothing was remembered: the same token against a complete expectation still passes.
    const full: Expectation = { ...expectation, triggerId: commandInvocation };
    expect(await env.verdict(tok, full)).toBe(TokenVerdict.VALID);
  });

  it("never logs the token or the jti, only a short hash prefix of the jti", async () => {
    const env = new Env();
    const tok = env.issuer.token();
    const jti = String(payloadOf(tok.reveal()).jti);
    await env.verifier.check(tok, env.full(), ACTUAL);
    await env.verifier.check(tok, env.full(), ACTUAL); // replay, logged
    await env.verifier.check(tok, { ...env.full(), runId: uuidv7() }, ACTUAL); // refused at point 6, logged
    const check = await env.verifier.check(tok, env.full(), ACTUAL);
    const logs = env.logger.lines.join("\n");
    const text = [
      logs,
      JSON.stringify(check),
      JSON.stringify(env.verifier),
    ].join("\n");
    expect(env.logger.lines.length).toBeGreaterThan(0);
    for (const part of tok.reveal().split("."))
      expect(text).not.toContain(part);
    // The claims carry the jti for the adapter; a log line only a hash prefix of it.
    expect(logs).not.toContain(jti);
    const prefix = createHash("sha256").update(jti).digest("hex").slice(0, 12);
    expect(logs).toContain(prefix);
  });

  it("checks one token at a time", async () => {
    const env = new Env();
    const tok = env.issuer.token();
    // Two concurrent checks of the same token: exactly one passes, the other is a replay.
    const verdicts = await Promise.all([
      env.verdict(tok),
      env.verdict(tok),
      env.verdict(tok),
    ]);
    expect(verdicts.sort()).toEqual([
      TokenVerdict.REPLAY,
      TokenVerdict.REPLAY,
      TokenVerdict.VALID,
    ]);
    expect(env.verifier.fetches).toBe(1);
  });
});

describe("compareActual", () => {
  it("compares points 8 and 10 only", () => {
    const env = new Env();
    const claims = { billingMode: "organization", toolClass: "scoped" };
    expect(compareActual(claims, ACTUAL)).toBe(TokenVerdict.VALID);
    expect(
      compareActual(claims, { billingMode: "unknown", toolClass: "chat" }),
    ).toBe(TokenVerdict.BILLING_MODE_MISMATCH);
    expect(
      compareActual(claims, { billingMode: "organization", toolClass: "host" }),
    ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);
    expect(
      compareActual(
        { billingMode: "organization", toolClass: "root" },
        { billingMode: "organization", toolClass: "chat" },
      ),
    ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);
    // Inherited object members are unknown tool classes on both sides.
    for (const name of ["constructor", "toString", "__proto__"]) {
      expect(
        compareActual(
          { billingMode: "organization", toolClass: "chat" },
          { billingMode: "organization", toolClass: name },
        ),
        name,
      ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);
      expect(
        compareActual(
          { billingMode: "organization", toolClass: name },
          { billingMode: "organization", toolClass: "host" },
        ),
        name,
      ).toBe(TokenVerdict.TOOL_CLASS_MISMATCH);
    }
    expect(env.verifier.fetches).toBe(0);
  });
});

function principal(): Json {
  return { id: uuidv7(), kind: "user", handle: "alex", display_name: "Alex" };
}

describe("expectationFor", () => {
  it("returns the row of each event type", () => {
    const [run, thread, channel, obj] = [
      uuidv7(),
      uuidv7(),
      uuidv7(),
      uuidv7(),
    ];
    const base = {
      run_id: run,
      thread_id: thread,
      channel_id: channel,
      triggered_by: principal(),
      billing_mode: "organization",
    };
    const rows: Record<string, [Json, string, string | undefined]> = {
      message: [
        { trigger_kind: "message", message_id: obj },
        "message_id",
        obj,
      ],
      interaction: [
        { trigger_kind: "interaction", interaction_id: obj },
        "interaction_id",
        obj,
      ],
      "message without id": [
        { trigger_kind: "message" },
        "message_id",
        undefined,
      ],
    };
    for (const [name, [extra, claim, trigger]] of Object.entries(rows)) {
      const got = expectationFor("agent.invocation", {
        payload: { ...base, ...extra },
      });
      expect([name, got]).toEqual([
        name,
        {
          threadId: thread,
          runId: run,
          triggerClaim: claim,
          triggerId: trigger,
        },
      ]);
    }
    // A trigger kind this SDK does not know has no counterpart: fail closed.
    const steer = expectationFor("agent.invocation", {
      payload: { ...base, trigger_kind: "steer" },
    });
    expect([steer.threadId, steer.runId, steer.triggerId]).toEqual([
      thread,
      run,
      undefined,
    ]);
    // A payload ID in another spelling is compared in its canonical form.
    const upper = expectationFor("agent.followup", {
      payload: {
        run_id: run.toUpperCase(),
        thread_id: thread,
        message_id: obj,
      },
    });
    expect(upper.runId).toBe(run);

    const followup = {
      payload: { run_id: run, thread_id: thread, message_id: obj },
    };
    expect(expectationFor("agent.followup", followup)).toEqual({
      threadId: thread,
      runId: run,
      triggerClaim: "message_id",
      triggerId: obj,
    });

    const interaction = {
      id: obj,
      component_id: uuidv7(),
      message_id: uuidv7(),
      principal_id: uuidv7(),
      client_interaction_id: uuidv7(),
      values: {},
      state: "dispatched",
      created_at: "2026-10-20T09:00:00.000Z",
      run_id: run,
      thread_id: thread,
    };
    const created = { payload: { interaction } };
    expect(expectationFor("interaction.created", created)).toEqual({
      threadId: thread,
      runId: run,
      triggerClaim: "interaction_id",
      triggerId: obj,
    });

    const command = {
      invocation_id: uuidv7(),
      command_id: obj,
      command_name: "status",
      control: false,
      channel_id: channel,
      invoked_by: principal(),
      options: [],
    };
    expect(expectationFor("command.invoked", { payload: command })).toEqual({
      threadId: undefined,
      runId: undefined,
      triggerClaim: "command_id",
      triggerId: obj,
    });
    expect(
      expectationFor("command.invoked", {
        payload: { ...command, run_id: run, thread_id: thread },
      }),
    ).toEqual({
      threadId: thread,
      runId: run,
      triggerClaim: "command_id",
      triggerId: obj,
    });

    const auto = {
      request_id: uuidv7(),
      command_id: obj,
      channel_id: channel,
      user_id: uuidv7(),
      option: "name",
      value: "",
      expires_at: "2026-10-20T09:00:03.000Z",
      thread_id: thread,
    };
    expect(
      expectationFor("command.autocomplete_requested", { payload: auto }),
    ).toEqual({
      threadId: thread,
      runId: undefined,
      triggerClaim: "command_id",
      triggerId: obj,
    });

    expect([...TOKEN_EVENT_TYPES].sort()).toEqual([
      "agent.followup",
      "agent.invocation",
      "command.autocomplete_requested",
      "command.invoked",
      "interaction.created",
    ]);
    expect(() => expectationFor("run.cancel_requested", followup)).toThrow(
      /run\.cancel_requested/,
    );
    expect(() => expectationFor("agent.followup", created)).toThrow(TypeError);
    expect(() =>
      expectationFor("agent.followup", {
        payload: { run_id: "run", thread_id: thread, message_id: obj },
      }),
    ).toThrow(TypeError);
    expect(() => expectationFor("agent.followup", {})).toThrow(TypeError);
  });
});

describe("storedClaims", () => {
  it("reads the claims of a verified token without checking it again", () => {
    const env = new Env();
    const tok = env.issuer.token();
    const claims = storedClaims(tok);
    expect(claims.jti).toBe(payloadOf(tok.reveal()).jti);
    expect(claims.runId).toBe(env.issuer.grant.run_id);
    expect(claims.triggerKind).toBe("message");
    expect(() => storedClaims(new Secret("a.b"))).toThrow(RangeError);
    expect(() => storedClaims(new Secret("a.!.c"))).toThrow(RangeError);
    const shapeless = env.issuer.resign(tok.reveal(), (m) => {
      delete m.iss;
    });
    expect(() => storedClaims(new Secret(shapeless))).toThrow(RangeError);
  });
});

describe("jwksFetcher and triggererLoader", () => {
  it("fetch getJwks and getAgentTriggerers through the client with the max-age", async () => {
    const env = new Env();
    const seen: string[] = [];
    let cacheControl: string | undefined = "public, max-age=120";
    const client = createApiClient({
      baseUrl: "http://127.0.0.1:9/api/v1",
      fetch: (request: Request) => {
        const path = new URL(request.url).pathname;
        seen.push(`${request.method} ${path}`);
        if (path.endsWith("/instance/jwks"))
          return Promise.resolve(
            Response.json(env.issuer.jwks(), {
              headers:
                cacheControl === undefined
                  ? {}
                  : { "Cache-Control": cacheControl },
            }),
          );
        if (path.endsWith("/agents/me/triggerers"))
          return Promise.resolve(
            Response.json({ principal_ids: [uuidv7()], permission_version: 3 }),
          );
        return Promise.resolve(
          Response.json(
            {
              type: "about:blank",
              title: "Not found",
              status: 404,
              code: "not_found",
            },
            {
              status: 404,
              headers: { "Content-Type": "application/problem+json" },
            },
          ),
        );
      },
    });
    const [body, maxAgeMs] = await jwksFetcher(client)();
    expect(body).toEqual(env.issuer.jwks());
    expect(maxAgeMs).toBe(120_000);
    for (const [header, want] of [
      ['max-age="30", public', 30_000],
      ["no-store", undefined],
      ["max-age=soon", undefined],
      [undefined, undefined],
    ] as const) {
      cacheControl = header;
      expect((await jwksFetcher(client)())[1]).toBe(want);
    }
    const loaded = await triggererLoader(client)();
    expect(loaded.permission_version).toBe(3);
    expect(seen).toEqual([
      "GET /api/v1/instance/jwks",
      "GET /api/v1/instance/jwks",
      "GET /api/v1/instance/jwks",
      "GET /api/v1/instance/jwks",
      "GET /api/v1/instance/jwks",
      "GET /api/v1/agents/me/triggerers",
    ]);
  });

  it("verify a token the FakeServer delivered and refuse its forgeries", async () => {
    const clock = new FakeClock();
    const server = new FakeServer({ clock, dpop: false });
    await server.start();
    try {
      const client = createApiClient({
        baseUrl: server.baseUrl,
        token: server.credentials.apiToken,
        clock,
      });
      const verifier = new InvocationVerifier({
        instanceId: server.instanceId,
        agentId: server.agentId,
        jwks: jwksFetcher(client),
        replay: new MemoryInboxStore(),
        settings: server.sdkSettings,
        clock,
        allowlist: triggererLoader(client),
      });
      // The first poll creates the consumer at the head; the trigger comes after it.
      const poll = () =>
        client.GET("/agents/me/consumers/{consumer}/events", {
          params: { path: { consumer: "default" } },
        });
      await poll();
      const triggered = await server.trigger();
      const { data } = await client.GET(
        "/agents/me/consumers/{consumer}/events",
        { params: { path: { consumer: "default" } } },
      );
      const event = data?.events.find((e) => e.type === "agent.invocation");
      expect(event?.invocation_token).toBeTypeOf("string");
      const token = new Secret(String(event?.invocation_token));
      const expectation = expectationFor("agent.invocation", event);
      expect(expectation.runId).toBe(triggered.runId);
      const check = await verifier.check(token, expectation, ACTUAL);
      expect(check.verdict).toBe(TokenVerdict.VALID);
      expect(check.claims?.runId).toBe(triggered.runId);
      for (const variant of [
        "bad_signature",
        "alg_hs256",
        "aud_other_agent",
        "typ_access",
      ] as const) {
        const forged = await server.forgeToken(triggered.runId, variant);
        const refused = await verifier.check(forged, expectation, ACTUAL);
        expect([variant, refused.verdict]).toEqual([
          variant,
          TokenVerdict.INVALID,
        ]);
      }
      expect(verifier.fetches).toBe(1);
    } finally {
      await server.stop();
    }
  });
});
