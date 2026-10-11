---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# Device binding, DPoP, step-up for admin actions

## Context and Problem Statement

Tokens end up in logs, crash reports, proxies and device backups. A stolen
bearer token opens everything the person may do until it expires. Administrative actions (grants,
owner, agent policy, signing keys) should not be possible even if someone takes over a
valid session. Passkeys as the only factor fail in native Apple apps without a
publicly reachable domain (Baseline R-Passkeys). The admin console runs in the browser.

How do we bind tokens to devices, and how does a person confirm administrative actions?

## Decision Drivers

- A stolen token alone must be of no use.
- Works behind TLS-terminating proxies (reverse proxy, `tailscale serve`).
- Works in native apps without a public domain.
- Confirmation by the person at the device, not by code in the process.
- Browser console without tokens in JavaScript.

## Considered Options

1. Device keys in secure storage: a DPoP key for every request (RFC 9449) and a
   step-up key with user confirmation per use; console with a session following the
   backend-for-frontend pattern and step-up via passkey.
2. Bearer tokens with a short lifetime and refresh rotation.
3. Mutual TLS with client certificates.
4. Passkeys (WebAuthn) everywhere, also as device binding.

## Decision Outcome

Chosen option: 1, because it binds tokens at the application layer (independent of where TLS
terminates), gets by without a public domain and ties step-up to the biometrics or PIN of the
device.

Rulings ([../spec/08-identity.md](../spec/08-identity.md)):

- Two non-exportable keys per native device: `dpop` (without a prompt per use) and
  `step_up` (biometrics or PIN per use).
- Access token with `cnf.jkt` and `sid`, short-lived; DPoP proof with server nonce, `ath`,
  `jti` check; bearer use of a DPoP token is rejected.
- Refresh tokens rotating in families, bound to the DPoP key of the family; reuse
  revokes the family and the session. Separate families for app extensions that refresh
  on their own.
- Step-up is valid for `auth.step_up_max_age` for the session in which it was performed; only
  people.
- Console: cookie with `__Host-`, `Secure`, `HttpOnly`, `SameSite=Strict`; CSRF header and
  `Origin` check; step-up via passkey, otherwise a new OIDC login with `max_age=0`.

### Consequences

- Good, because token theft without the key is worthless.
- Good, because proxies with TLS termination do not break the binding.
- Good, because administration needs a physical confirmation at the device.
- Bad, because every request needs a signature and the server needs a `jti` check and nonce
  management.
- Bad, because the platforms differ; whether Windows Hello delivers the step-up key as
  `ES256` is open.
- Bad, because the console has a second session path (cookie) that needs its own tests.

### Confirmation

Tests ID-T12 to ID-T19 and ID-T26 in 08 (DPoP error cases, refresh reuse, separate
families, key binding of the family, step-up cases, console cookie and CSRF, device revocation);
guard rule SR5 in 07 with AZ-M35 and AZ-M36.

## Pros and Cons of the Options

### 1. Device key with DPoP and step-up key

- Good, because standardized (RFC 9449) and independent of the transport.
- Good, because keys in secure storage cannot be copied.
- Neutral, because hardware binding cannot be proven without attestation; attestation is
  stored and can become mandatory later.
- Bad, because more logic in clients and server.

### 2. Bearer with a short lifetime

- Good, because simple and supported everywhere.
- Bad, because a stolen token is fully usable until it expires, and a stolen
  refresh token fetches new tokens until the rotation notices the theft.

### 3. Mutual TLS

- Good, because the binding lies in the transport.
- Bad, because TLS-terminating proxies do not pass the certificate through without the server
  having to trust a header.
- Bad, because distributing certificates to mobile devices and in the browser is laborious.

### 4. Passkeys everywhere

- Good, because phishing-resistant and standardized.
- Bad, because native Apple apps need a publicly reachable associated domain; private
  instances would have no way.
- Bad, because WebAuthn provides no binding of individual API requests.

## More Information

- RFC 9449 (DPoP): https://www.rfc-editor.org/rfc/rfc9449.html, retrieved 2026-10-10 [verified].
- RFC 9700, section 2.2.2 (refresh tokens of public clients sender-constrained or rotating),
  retrieved 2026-10-10 [verified].
- RFC 10017 (browser applications, BFF pattern and cookie attributes):
  https://datatracker.ietf.org/doc/rfc10017/, retrieved 2026-10-10 [verified].
- WebSocket variant of the DPoP proof: [../spec/04-sync-protocol.md](../spec/04-sync-protocol.md),
  R-SY-01.
