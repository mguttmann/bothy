# Package `test/smoke`

Status: built (M1-73). Component `testsupport` (M1-05), never imported by production code. The
probe of the Compose smoke test: a black-box client of a server started from the real image, and
the scanner of its collector export and container logs. The harness around it (`smoke.sh`, the
test overlay, the variables, the steps) is documented in
[infra/compose/test/README.md](../../../../infra/compose/test/README.md).

## Black box

The package imports only the generated `api/rest` and `github.com/coder/websocket` besides the
standard library, never a package below `internal/`; `TestBlackBoxImports` holds this with
`go list`. It builds DPoP proofs and the step-up JWS itself with `crypto/ecdsa` (ADR 0067: the
product has no JOSE library) and checks the server only through the public listener.

## Command

`cmd/bothy-smoke` with the subcommands `precheck`, `run`, `scan`, `seed`, `alerts` and
`alert-stub` (flags `--base-url`, `--public-url`, `--out`, `--in`, `--ctl`, `--listen`,
`--timeout`, `--expect`); the image target `smoke` of `services/server/Dockerfile` builds
it. The environment gives `SMOKE_PUBLIC_URL` and `SMOKE_CANARY`. Steps log `ok <step>` on stderr,
never a value. Exit codes: 0 pass, 1 a check failed, 2 usage. `seed` prepares the console E2E test
of M6-03 instead of the scenario: it reads `setup.out`, gives the owner a random password with a
confirmed TOTP and writes `console-login.json` (mode 0600, never printed).

## Files

- `dpop.go`: `Key` (`PublicJWK`, `Thumbprint` per RFC 7638), `NewProof`, `StepUpSignature`.
- `alerts.go`: `SetupAlertChannel` (the channel "Operations", an app with `write` and an incoming
  webhook after a step-up with the device key), `AwaitAlerts`, `ExpectedAlert` and the
  `HTTPClient` methods `InstallApp`, `CreateGrant`, `CreateIncomingWebhook`,
  `ListThreadMessages` (M7-20, `make alert-e2e`).
- `alertstub.go`: `AlertStub`, the stand-in for the external on-call and heartbeat receivers and
  the scrape target of the test rules (M7-20).
- `seed.go`: `Seed`, `SeedREST`, `ConsoleLogin`, `TOTPStep`, `TOTPCode` (RFC 6238 with
  HMAC-SHA-1) and `HTTPClient.SetMyLocalCredential` (M6-03).
- `htu.go`: `GatewayHTU`, `RESTHTU` (the rule of the server, M1-33).
- `rest.go`: `REST`, `HTTPClient` (nonce tracking, one retry on `use_dpop_nonce`,
  `Idempotency-Key`), `StatusError`.
- `gateway.go`: `Gateway`, `WSGateway` (reader, heartbeats with a random first delay).
- `scenario.go`: `Precheck`, `Run` and the constants `liveMessages`, `awayMessages`,
  `revokeCloseWait`, `frameWait`.
- `enroll.go`: `ReadEnrollmentCode`.
- `scan.go`: `Canaries`, `Scan` with its positive control, `Roots`, `CheckTelemetry`.

## Tests

```sh
cd services/server
go test -race ./test/smoke/...
```

The scenario tests run `Run` against a fake server (`scenario_test.go`) whose knobs break one
guarantee each: order, `client_msg_id` echo, the cursor floor, `catchup.complete`, a live event
before it, the close `4012`, a dispatch after the revocation and the old token. The HTTP client
tests use `httptest` on `127.0.0.1` only.
