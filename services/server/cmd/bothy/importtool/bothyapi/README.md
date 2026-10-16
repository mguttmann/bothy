# cmd/bothy/importtool/bothyapi

Authenticated client of the Bothy API for the import tool `bothy import`
([22 Import and migration](../../../../../../docs/spec/22-import-migration.md), sections 2, 8 and
10; [08 Identity](../../../../../../docs/spec/08-identitaet.md), sections 5.3, 6.4, 7.3 and 7.4).
The tool reads and writes only through the public API (R-IM-01); `TestClientImportsNoServerModule`
holds that the package uses no server module, only `internal/platform/egress` and the kernel.

## Login and logout

`Login` logs the tool in as a device with `platform = cli` through device pairing: an Ed25519
DPoP key, a UUIDv7 device ID, `beginPairing` without a step-up key, the pairing code alone on
standard output and the instruction on standard error, then `completePairing` every
`import.login_poll_interval` until `201` (session stored), `410` (`ErrPairingGone`) or
`expires_at` (`ErrPairingExpired`). Ctrl-C writes nothing (`ErrInterrupted`). A valid session
for the same server is refused (`ErrAlreadyLoggedIn`). The tool never provides step-up: every
`import.manage` step happens in the web console (22 8).

`Logout` takes the lock, calls `logout`, deletes the session file (also when the server already
revoked the session) and keeps the lock file.

## Session file

| Member | Content |
|---|---|
| `version` | `1` |
| `server_url` | origin of the server |
| `device_id`, `user_id` | the `cli` device and its person |
| `dpop_private_jwk` | Ed25519 private JWK (RFC 8037) |
| `access_token`, `access_expires_at` | access token and the client's expiry (`expires_in` from the answer) |
| `refresh_token`, `refresh_expires_at` | refresh token and its expiry |

Default path `<os.UserConfigDir()>/bothy/import-session.json`, `--session-file` on every
command. Mode `0600` in a directory with mode `0700`; anything group or others can access is
refused (`ErrInsecureSessionFile`; on Windows the file lives in the user profile and the mode
check is skipped). Stores are atomic (temporary file, `fsync`, rename, `fsync` of the directory).
`Session` prints only `server_url` and `device_id` in every format.

## Transport

`New` returns the generated client of [api/restclient](../../../../api/README.md) over a
transport that, per request:

1. takes the access token, refreshing first when `access_expires_at` has passed;
2. sets `Authorization: DPoP <token>` (not on pairing and refresh) and a fresh proof: `typ`
   `dpop+jwt`, `alg` `EdDSA`, public `jwk`, random 128-bit `jti`, `htm`, `htu` (server origin plus
   path, without query and fragment), `iat`, `ath`, and the last `DPoP-Nonce`;
3. repeats once on `401` with `error="use_dpop_nonce"`, refreshes and repeats once on `401` with
   `error="invalid_token"`;
4. waits `Retry-After` on `429` at most `import.discord.max_429_retries` times and aborts a wait
   above `import.discord.max_rate_wait` (`ErrRateLimited`, `ErrRateWaitTooLong`);
5. never follows a redirect (`ErrRedirect`);
6. adds one `Idempotency-Key` per call to `POST`, `PUT`, `PATCH` and `DELETE` outside `/auth`.

A refresh holds the exclusive lock of `<file>.lock` (`flock`, `LockFileEx` on Windows), reads the
file again and uses tokens another process already rotated; otherwise it calls `refreshToken`,
stores the new pair and only then uses the new access token. A `401` of the refresh is
`ErrSessionRevoked` (run `bothy import login`); nothing is deleted.

Every connection goes through `platform/egress`: the server host is the only allowed host (it may
resolve to a private address), `http` only for a loopback server, no environment proxy, no
redirects.

## Limits

`Limits` carries `import.login_poll_interval`, `import.discord.max_429_retries`,
`import.discord.max_rate_wait`, `import.http_timeout` and `egress.max_response_bytes`;
`DefaultLimits` returns the defaults until the tool's configuration loader (M8-04) fills them.

## Test fake

`bothytest` is a fake Bothy on `127.0.0.1` for the auth paths: pairing with approve, reject and
expiry, refresh rotation with reuse revocation, logout, `getMe`, `getInstance`, DPoP checks with
nonces, `429`, redirects and an echo of the presented tokens for the leak test. Only tests import
it (`TestNoTestPackageInBinary`).
