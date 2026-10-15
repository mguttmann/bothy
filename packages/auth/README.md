# auth

Login, tokens, DPoP proofs (RFC 9449), single-flight refresh and step-up of the apps, without
Flutter.

Kind: pure Dart package, a member of the Dart workspace at the repository root
(`resolution: workspace`, `publish_to: none`). It performs HTTP only through the `api_client` it
is given; keys, secrets, the browser, the app redirect, passkeys and time come in through ports,
so every flow runs against fakes in the tests.

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`):
`api_client`, `domain` (arrows of the diagram). Every other member is out of reach; `make arch`
fails on a dependency or an import of it. From pub.dev: `crypto` (SHA-256 for `ath`, PKCE and the
JWK thumbprint) and the types of the `api_client` surface (`dio`, `built_value`,
`built_collection`). `pointycastle` is a dev dependency of the software signer in `test/support/`
only.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

## Ports to implement per platform

| Port | Purpose | Implemented by |
|---|---|---|
| `DeviceKeySigner` | ES256 key that returns the JOSE `R \|\| S` signature (64 bytes) | M4-10 (Apple), M4-11 (Windows), M5-01 (Android) |
| `DeviceKeys` | the pair `dpop` (no confirmation) and `stepUp` (user presence) | same |
| `SecretStore` | refresh token and device ID by key; a write is durable when its future completes | M4-18 |
| `BrowserLauncher` | opens the OIDC URL in the system browser, never a web view | M4-18 |
| `RedirectReceiver` | the `app_redirect_uri` (loopback `127.0.0.1` on desktop) and the redirect back | M4-18 |
| `PasskeyAuthenticator` | WebAuthn get and create, only for instances with a public domain | M4-18 |
| `StepUpPrompt` | explains the step-up in the person's language before the step-up key signs | M4-18 |
| `Clock` | time and polling waits (`SystemClock` in the app) | this package |

## Session and interceptor order

One `AuthSession` per server address owns the token state, the nonce store, the refresh
coordinator, the step-up service and the login flows; there is no global state. It builds its
client through `createApiClient` (M4-02) and fills the auth slot in this order:

1. `RefreshInterceptor`: refreshes before a request whose access token has expired by the clock,
   refreshes once and repeats once on `401 token_expired` or `401 unauthenticated`, and ends the
   session on `401 session_revoked`.
2. `DpopInterceptor`: adds `Authorization: DPoP <token>` and a fresh proof with `ath`, gives the
   unauthenticated login completions a proof without `ath`, adopts every `DPoP-Nonce` and repeats
   once on `401 use_dpop_nonce`.
3. `StepUpInterceptor`: on `401 step_up_required` runs the step-up once and repeats the original
   request with the same `Idempotency-Key` and body, a new proof and the current token.

The refresh runs before the proof is built, so `ath` always hashes the token that is sent. Every
interceptor repeats a request at most once; a second identical problem goes to the caller.

`DpopProofFactory.forGateway` builds the gateway proof (`htm = GET`, `htu` the HTTPS form of the
gateway URL, the nonce of `hello`) for M4-13; `RefreshCoordinator.refreshNow` serves the gateway's
`reauth_required`, and `AuthSession.sessionRevoked` the close code `4012`.

## Flows

`LoginService` has one method per flow, each returning a `LoginOutcome` (`LoginSignedIn`,
`LoginPending`, `LoginNeedsSecondFactor`, `LoginFailed`):

- OIDC with PKCE (`beginOidc`): `client_challenge` (S256), system browser, redirect with
  `login_code`, `completeOidcLogin` with `client_verifier`, the proof and the device fields.
- Password with TOTP (`loginWithPassword`): without a TOTP code nothing is sent and the outcome
  is `LoginNeedsSecondFactor`.
- Proxy header (`loginWithTrustedHeader`), passkey (`loginWithPasskey`), enrollment code
  (`redeemEnrollmentCode`, `enrollWithPasskey`), pairing (`beginPairing`, `pollPairing`).
- Pending device (`pollPendingDevice`) and `logout`, which clears the store and signs out even
  offline.

Codes are normalized before they are sent (`normalizeCode`: spaces and hyphens removed, a leading
`en_` in lower case, the rest in upper case). Polling waits with `client.outbox_retry_base`
doubling up to `client.outbox_retry_cap` from `AppConfig`.

## Events

`AuthSession.events` is a broadcast stream of `SignedIn`, `DevicePending`, `SignedOut(reason)`
and `RefreshFailed`. `SignedOut.deleteLocalData` is true for a revoked session or device; the app
then deletes the local data of that server (R-SY-14).

## Secrets

The access token stays in memory; the refresh token lives in the `SecretStore` and is written
before a rotated access token is published. No `toString`, exception or log line of this package
contains a token, proof, code, verifier, password or TOTP value; they print the kind and the
length only (`test/no_secrets_test.dart`).

## Layout

| Path | Content |
|---|---|
| `lib/auth.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/` | ports, JOSE, proofs, nonce store, tokens, refresh, step-up, interceptors, login flows, events |
| `test/` | tests with a software P-256 signer and a fake API, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
