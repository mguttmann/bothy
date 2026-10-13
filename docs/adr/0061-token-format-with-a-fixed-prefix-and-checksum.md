---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team C (identity), Team H (quality and repo), Team B (API)
informed: Team D (agent platform)
---

# Token format with a fixed prefix and checksum

## Context and Problem Statement

Agents, apps, personal tools and incoming webhooks authenticate with long-lived API tokens
([08-identity.md](../spec/08-identity.md), section 12). Experience shows that such tokens end
up in configuration files, logs and commits. A secret scanner finds them only if they differ from
other random data (hashes, IDs). Random hits and typos should not burden the server with
database queries. Which format do API tokens have?

## Decision Drivers

- A secret scanner detects leaked tokens reliably and with few false alarms (gate
  `gitleaks`, [23-quality.md](../spec/23-quality.md)).
- Typos and random hits are rejected without database access.
- Enough randomness that guessing is hopeless; only a hash is stored (03, principle 9).
- Revocation takes effect immediately; the kind of token is visible.
- The product name appears only in listed places (ADR 0044, ruling L6).

## Considered Options

- Fixed prefix, kind, randomness in Base62 and checksum: `bothy_<kind>_<random>_<checksum>`
- Opaque randomness without structure
- Self-contained signed tokens (JWT or PASETO)
- Capability tokens with attenuation (Macaroons, Biscuit)
- Fixed prefix without a checksum

## Decision Outcome

Chosen option: "Fixed prefix, kind, randomness and checksum", because scanners find the prefix
unambiguously and the checksum discards false alarms without a database. Normative in
[08-identity.md](../spec/08-identity.md), section 12, and
[03-data-model.md](../spec/03-data-model.md), table `api_tokens`.

1. **Form.** `bothy_<kind>_<random>_<checksum>`. Kind: `agt` (agent), `app` (app), `pat`
   (personal), `whi` (incoming webhook), `hpc` (pairing code of a host for the terminal,
   addendum 2026-10-10, [24-terminal.md](../spec/24-terminal.md) section 3.2). Randomness: 256 bits
   from a cryptographic random number generator, in Base62 as exactly 43 characters. Base62 uses the
   alphabet `0-9A-Za-z` in this order (digit values 0 to 61). Checksum: CRC-32 (IEEE) over the
   characters of the random part, in Base62 as exactly 6 characters with leading zeros.
2. **Check order** on every request: form, checksum, then lookup via
   HMAC-SHA-256 with the server pepper, then expiry, revocation and DPoP binding. An error in form
   or checksum results in `401` without database access.
3. **Storage.** Only the HMAC (`token_hash`) and `token_prefix` (`bothy_<kind>_` plus the first
   6 characters of the random part) for recognition in the console. Shown exactly once on
   `token.create`.
4. **Limits** as in 08, section 12: mandatory expiry up to `auth.api_token_max_ttl` or
   `auth.personal_token_max_ttl` respectively, DPoP binding per `auth.dpop_required_for`, no
   step-up, no owner permissions through personal tokens.
5. **Scanner.** The gitleaks rule `bothy-api-token` (`.gitleaks.toml`) detects exactly this
   format, with exactly 6 characters of checksum.

The checksum is not a protection against forgery but a filter: CRC-32 detects typos and
random hits; the HMAC comparison checks authenticity. GitHub uses the same pattern (prefix,
underscore, Base62, CRC-32 in 6 characters) for its tokens [Q1, verified].

### Consequences

- Good, because a leaked token is recognized as a product token in the repo (gate `gitleaks`) and,
  with custom patterns, in secret scanning services; the underscore separates the prefix from
  Base62 data [Q1, verified].
- Good, because junk tokens (scanner hits, typos, attacks with random values) never reach the
  database.
- Good, because a fast HMAC is enough: with 256 bits of randomness guessing is hopeless, and a slow
  password hash would only slow down every request.
- Bad, because the working name is in the prefix; a rename needs a transition period in which
  the server accepts old prefixes until the tokens expire (ADR 0044 lists the place).
- Bad, because tokens are not self-contained: every request needs a lookup (with a cache).
  In return, a revocation takes effect immediately.

### Confirmation

- `tools/ci/tests/test_gitleaks_rules.py`: the rule `bothy-api-token` finds a token built at
  runtime with 43 characters of randomness and 6 characters of checksum.
- Test ID-T23 ([08-identity.md](../spec/08-identity.md), section 15): only the hash is stored,
  it is shown once, a wrong checksum and expired tokens are rejected, an agent token that requires
  DPoP is rejected without a proof.
- Unit test of the format: generating and parsing yield the same result; a checksum with leading
  zeros has 6 characters; every change of a character in the random part is caught by the checksum.
- Test: a token with a wrong checksum is rejected without a database query (the query counter of
  the test repository stays 0).
- Build ticket M1-44 in [PLAN.md](../PLAN.md). Built (2026-10-13): `TestFormat_RoundTrip`,
  `TestFormat_ChecksumVectors`, `TestParse_Rejects`, `TestGenerate_ReadsExactly32BytesFromCryptoRand`
  and `TestTokenPrefix` in `services/server/internal/identity/domain/apitoken`, the vectors
  recomputed by `test_checksum_vectors_match_go`, and `TestIDT23_WrongChecksumRejectedWithoutLookup`
  (lookup counter stays 0). Identity does not cache token lookups (one indexed query per request);
  authz caches the token liveness with its rights snapshot per rights version, and every
  revocation raises that version, so a revocation holds at the next request in every process.

## Pros and Cons of the Options

### Prefix, kind, randomness, checksum

- Good, because it is recognizable, quick to discard and needs no state in the format.
- Bad, because the form has more rules than pure randomness.

### Opaque randomness

- Good, because it is simple.
- Bad, because scanners cannot tell such values apart from hashes or IDs; GitHub names exactly
  that as the reason for its new format [Q1, verified].

### JWT or PASETO

- Good, because the server could check without a lookup.
- Bad, because a revocation needs a deny list or short lifetimes; long-lived
  agent tokens, however, should be revocable immediately.
- Bad, because the tokens are long and carry content (claims) readable in logs.

### Macaroons or Biscuit

- Good, because the holder can further restrict permissions themselves.
- Bad, because permissions in the product come from grants and scopes in the server (07, 11); a
  second permission language in the token would contradict the one authorization function
  (ADR 0017).

### Prefix without a checksum

- Good, because it is shorter.
- Bad, because every hit with a matching prefix triggers a database query and scanners report more
  false alarms; according to GitHub, only the checksum almost entirely eliminates false alarms when
  scanning without a database [Q1, verified].

## More Information

- Source (retrieved 2026-10-10): Q1 GitHub Blog, "Behind GitHub's new authentication token
  formats",
  https://github.blog/engineering/platform-security/behind-githubs-new-authentication-token-formats/
  [verified].
- Related: ADR 0020 (DPoP), ADR 0038 (local gates), ADR 0044 (working name).
