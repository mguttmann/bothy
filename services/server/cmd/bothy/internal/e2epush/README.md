# Package `cmd/bothy/internal/e2epush`

Status: built (M5-11). The push endpoints of the E2E server build of the mobile device E2E test
(brief M5-11 Design note 1): `Apply` reads `E2E_PUSH_APNS_URL` and `E2E_PUSH_FCM_URL` from the
process environment and sets `wire.Deps.APNsEndpoints` (environments `production` and `sandbox`)
and `wire.Deps.FCMEndpoints` (send base, and token URL as the base plus `/token`), so the server
sends to the fake push provider of [test/fakepush](../../../../test/fakepush/README.md).

- Only `cmd/bothy/e2e_push.go` (Go build tag `e2e`) imports the package; the release build
  compiles `e2e_push_off.go`, a no-op, and never links this package. `TestReleaseBuildHasNoE2EPush`
  holds that with `go list` with and without the tag and checks that no other Go file of the
  module names the variables.
- The build fails closed: a missing variable or one that is not an https base URL stops
  `bothy serve` with exit code 2 and a message that names the variable, so a misconfigured run
  never reaches Apple or Google.
- The fakes' CA comes from `SSL_CERT_FILE`, which Go reads on Linux; no root pool is set here.
- The variables do not start with `BOTHY_`, so the configuration loader, which refuses unknown
  `BOTHY_` variables, never sees them.

The image target `e2e-server` of `services/server/Dockerfile` builds the binary with
`-tags e2e`; `infra/compose/test/compose.e2e-mobile.yaml` sets both variables.
