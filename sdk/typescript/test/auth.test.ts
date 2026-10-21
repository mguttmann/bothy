// SPDX-License-Identifier: Apache-2.0
//
// DPoP keys and proofs (docs/spec/06-api.md 3.1, 08 7.3, RFC 9449, RFC 7638) and the clear-text
// rule of D-M2-29-3. Proofs are verified here with node:crypto against their own header `jwk`.

import {
  createHash,
  createPublicKey,
  generateKeyPairSync,
  verify,
} from "node:crypto";
import {
  chmodSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { inspect } from "node:util";
import { afterAll, describe, expect, it } from "vitest";

import {
  DpopKey,
  Secret,
  createDpopProof,
  dpopThumbprint,
  gatewayHtu,
  loadDpopKey,
  normalizeHtu,
  requireSafeUrl,
  type DpopSigner,
} from "../src/auth/index.js";

const CANARY = "canary-token-0123456789";
const NOW = Date.UTC(2026, 0, 1, 12, 0, 0);
const dir = mkdtempSync(join(tmpdir(), "bothy-dpop-"));

afterAll(() => {
  rmSync(dir, { recursive: true, force: true });
});

function b64urlJson(part: string): Record<string, unknown> {
  return JSON.parse(Buffer.from(part, "base64url").toString("utf8")) as Record<
    string,
    unknown
  >;
}

/** Decodes a proof and verifies its signature with the public `jwk` of its header. */
function decode(proof: string): {
  header: Record<string, unknown>;
  claims: Record<string, unknown>;
  signature: Buffer;
} {
  const [h, c, s] = proof.split(".");
  if (h === undefined || c === undefined || s === undefined)
    throw new Error("not a JWS");
  const header = b64urlJson(h);
  const claims = b64urlJson(c);
  const signature = Buffer.from(s, "base64url");
  const key = createPublicKey({
    key: header.jwk as Record<string, string>,
    format: "jwk",
  });
  const data = Buffer.from(`${h}.${c}`, "ascii");
  const ok =
    header.alg === "ES256"
      ? verify("sha256", data, { key, dsaEncoding: "ieee-p1363" }, signature)
      : verify(null, data, key, signature);
  expect(ok).toBe(true);
  return { header, claims, signature };
}

describe("dpopThumbprint", () => {
  it("gives the RFC 7638 section 3.1 example", () => {
    const rfc = {
      kty: "RSA",
      e: "AQAB",
      n:
        "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1" +
        "L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4" +
        "QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbO" +
        "pbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csF" +
        "Cur-kEgU8awapJzKnqDKgw",
      alg: "RS256",
      kid: "2011-04-29",
    };
    expect(dpopThumbprint(rfc)).toBe(
      "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs",
    );
    // The OKP example of the Python SDK (bothy_sdk.auth.jwk_thumbprint).
    expect(dpopThumbprint({ kty: "OKP", crv: "Ed25519", x: "AA" })).toBe(
      "3I7TDbW-lKbNSt8372-ez4nbmhZyQrtliLvMotQEsgE",
    );
  });

  it("refuses an unsupported or incomplete key", () => {
    expect(() => dpopThumbprint({ kty: "oct", k: "AA" })).toThrow(TypeError);
    expect(() => dpopThumbprint({ kty: "EC", crv: "P-256", x: "AA" })).toThrow(
      TypeError,
    );
  });
});

describe("DpopKey", () => {
  it("generates an ES256 key into a new file of mode 0600 and loads it again", async () => {
    const path = join(dir, "es256.pem");
    const key = await DpopKey.generate({ path });
    expect(key.alg).toBe("ES256");
    expect(statSync(path).mode & 0o777).toBe(0o600);
    expect(readFileSync(path, "utf8")).toContain(
      ["BEGIN", "PRIVATE", "KEY"].join(" "),
    );
    await expect(DpopKey.generate({ path })).rejects.toThrow(/EEXIST|exists/);
    const loaded = await DpopKey.load(path);
    expect(loaded.jkt).toBe(key.jkt);
    expect(key.jkt).toBe(dpopThumbprint(key.publicJwk));
    expect(Object.keys(key.publicJwk).sort()).toEqual(["crv", "kty", "x", "y"]);
  });

  it("generates EdDSA keys and keeps them in memory without a path", async () => {
    const key = await DpopKey.generate({ alg: "EdDSA" });
    expect(key.alg).toBe("EdDSA");
    expect(key.publicJwk).toMatchObject({ kty: "OKP", crv: "Ed25519" });
    const path = join(dir, "ed25519.pem");
    const stored = await DpopKey.generate({ path, alg: "EdDSA" });
    expect((await loadDpopKey(path)).jkt).toBe(stored.jkt);
  });

  it("refuses a file readable by group or others", async () => {
    const path = join(dir, "open.pem");
    await DpopKey.generate({ path });
    chmodSync(path, 0o644);
    await expect(DpopKey.load(path)).rejects.toThrow(/group or others/);
    chmodSync(path, 0o640);
    await expect(loadDpopKey(path)).rejects.toThrow(/group or others/);
  });

  it("loads a private JWK file and refuses public keys and other curves", async () => {
    const { privateKey } = generateKeyPairSync("ec", { namedCurve: "P-256" });
    const jwkPath = join(dir, "key.jwk");
    writeFileSync(
      jwkPath,
      JSON.stringify(privateKey.export({ format: "jwk" })),
      {
        mode: 0o600,
      },
    );
    const key = await loadDpopKey(jwkPath);
    expect(key.alg).toBe("ES256");
    const publicOnly = join(dir, "public.jwk");
    const pub = { ...privateKey.export({ format: "jwk" }) };
    delete pub.d;
    writeFileSync(publicOnly, JSON.stringify(pub), { mode: 0o600 });
    await expect(loadDpopKey(publicOnly)).rejects.toThrow(/private/);
    const p384 = generateKeyPairSync("ec", { namedCurve: "P-384" }).privateKey;
    const p384Path = join(dir, "p384.pem");
    writeFileSync(p384Path, p384.export({ format: "pem", type: "pkcs8" }), {
      mode: 0o600,
    });
    await expect(loadDpopKey(p384Path)).rejects.toThrow(/P-256 or Ed25519/);
  });

  it("never renders the private key", async () => {
    const key = await DpopKey.generate();
    const rendered = `${inspect(key, { showHidden: true, depth: 5 })} ${JSON.stringify(key)} ${String(key)}`;
    expect(rendered).not.toContain("PRIVATE");
    expect(rendered).not.toMatch(/"d"/);
    expect(rendered).toContain(key.jkt);
  });
});

describe("createDpopProof", () => {
  it("carries typ, alg, public jwk, jti, htm, htu, iat, ath and nonce; ES256 signs 64 bytes", async () => {
    const key = await DpopKey.generate();
    const proof = await createDpopProof(key, {
      htm: "post",
      htu: "HTTPS://Example.COM:443/api/v1/runs?x=1#frag",
      accessToken: new Secret(CANARY),
      nonce: "n-1",
      now: NOW,
    });
    const { header, claims, signature } = decode(proof);
    expect(header.typ).toBe("dpop+jwt");
    expect(header.alg).toBe("ES256");
    expect(Object.keys(header.jwk as object).sort()).toEqual([
      "crv",
      "kty",
      "x",
      "y",
    ]);
    expect(signature).toHaveLength(64);
    expect(claims.htm).toBe("POST");
    expect(claims.htu).toBe("https://example.com/api/v1/runs");
    expect(claims.iat).toBe(NOW / 1000);
    expect(claims.nonce).toBe("n-1");
    expect(claims.ath).toBe(
      createHash("sha256").update(CANARY).digest("base64url"),
    );
    expect(claims.jti).toMatch(/^[0-9a-f-]{36}$/);
    const other = await createDpopProof(key, {
      htm: "GET",
      htu: "https://example.com/",
    });
    const second = decode(other).claims;
    expect(second.jti).not.toBe(claims.jti);
    expect(second).not.toHaveProperty("ath");
    expect(second).not.toHaveProperty("nonce");
    expect(proof).not.toContain(CANARY);
  });

  it("signs with EdDSA and with a foreign signer of a key store", async () => {
    const ed = await DpopKey.generate({ alg: "EdDSA" });
    const proof = await createDpopProof(ed, {
      htm: "GET",
      htu: "https://example.com/api/v1/gateway",
      accessToken: CANARY,
    });
    expect(decode(proof).header.alg).toBe("EdDSA");
    const signer: DpopSigner = {
      alg: ed.alg,
      publicJwk: ed.publicJwk,
      sign: (data) => ed.sign(data),
    };
    const viaStore = await createDpopProof(signer, {
      htm: "GET",
      htu: "https://example.com/x",
    });
    expect(decode(viaStore).header.jwk).toEqual(ed.publicJwk);
  });
});

describe("URLs", () => {
  it("normalizes htu and the gateway form", () => {
    expect(normalizeHtu("HTTPS://Example.COM:443/api/v1/runs?x=1#f")).toBe(
      "https://example.com/api/v1/runs",
    );
    expect(normalizeHtu("http://[::1]:80/x")).toBe("http://[::1]/x");
    expect(normalizeHtu("http://127.0.0.1:8099/a")).toBe(
      "http://127.0.0.1:8099/a",
    );
    expect(gatewayHtu("wss://Example.com:443/api/v1/gateway?v=1")).toBe(
      "https://example.com/api/v1/gateway",
    );
    expect(gatewayHtu("ws://127.0.0.1:8099/api/v1/gateway")).toBe(
      "https://127.0.0.1:8099/api/v1/gateway",
    );
    expect(gatewayHtu("ws://example.com:80/api/v1/gateway")).toBe(
      "https://example.com/api/v1/gateway",
    );
  });

  it("allows clear text only for loopback hosts", () => {
    for (const ok of [
      "https://example.com/api/v1",
      "http://127.0.0.1:8080/api/v1",
      "http://127.5.0.1/api/v1",
      "http://[::1]/api/v1",
      "http://localhost:1/api/v1",
      "wss://example.com/api/v1/gateway",
      "ws://127.0.0.1:9/g",
    ])
      expect(() => requireSafeUrl(ok), ok).not.toThrow();
    for (const bad of [
      "http://example.com/api/v1",
      "ws://192.0.2.1/g",
      "ftp://example.com/",
      "http://localhost.example.com/",
      "not a url",
    ])
      expect(() => requireSafeUrl(bad), bad).toThrow(/only https and wss/);
  });
});
