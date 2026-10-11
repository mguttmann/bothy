---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# Identity providers: OIDC, proxy header, local passkey accounts

## Context and Problem Statement

The product is self-hosted and public. Instance operators have very different environments: a
company with Microsoft Entra ID, a small team behind a private overlay network with identity
headers (for example `tailscale serve`), or an installation without any identity provider at all.
The product must not require Tailscale or any particular provider (Baseline B9). Native
apps on iOS, Android, macOS and Windows must log in without secrets in the app.

How do people log in, and how is an external identity mapped to an account?

## Decision Drivers

- No self-registration; new people come in through the owner (Study C).
- No provider tokens and no client secrets on devices.
- One token format for all paths, bound to device keys (ADR 0020).
- Secure mapping without mutable attributes such as email.
- Native apps: system browser, no embedded web views (RFC 8252).
- Passkeys only work in native Apple apps with a publicly reachable associated domain
  (Baseline R-Passkeys).

## Considered Options

1. Port with three drivers (OIDC, trusted proxy header, local accounts); the server is the
   OIDC relying party and the authorization server of its own apps.
2. Only OIDC; instances without a provider get a bundled OIDC service.
3. Only identity from the overlay network (proxy headers).
4. Apps log in directly with the provider and send its tokens to the server.

## Decision Outcome

Chosen option: 1, because it covers all three environments without requiring a service, and
because the server, as intermediary, issues uniform, device-bound tokens.

Rulings ([../spec/08-identity.md](../spec/08-identity.md)):

- OIDC: authorization code with PKCE (`S256`) on both legs (app to server, server to
  provider); the server as a confidential client with `private_key_jwt` or a secret from the
  secret provider; check of `iss` against mix-up (RFC 9700, RFC 9207).
- Mapping only via `(issuer, subject)` or a one-time enrollment code; never via email.
- Proxy header only if all listeners are bound to loopback; only from trusted peers;
  only for login, optionally bound to every request.
- Local accounts: passkey (console, public domains), enrollment code, device pairing,
  password with TOTP as a fallback that can be switched off.
- The first owner only via a server command on the host; before that, the API answers
  `503 setup_required`.

### Consequences

- Good, because no instance operator needs a particular provider or a particular network.
- Good, because apps only talk to their own server and hold no provider tokens.
- Good, because an overlooked email match as an attack path (account takeover) is ruled out.
- Bad, because the server must run login flows, redirects and enrollment codes itself
  (more of our own security-related code, hence more tests).
- Bad, because disabling a person at the provider only takes effect at the next login
  (`auth.idp_recheck_interval`); automatic propagation (SCIM or Shared Signals) is a
  proposed follow-up ADR.

### Confirmation

Tests ID-T01 to ID-T11 and ID-T20 to ID-T25 as well as ID-T27 in 08 (PKCE downgrade, `state`,
`iss`, `alg`, mapping without email, `tid` list, first installation, loopback requirement, header
binding, enrollment codes, TOTP replay, response time, Argon2id lower bound, trusted proxies).

## Pros and Cons of the Options

### 1. Three drivers, the server as intermediary

- Good, because all target environments are covered without an additional service.
- Good, because device and token logic is the same for all drivers.
- Bad, because the server implements parts of an authorization server itself.

### 2. Only OIDC with a bundled service

- Good, because only one login path in the server.
- Bad, because small installations must run, back up and update a further service.
- Bad, because the proxy case would still need a bridge.

### 3. Only identity from the overlay network

- Good, because very simple where the network already exists.
- Bad, because it binds the product to a network (forbidden by Baseline B9).
- Bad, because tagged devices and public traffic carry no identity.

### 4. Apps directly with the provider

- Good, because the server runs no login flow.
- Bad, because provider tokens live on devices and the server must check tokens in foreign
  formats.
- Bad, because device binding (DPoP) depends on the provider.
- Bad, because every provider requires its own redirects per platform.

## More Information

- RFC 8252 (Native Apps): https://www.rfc-editor.org/rfc/rfc8252.txt, retrieved 2026-10-10 [verified].
- RFC 9700 (OAuth 2.0 Security BCP): https://www.rfc-editor.org/rfc/rfc9700.html, retrieved
  2026-10-10 [verified].
- Microsoft on claims: https://learn.microsoft.com/en-us/entra/identity-platform/claims-validation,
  retrieved 2026-10-10 [verified].
- Tailscale Serve: https://tailscale.com/kb/1312/serve, retrieved 2026-10-10 [verified].
- Device binding and step-up: ADR 0020.
