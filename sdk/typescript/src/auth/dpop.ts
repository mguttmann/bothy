// SPDX-License-Identifier: Apache-2.0
/**
 * DPoP keys and proofs (RFC 9449; docs/spec/06-api.md 3.1, 08 7.3) and the clear-text rule.
 *
 * A token bound to a key at `createToken` (`dpop_jkt`) travels as `Authorization: DPoP <token>`
 * with a fresh proof per request attempt: header `typ = dpop+jwt`, `alg` `ES256` (signature as
 * 64-byte `R || S`) or `EdDSA`, the public `jwk`; claims `jti`, `htm`, `htu` (without query and
 * fragment), `iat`, `ath` (base64url SHA-256 of the token) and the newest server `nonce`
 * (docs/DECISIONS.md D-M2-29-3, D-M2-30-3). Only `node:crypto` signs, no JOSE library
 * (D-M2-30-2). A private key never renders: `util.inspect`, `String` and `JSON.stringify` show
 * the thumbprint only.
 *
 * @example
 * ```ts
 * const key = await DpopKey.generate({ path: "dpop.pem" }); // mode 0600, never overwritten
 * key.jkt; // the `dpop_jkt` for createToken
 * const proof = await createDpopProof(key, { htm: "GET", htu: "https://example.com/api/v1/me" });
 * ```
 *
 * @module
 */

import {
  type KeyObject,
  createHash,
  createPrivateKey,
  createPublicKey,
  generateKeyPairSync,
  sign as cryptoSign,
} from "node:crypto";
import { open, readFile, stat } from "node:fs/promises";
import { isIPv4 } from "node:net";

import { uuidv7 } from "../ids.js";
import { Secret } from "./secret.js";
import type { DpopAlgorithm, DpopSigner } from "./types.js";

const REQUIRED_MEMBERS: Readonly<Record<string, readonly string[]>> =
  Object.freeze({
    EC: ["crv", "kty", "x", "y"],
    OKP: ["crv", "kty", "x"],
    RSA: ["e", "kty", "n"],
  });

function b64url(data: Uint8Array | string): string {
  return Buffer.from(data).toString("base64url");
}

/**
 * Returns the RFC 7638 thumbprint (SHA-256, base64url) of a public JWK: only the required
 * members of the key type enter the hash, in lexicographic order.
 *
 * @param jwk - A public JWK of type `EC`, `OKP` or `RSA`.
 * @returns The thumbprint, the value of `dpop_jkt` at `createToken`.
 * @throws `TypeError` when the key type is not supported or a required member is missing.
 *
 * @example
 * ```ts
 * dpopThumbprint({ kty: "OKP", crv: "Ed25519", x: "AA" });
 * // "3I7TDbW-lKbNSt8372-ez4nbmhZyQrtliLvMotQEsgE"
 * ```
 */
export function dpopThumbprint(jwk: Readonly<Record<string, unknown>>): string {
  const names =
    typeof jwk.kty === "string" ? REQUIRED_MEMBERS[jwk.kty] : undefined;
  if (names === undefined || names.some((n) => typeof jwk[n] !== "string"))
    throw new TypeError("unsupported or incomplete JWK");
  const canonical = `{${names.map((n) => `${JSON.stringify(n)}:${JSON.stringify(jwk[n])}`).join(",")}}`;
  return createHash("sha256").update(canonical, "utf8").digest("base64url");
}

function algorithmOf(key: KeyObject): DpopAlgorithm {
  if (key.asymmetricKeyType === "ed25519") return "EdDSA";
  if (
    key.asymmetricKeyType === "ec" &&
    key.asymmetricKeyDetails?.namedCurve === "prime256v1"
  )
    return "ES256";
  throw new TypeError("a DPoP key must be P-256 or Ed25519");
}

function publicJwkOf(key: KeyObject): Readonly<Record<string, string>> {
  const jwk = createPublicKey(key).export({ format: "jwk" });
  const out: Record<string, string> = {};
  for (const name of REQUIRED_MEMBERS[jwk.kty ?? ""] ?? [])
    out[name] = String(jwk[name as keyof typeof jwk]);
  return Object.freeze(out);
}

async function requirePrivate(path: string): Promise<void> {
  if (process.platform === "win32") return;
  const { mode } = await stat(path);
  if (mode & 0o077)
    throw new Error(
      `${path} must not be accessible by group or others (mode 0600)`,
    );
}

/**
 * A DPoP key held in memory: `ES256` (P-256) or `EdDSA` (Ed25519). Use {@link DpopKey.generate}
 * once and {@link DpopKey.load} afterwards; {@link DpopKey.jkt} is the thumbprint the owner
 * enters at `createToken` (`dpop_jkt`). A key store (for example an HSM) implements
 * {@link DpopSigner} instead.
 */
export class DpopKey implements DpopSigner {
  readonly #key: KeyObject;
  /** The JWS `alg` of the proofs. */
  readonly alg: DpopAlgorithm;
  /** The public JWK without private members. */
  readonly publicJwk: Readonly<Record<string, string>>;
  /** The RFC 7638 thumbprint of the public key. */
  readonly jkt: string;

  /**
   * Wraps a private key; prefer {@link DpopKey.generate}, {@link DpopKey.load} or
   * {@link loadDpopKey}.
   *
   * @param key - A P-256 or Ed25519 private key.
   * @throws `TypeError` for another key type or a public key.
   */
  constructor(key: KeyObject) {
    if (key.type !== "private")
      throw new TypeError("a DPoP key must be a private key");
    this.#key = key;
    this.alg = algorithmOf(key);
    this.publicJwk = publicJwkOf(key);
    this.jkt = dpopThumbprint(this.publicJwk);
  }

  /**
   * Creates a new key; with `path` it is written there as a PKCS#8 PEM of mode `0600`.
   *
   * @param options - `path` of a new file (must not exist; opened with `O_EXCL`) and `alg`
   *   (default `ES256`).
   * @returns The key.
   * @throws The `EEXIST` error of `open` when the file exists.
   */
  static async generate(
    options: {
      path?: string | undefined;
      alg?: DpopAlgorithm | undefined;
    } = {},
  ): Promise<DpopKey> {
    const { privateKey } =
      options.alg === "EdDSA"
        ? generateKeyPairSync("ed25519")
        : generateKeyPairSync("ec", { namedCurve: "P-256" });
    if (options.path !== undefined) {
      const pem = privateKey.export({ format: "pem", type: "pkcs8" });
      const file = await open(options.path, "wx", 0o600);
      try {
        await file.writeFile(pem);
      } finally {
        await file.close();
      }
    }
    return new DpopKey(privateKey);
  }

  /**
   * Loads a key that {@link DpopKey.generate} wrote (or any file {@link loadDpopKey} reads).
   *
   * @param path - The key file.
   * @returns The key.
   * @throws When the file is accessible by group or others, or holds no usable private key.
   */
  static load(path: string): Promise<DpopKey> {
    return loadDpopKey(path);
  }

  /**
   * Returns the JWS signature of `data`: 64-byte `R || S` for `ES256`, raw Ed25519 for `EdDSA`.
   *
   * @param data - The JWS signing input.
   * @returns The signature.
   */
  sign(data: Uint8Array): Promise<Uint8Array> {
    const signature =
      this.alg === "ES256"
        ? cryptoSign("sha256", data, {
            key: this.#key,
            dsaEncoding: "ieee-p1363",
          })
        : cryptoSign(null, data, this.#key);
    return Promise.resolve(new Uint8Array(signature));
  }

  /** Returns `DpopKey(<alg>, jkt=<thumbprint>)`, never the key. */
  toString(): string {
    return `DpopKey(${this.alg}, jkt=${this.jkt})`;
  }

  /** Returns `alg` and `jkt` only, so a key never lands in JSON output. */
  toJSON(): Record<string, string> {
    return { alg: this.alg, jkt: this.jkt };
  }

  /** Returns the same text as {@link DpopKey.toString} for `util.inspect`. */
  [Symbol.for("nodejs.util.inspect.custom")](): string {
    return this.toString();
  }
}

/**
 * Reads a DPoP key from a PKCS#8 PEM file or a private JWK file.
 *
 * @param path - The file; on POSIX it must not be accessible by group or others.
 * @returns The key.
 * @throws When the file mode is too open, the JWK has no private member `d`, or the key is
 *   neither P-256 nor Ed25519.
 */
export async function loadDpopKey(path: string): Promise<DpopKey> {
  await requirePrivate(path);
  const text = (await readFile(path, "utf8")).trim();
  let key: KeyObject;
  if (text.startsWith("{")) {
    const jwk = JSON.parse(text) as Record<string, unknown>;
    if (typeof jwk.d !== "string")
      throw new TypeError("the JWK file holds no private key");
    key = createPrivateKey({ key: jwk as never, format: "jwk" });
  } else {
    key = createPrivateKey(text);
  }
  return new DpopKey(key);
}

/** The inputs of one DPoP proof. */
export interface DpopProofOptions {
  /** The HTTP method (`GET` for the gateway). */
  readonly htm: string;
  /** The target URL; it is normalized by {@link normalizeHtu}. */
  readonly htu: string;
  /** The access token the proof binds through `ath`; absent leaves `ath` out. */
  readonly accessToken?: Secret | string | undefined;
  /** The newest server nonce, if any. */
  readonly nonce?: string | undefined;
  /** The wall time of `iat`, Unix milliseconds (default `Date.now()`). */
  readonly now?: number | undefined;
}

/**
 * Builds one DPoP proof (RFC 9449 4.2); each call has a new `jti`, so build one per attempt.
 *
 * @param signer - The DPoP key or key store.
 * @param options - Method, URL, token, nonce and time.
 * @returns The compact JWS.
 */
export async function createDpopProof(
  signer: DpopSigner,
  options: DpopProofOptions,
): Promise<string> {
  const header = {
    typ: "dpop+jwt",
    alg: signer.alg,
    jwk: { ...signer.publicJwk },
  };
  const claims: Record<string, string | number> = {
    jti: uuidv7(),
    htm: options.htm.toUpperCase(),
    htu: normalizeHtu(options.htu),
    iat: Math.floor((options.now ?? Date.now()) / 1_000),
  };
  if (options.accessToken !== undefined) {
    const token =
      options.accessToken instanceof Secret
        ? options.accessToken.reveal()
        : options.accessToken;
    claims.ath = createHash("sha256")
      .update(token, "ascii")
      .digest("base64url");
  }
  if (options.nonce) claims.nonce = options.nonce;
  const input = `${b64url(JSON.stringify(header))}.${b64url(JSON.stringify(claims))}`;
  const signature = await signer.sign(new TextEncoder().encode(input));
  return `${input}.${b64url(signature)}`;
}

/**
 * Returns the DPoP `htu` of a request URL: scheme and host lower case, the default port
 * removed, no query and no fragment (RFC 9449 4.2 and 4.3).
 *
 * @param url - The request URL.
 * @returns The normalized URL.
 *
 * @example
 * ```ts
 * normalizeHtu("HTTPS://Example.COM:443/api/v1/runs?x=1#f"); // "https://example.com/api/v1/runs"
 * ```
 */
export function normalizeHtu(url: string): string {
  const u = new URL(url);
  return `${u.protocol}//${u.host}${u.pathname || "/"}`;
}

/**
 * Returns the `htu` of a gateway proof: the HTTPS form of the gateway URL
 * (`GatewayIdentify.dpop`, 04 6.3 step 3), also when the server speaks clear text behind a TLS
 * terminator or on loopback; a default port (`80` of `ws`, `443` of `wss`) is dropped, query and
 * fragment are removed (docs/DECISIONS.md D-M2-29-3).
 *
 * @param gatewayUrl - The WebSocket URL of the gateway.
 * @returns The `htu`.
 *
 * @example
 * ```ts
 * gatewayHtu("ws://127.0.0.1:8099/api/v1/gateway"); // "https://127.0.0.1:8099/api/v1/gateway"
 * ```
 */
export function gatewayHtu(gatewayUrl: string): string {
  const u = new URL(gatewayUrl);
  return `https://${u.host}${u.pathname || "/"}`;
}

function isLoopback(hostname: string): boolean {
  const host = hostname.toLowerCase();
  if (host === "localhost") return true;
  if (isIPv4(host)) return host.startsWith("127.");
  return host === "[::1]";
}

/**
 * Refuses a clear-text URL that leaves the host: `http` and `ws` only for a loopback host
 * (`127.0.0.0/8`, `::1`, `localhost`), so a token never travels in clear text off the machine
 * (docs/DECISIONS.md D-M2-29-3).
 *
 * @param url - A base URL or gateway URL.
 * @throws `TypeError` when the URL does not parse, its scheme is not `http`, `https`, `ws` or
 *   `wss`, or a clear-text scheme names another host.
 */
export function requireSafeUrl(url: string): void {
  let u: URL | undefined;
  try {
    u = new URL(url);
  } catch {
    u = undefined;
  }
  if (u !== undefined) {
    if (u.protocol === "https:" || u.protocol === "wss:") return;
    if (
      (u.protocol === "http:" || u.protocol === "ws:") &&
      isLoopback(u.hostname)
    )
      return;
  }
  throw new TypeError("only https and wss are allowed off the loopback host");
}
