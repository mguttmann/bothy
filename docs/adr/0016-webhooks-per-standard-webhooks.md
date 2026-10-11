---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team B (API)
informed: all teams
---

# Webhooks per Standard Webhooks

## Context and Problem Statement

Apps never run in the server process (B6). Whoever does not hold a gateway connection learns about
events through outgoing webhooks. Receivers must be able to check authenticity, freshness and
duplicates; the server must not hold third-party secrets in plain text for this (03, principle 9).
Which format and which signature do we use?

## Decision Drivers

* Receivers verify with existing libraries instead of their own code.
* No stored secret per subscription in the database.
* Key rotation without downtime.
* The same event shape as the gateway and `GET /events`.

## Considered Options

* Standard Webhooks: `webhook-id`, `webhook-timestamp`, `webhook-signature`; signature `v1a`
  (Ed25519) always, `v1` (HMAC) can be enabled per subscription.
* Custom HMAC header format modeled on large platforms
  (`X-Signature-256: sha256=...`).
* Body as JWS (signed JSON).

## Decision Outcome

Chosen option: "Standard Webhooks with v1a always and v1 optional", because it is an open,
documented standard with reference libraries, explicitly provides for asymmetric signatures
and allows several signatures for key rotation.

Rulings (docs/api/webhooks.md):

* `webhook-id` is the event ID; the signed content is `{id}.{timestamp}.{body}`.
* `v1a` with the server key of the purpose `webhook_out`; public keys under
  `GET /api/v1/instance/webhook-keys`.
* `v1` with a derived secret (`HMAC` from the master key of the secret provider,
  subscription ID and generation); nothing is stored, it is shown once.
* The body is the event envelope; `occurred_at` instead of the recommended name `timestamp`.
* Retry with backoff up to `webhook.retry_max_age`; `410` switches the subscription off; every
  attempt runs through `authorize`.

### Consequences

* Good, because receivers can use widespread libraries for HMAC and need only the
  standard library of their language for Ed25519.
* Good, because a leaked database dump contains neither signing keys nor HMAC secrets.
* Bad, because two methods must be tested (G10).
* Bad, because, as far as we know, the reference libraries do not describe `v1a` [assumed];
  receivers with Ed25519 verify by hand following webhooks.md.
* Neutral, because the body shape deviates from the recommendation; the standard does not require
  it.

### Confirmation

Gate G10: test vectors (key, timestamp, body, expected signatures) in the repo; a
delivery in the integration test is verified with the reference library (`v1`) and with Ed25519
from the standard library (`v1a`). Lint (G1) holds the event schemas under `webhooks:`, G2 the
receive actions against 05-actions.md.

## Pros and Cons of the Options

### Standard Webhooks

* Good, because open (Apache-2.0), with libraries in many languages.
* Good, because rotation through several signatures in one header.
* Bad, because a young standard; the asymmetric variant is less well documented in the libraries.

### Custom HMAC format

* Good, because very simple.
* Bad, because a stored secret per subscription; no standard, every platform differs.

### Body as JWS

* Good, because asymmetric, keys via JWKS.
* Bad, because the body is no longer readable JSON; the receivers' tools expect JSON;
  replay protection must be built separately.

## More Information

* Specification: https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md
  (retrieved 2026-10-10) [verified].
* Libraries: https://github.com/standard-webhooks/standard-webhooks (retrieved 2026-10-10);
  support for `v1a` is not described there [assumed].
