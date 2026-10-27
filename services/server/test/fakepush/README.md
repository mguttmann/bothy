# Package `test/fakepush`

Status: built (M5-11). Component `testsupport`, never imported by production code. The fake push
provider of the mobile device E2E test (brief M5-11 Design note 1): Fake-APNs of M3-20
(`internal/notify/adapters/apns/apnstest`) and Fake-FCM of M3-21
(`internal/notify/adapters/fcm/fcmtest`) as one program with a capture API, run as the Compose
service `fakepush` of [compose.e2e-mobile.yaml](../../../../infra/compose/test/compose.e2e-mobile.yaml)
([infra/compose/test/README.md](../../../../infra/compose/test/README.md), section "Mobile E2E push
path (M5-11)"). Not to be confused with `internal/notify/adapters/fakepush`, the in-process
test double of the port `app.PushProvider` (M3-19), which speaks no HTTP.

## What it does

- **Run material.** `Start` generates per run, into `Options.Dir` (the named volume `/push`): the
  APNs ES256 key `apns_key.p8` (PKCS #8 PEM, mode `0600`), the FCM service account
  `fcm_service_account.json` (RSA 2048, `token_uri` = `https://<Host>:<FCM port>/token`, mode
  `0600`) and `ca.pem`, the per-run CA (mode `0644`) of the TLS leaf both fakes serve for `Host`.
  The CA key exists only in memory while the leaf is signed. Files are written through a
  temporary name and a rename.
- **Fakes.** Fake-APNs (HTTP/2) and Fake-FCM (HTTP/1.1) verify what Apple and Google verify: the
  provider JWT (`iss` = `APNsTeamID`, `kid` = `APNsKeyID`, ES256 with the generated key), the RS256
  assertion of the service account and the bearer token. Every valid send is answered `200`;
  there is no script.
- **Capture API.** Plain HTTP on `Options.CaptureAddr`: `GET /healthz` (200 once both fakes listen
  and every file is written, because the listener starts last) and `GET /captured?after=<seq>`
  with `{"items":[...]}`, the accepted sends with `seq > after` in increasing `seq` from 1 over
  both fakes (`OnAccept` of the fakes numbers them under one lock). An APNs item has `token`,
  `topic`, `push_type`, `priority` and `payload` (the body as sent); an FCM item `token`, `data`
  and `android` when present. `after` must be one whole number of at least 0 (else 400).

The example identity (`APNsTeamID`, `APNsKeyID`, `FCMProjectID`, `FCMClientEmail`, `FCMKeyID`)
is what the overlay configures for the server; `TestOverlayMatchesTheFake` holds both equal.

## Command

`cmd/fakepush`, built as the target `fakepush` of `services/server/Dockerfile` (distroless static,
user `65532`, `/push` owned by `65532` with mode `0700`):

- `fakepush serve [--dir /push] [--host fakepush] [--apns-addr :8443] [--fcm-addr :8444]
  [--capture-addr :8090]` prints one line `fakepush: ready apns=... fcm=... capture=...` and runs
  until SIGTERM or SIGINT; it logs addresses and errors, never a key, token or payload.
- `fakepush health [--addr 127.0.0.1:8090]` exits 0 when `/healthz` answers 200, else 1: the
  Compose healthcheck of an image without a shell.

Exit codes: 0 success, 1 failure, 2 usage.

## Tests

`go test -race ./test/fakepush/...`: the run material (modes, the secret file driver of the server
accepts the files, the CA verifies both fakes), the real APNs and FCM adapters of the server
sending to the fakes and the capture API returning their payloads, order and `after`, the
commands, and the consistency with the overlay. No Docker needed.
