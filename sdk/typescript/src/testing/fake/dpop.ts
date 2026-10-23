// SPDX-License-Identifier: Apache-2.0
/**
 * API token and DPoP checks of the FakeServer (private; port of
 * `bothy_sdk/testing/_fake_dpop.py`).
 *
 * The rules follow the server's verifier (docs/spec/06-api.md 3.1,
 * docs/spec/08-identity.md 7.3): one proof, `typ = dpop+jwt`, ES256 with a public `jwk` whose
 * thumbprint is the token's bound `jkt`, `htm`, normalized `htu`, `iat` within
 * `auth.dpop_proof_max_age` and `auth.clock_leeway`, `ath`, a current nonce (a missing or old
 * one answers `401 use_dpop_nonce`) and no reused `jti`. Nonces are windows of
 * `auth.dpop_nonce_ttl`; the current and the previous window pass, and
 * {@link NonceSource.rotate} starts a new window at once.
 *
 * @module
 */

import {
  createHash,
  createHmac,
  createPublicKey,
  randomBytes,
  timingSafeEqual,
  verify,
} from "node:crypto";

import { calculateJwkThumbprint, type JWK } from "jose";

import type { Clock } from "../../clock.js";
import {
  b64url,
  b64urlDecode,
  FakeProblem,
  isPlainObject,
  strictObject,
  type Timings,
} from "./base.js";

/** The challenge of a missing or unusable credential. */
export const WWW_AUTHENTICATE = 'DPoP algs="ES256 EdDSA"';

const PRIVATE_MEMBERS = ["d", "p", "q", "dp", "dq", "qi", "k", "oth"];

/**
 * Returns the DPoP `htu` of a request URL: scheme and host lower case, the default port
 * removed, no query and no fragment (RFC 9449 4.2 and 4.3).
 *
 * @param url - The request URL.
 * @returns The normalized URL.
 * @throws `TypeError` when `url` is not a URL.
 */
export function normalizeHtu(url: string): string {
  const parts = new URL(url);
  return `${parts.protocol}//${parts.host}${parts.pathname || "/"}`;
}

/** Stateless windowed nonces: `b64url(window || HMAC16)`. */
export class NonceSource {
  private readonly key = randomBytes(32);
  private bump = 0;

  /**
   * Creates the source.
   *
   * @param clock - The FakeServer's clock.
   * @param ttl - Returns `auth.dpop_nonce_ttl` in milliseconds.
   */
  constructor(
    private readonly clock: Clock,
    private readonly ttl: () => number,
  ) {}

  private window(): number {
    return Math.floor(this.clock.now() / this.ttl()) + this.bump;
  }

  private mac(raw: Uint8Array): Uint8Array {
    return createHmac("sha256", this.key).update(raw).digest().subarray(0, 16);
  }

  private encode(window: number): string {
    const raw = Buffer.alloc(8);
    raw.writeBigUInt64BE(BigInt(window));
    return b64url(Buffer.concat([raw, this.mac(raw)]));
  }

  /**
   * Returns the nonce of the current window.
   *
   * @returns The nonce.
   */
  current(): string {
    return this.encode(this.window());
  }

  /** Starts a new window now, so a client must follow the newest nonce. */
  rotate(): void {
    this.bump += 1;
  }

  /**
   * Tells whether `nonce` belongs to the current or the previous window.
   *
   * @param nonce - The nonce of a proof.
   * @returns Whether it is valid.
   */
  valid(nonce: string): boolean {
    let raw: Uint8Array;
    try {
      raw = b64urlDecode(nonce);
    } catch {
      return false;
    }
    if (raw.length !== 24) return false;
    const head = raw.subarray(0, 8);
    const window = Number(Buffer.from(head).readBigUInt64BE());
    const age = this.window() - window;
    return (
      timingSafeEqual(this.mac(head), raw.subarray(8)) &&
      (age === 0 || age === 1)
    );
  }
}

/** The inputs of one {@link Authenticator.authenticate} call. */
export interface Presented {
  /** The HTTP method (`GET` for the gateway). */
  readonly method: string;
  /** The URL the proof must name. */
  readonly htu: string;
  /** Every `Authorization` value of the request. */
  readonly authorization: readonly string[];
  /** Every `DPoP` value of the request. */
  readonly proofs: readonly string[];
}

/** Checks `Authorization` and `DPoP` of one request or gateway credential. */
export class Authenticator {
  /** The DPoP nonces. */
  readonly nonces: NonceSource;
  private jkt: Promise<string> | undefined;
  private seen = new Map<string, number>();

  /**
   * Creates the checks.
   *
   * @param apiToken - The agent's API token value.
   * @param boundJwk - The public JWK the token is bound to, or `undefined` for a bearer token.
   * @param clock - The FakeServer's clock.
   * @param timings - The FakeServer's timing values.
   */
  constructor(
    private readonly apiToken: string,
    boundJwk: Readonly<Record<string, string>> | undefined,
    private readonly clock: Clock,
    private readonly timings: Timings,
  ) {
    this.nonces = new NonceSource(clock, () => timings["auth.dpop_nonce_ttl"]);
    if (boundJwk !== undefined)
      this.jkt = calculateJwkThumbprint({ ...boundJwk } as JWK);
  }

  private dpopError(code: string): FakeProblem {
    return new FakeProblem(code, {
      headers: {
        "WWW-Authenticate": `DPoP error="${code}", algs="ES256 EdDSA"`,
        "DPoP-Nonce": this.nonces.current(),
      },
    });
  }

  private unauthenticated(): FakeProblem {
    return new FakeProblem("unauthenticated", {
      headers: { "WWW-Authenticate": WWW_AUTHENTICATE },
    });
  }

  /**
   * Accepts the credentials or throws the problem the server answers.
   *
   * @param presented - Method, URL and the credential headers.
   * @throws {@link FakeProblem} `401 unauthenticated`, `invalid_dpop_proof` or `use_dpop_nonce`.
   */
  async authenticate(presented: Presented): Promise<void> {
    if (presented.authorization.length !== 1) throw this.unauthenticated();
    const value = presented.authorization[0] ?? "";
    const space = value.indexOf(" ");
    const scheme = (space < 0 ? value : value.slice(0, space)).toLowerCase();
    const token = space < 0 ? "" : value.slice(space + 1);
    if (
      !["bearer", "dpop"].includes(scheme) ||
      !token ||
      token.includes(" ") ||
      token.includes(",")
    )
      throw this.unauthenticated();
    const a = Buffer.from(token);
    const b = Buffer.from(this.apiToken);
    if (a.length !== b.length || !timingSafeEqual(a, b))
      throw this.unauthenticated();
    if (this.jkt === undefined) {
      if (scheme === "dpop" || presented.proofs.length)
        throw this.unauthenticated();
      return;
    }
    if (scheme === "bearer") throw this.dpopError("invalid_dpop_proof");
    await this.checkProof(presented, token, await this.jkt);
  }

  private async checkProof(
    presented: Presented,
    token: string,
    jkt: string,
  ): Promise<void> {
    if (presented.proofs.length !== 1)
      throw this.dpopError("invalid_dpop_proof");
    const [header, claims] = this.parse(presented.proofs[0] ?? "");
    if (claims.htm !== presented.method)
      throw this.dpopError("invalid_dpop_proof");
    let sameHtu: boolean;
    try {
      sameHtu =
        normalizeHtu(String(claims.htu)) === normalizeHtu(presented.htu);
    } catch {
      sameHtu = false;
    }
    if (!sameHtu) throw this.dpopError("invalid_dpop_proof");
    const now = this.clock.now() / 1000;
    const iat = claims.iat as number;
    if (iat < now - this.timings["auth.dpop_proof_max_age"] / 1000)
      throw this.dpopError("invalid_dpop_proof");
    if (iat > now + this.timings["auth.clock_leeway"] / 1000)
      throw this.dpopError("invalid_dpop_proof");
    if (!this.nonces.valid(claims.nonce as string))
      throw this.dpopError("use_dpop_nonce");
    const ath = b64url(createHash("sha256").update(token, "ascii").digest());
    if (
      claims.ath !== ath ||
      (await calculateJwkThumbprint(header.jwk as JWK)) !== jkt
    )
      throw this.dpopError("invalid_dpop_proof");
    const key = createHash("sha256")
      .update(`${normalizeHtu(presented.htu)}\u0000${String(claims.jti)}`)
      .digest("hex");
    for (const [k, until] of this.seen) if (until <= now) this.seen.delete(k);
    if (this.seen.has(key)) throw this.dpopError("invalid_dpop_proof");
    this.seen.set(
      key,
      now +
        (this.timings["auth.dpop_proof_max_age"] +
          this.timings["auth.clock_leeway"]) /
          1000,
    );
  }

  private parse(
    proof: string,
  ): [Record<string, unknown>, Record<string, unknown>] {
    let header: Record<string, unknown>;
    let claims: Record<string, unknown>;
    try {
      const parts = proof.split(".");
      if (parts.length !== 3) throw new SyntaxError("parts");
      header = strictObject(b64urlDecode(parts[0] ?? ""));
      claims = strictObject(b64urlDecode(parts[1] ?? ""));
      const signature = b64urlDecode(parts[2] ?? "");
      if (
        (typeof header.typ === "string" ? header.typ : "").toLowerCase() !==
          "dpop+jwt" ||
        header.alg !== "ES256"
      )
        throw new SyntaxError("header");
      const jwk = header.jwk;
      if (!isPlainObject(jwk) || jwk.kty !== "EC" || jwk.crv !== "P-256")
        throw new SyntaxError("jwk");
      if (PRIVATE_MEMBERS.some((n) => n in jwk))
        throw new SyntaxError("private jwk");
      const x = b64url(b64urlDecode(String(jwk.x)));
      const y = b64url(b64urlDecode(String(jwk.y)));
      const key = createPublicKey({
        key: { kty: "EC", crv: "P-256", x, y },
        format: "jwk",
      });
      if (signature.length !== 64) throw new SyntaxError("signature size");
      const input = Buffer.from(`${parts[0]}.${parts[1]}`, "ascii");
      if (
        !verify("sha256", input, { key, dsaEncoding: "ieee-p1363" }, signature)
      )
        throw new SyntaxError("signature");
    } catch {
      throw this.dpopError("invalid_dpop_proof");
    }
    if (
      typeof claims.jti !== "string" ||
      !claims.jti ||
      typeof claims.htu !== "string"
    )
      throw this.dpopError("invalid_dpop_proof");
    if (typeof claims.iat !== "number" || !Number.isInteger(claims.iat))
      throw this.dpopError("invalid_dpop_proof");
    if (!("nonce" in claims)) throw this.dpopError("use_dpop_nonce");
    if (typeof claims.nonce !== "string")
      throw this.dpopError("invalid_dpop_proof");
    return [header, claims];
  }
}
