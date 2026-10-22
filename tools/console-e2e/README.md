# tools/console-e2e: SEC-T05 and the administration suites, the console under its CSP

The headless browser test of the web console (brief M6-03, [19-security.md](../../docs/spec/19-security.md)
5 and 14, SEC-T05): the console of the real server image runs with its Content Security Policy
enforced in Chromium, Firefox and WebKit, and the test fails on any violation, on any request to an
origin other than the console origin and on a WebSocket the policy blocks (phase 1). Phase 2 (brief
M6-04 Design note 4) runs the `patrol` web suites of the console administration (A1 to A6, and A7
and A8 of brief M6-08: apps, outgoing and incoming webhooks) in Chromium under the same enforced
CSP against the same stack. `make console-e2e` runs both;
it is not part of `make gates`. The CI job `console-e2e` of `.github/workflows/ci.yml` runs it on
every change of the component `apps` or `server` and feeds `ci-ok`.

## Files

| File | Content |
|---|---|
| `console_e2e.py` | the runner on the host: preflight, temp directory, test CA, Compose stack, seed, sidecar, browsers, report check, teardown |
| `headless/console_headless.py` | the browser flow of phase 1; runs inside the Playwright image (service `console-browser`) and hands over to `admin_suites.py` with `E2E_MODE=admin` |
| `headless/admin_suites.py` | phase 2: login, seed through the API and patrol's web protocol in Chromium |
| `tests/test_console_e2e.py` | tests of the runner (teardown after a failure, preconditions, pins of the overlays, the CI job, phase 2, the Dockerfile targets) and of the pure helpers of `admin_suites.py`; part of `make test` |

The stack is the Compose smoke stack of M1-73 (`infra/compose/compose.yaml`,
`infra/compose/test/compose.smoke.yaml`, `smoke.env`) with the overlay
`infra/compose/test/compose.console-e2e.yaml` and `console-e2e.env`; phase 2 adds
`infra/compose/test/compose.console-e2e-admin.yaml` and starts the worker and the webhook receiver.

## Prerequisites

- Docker Engine 26 or newer and Docker Compose 2.26 or newer (as `make portability`).
- The server image: `make image` loads `bothy-server:<services/server/VERSION>-local`, the default;
  `E2E_SERVER_IMAGE` names another loaded image. The image carries the console build, so the test
  needs neither Flutter nor Go on the host.
- `uv` and the tools environment (`make bootstrap-ci`): the runner and the hashes of the browser
  packages come from `tools/uv.lock`.
- `TMPDIR` in a directory the Docker runtime shares with its VM (with colima: below the home
  directory), because the containers mount the temp directory of the run.

A missing precondition exits with code 2 and a message that names the fix.

## Steps

1. Preflight: Docker, Compose and their versions, the server image.
2. Workspace: `mktemp -d` with `secrets-input`, `out`, `logs`, `browser` and `certs`; the
   requirements of the browser image (`playwright`, `greenlet`, `pyee`, `typing-extensions` with
   the hashes of `tools/uv.lock`); a test CA and a leaf certificate for `localhost`, `api`,
   `webhook-receiver` and 127.0.0.1, valid for one day, created for this run only.
3. `compose config`; build of the probe image (`bothy-smoke`) and of the browser image
   (`bothy-console-browser`, the Playwright image pinned by digest plus the locked packages); pull of
   PostgreSQL and the sidecar image; the web bundle of the patrol suites (target `console-e2e-web`
   of `services/server/Dockerfile`, exported to `admin-bundle/` in the temp directory, see phase 2).
4. `compose up --wait api` (secrets init, PostgreSQL, migrations).
5. `bothy setup bootstrap-owner --handle alex` in the running `api`; the enrollment code goes to
   `out/setup.out` (mode 0600, never printed).
6. Seed (`bothy-smoke seed`): the owner's device redeems the code, sets a random password, enrolls
   TOTP and confirms it; the login goes to `out/console-login.json` (mode 0600, never printed).
7. The TLS sidecar `tls` (Caddy) in the network namespace of `api` on port 8443.
8. The browsers (`console-browser`, also in the namespace of `api`) open `https://localhost:8443/console/`.
9. The server log must hold the report of the negative test (`csp violation reported` with
   `script-src-elem` and `inline`).
10. Phase 2, only after phase 1 passed (see below).
11. Teardown, always: `compose down --volumes --remove-orphans --rmi local`, the two images of the
    run, the temp directory with the bundle; objects left with the project label fail the run.

The result of each browser is `out/results/<browser>.json`: the opened paths, `crossOriginIsolated`,
the observed Trusted Types policy names, violations, foreign origins and the browser version. With
`E2E_ARTIFACTS=<dir>` a failed run copies these files there (the CI artifact); they hold no secret.

## The browser flow

Per browser, with the CSP of the server enforced and an init script that records every
`securitypolicyviolation` event and every Trusted Types policy the page creates:

- log in with the console form (login name, password, one-time code), the step-up with password
  and one-time code, every entry of the navigation with its first frame;
- `crossOriginIsolated` must be true (COOP and COEP);
- a WebSocket to `/api/v1/gateway` of the own origin from the page: no `connect-src` violation and
  the handshake reaches the server (the server may refuse the connection; only the policy counts);
- Chromium only: a passkey registered with the virtual authenticator of the browser (CDP
  `WebAuthn.addVirtualAuthenticator`, user verification on) through the registration operations of
  the console session, then a second login whose step-up uses that passkey; and the negative test:
  `index.html` of the server, served by Playwright's route with the server's own headers plus one
  inline script, must block the script, fire a violation event and send a report to the endpoint.

Chromium runs as the full browser in its new headless mode and trusts the leaf by its SPKI hash, so
the certificate counts as valid (WebAuthn needs that). Firefox and WebKit, which have no virtual
authenticator in Playwright, accept the test certificate through the context option and do the
step-up with password and one-time code.

## Phase 2: the patrol suites of the console administration

`patrol_cli` has no web build command, so `apps/console/integration_test/admin/admin_bundle.dart` is
the hand-written bundle entry (in place of patrol's generated `test_bundle.dart`) that registers the
suites `integration_test/admin/*_test.dart`. The stage `console-e2e-build` of the server Dockerfile
builds it from the console stage with the flags of the real console (`--base-href /console/
--no-web-resources-cdn --no-pub --no-wasm-dry-run`) and writes `version.json` the same way; the
stage `console-e2e-web` holds only the built files. No release image contains them.

The runner then:

1. writes the trust bundle `certs/ca-bundle.pem`: the CA file of the server image
   (`/etc/ssl/certs/ca-certificates.crt`, copied with `docker create` and `docker cp` from a
   container that never starts and is removed by its ID) followed by the test CA of the run;
2. adds the overlay `compose.console-e2e-admin.yaml` to the files of the project and starts the
   webhook receiver `webhook-receiver` (brief M6-08): the pinned Caddy image of the sidecar with the
   same hardening, on the Compose network without a published port, TLS on 8443 with the leaf of
   the run (it names `webhook-receiver`), every POST answered 200 and logged in its JSON access log
   (method, path, headers, never a body);
3. recreates `api` and starts the worker (`--no-deps --force-recreate --wait`); the overlay mounts
   the bundle read-only at `/usr/share/bothy/console-e2e` and sets existing configuration keys:
   `BOTHY_CONSOLE__ASSETS_DIR` to that path (the server serves the bundle with the headers and the
   CSP of the console) and `BOTHY_AUTH__STEP_UP_MAX_AGE=3s`, so every changing action of a flow needs
   a fresh step-up through the console's step-up adapter; for `api` (which checks manifest and
   subscription addresses) and the worker (which delivers) `BOTHY_WEBHOOK__ALLOWED_HOSTS` and
   `BOTHY_EGRESS__ALLOWED_HOSTS` list `webhook-receiver:8443` (the egress entry with
   `allow_private`, because the receiver has a private Compose address), and `SSL_CERT_FILE` points
   to the trust bundle, which Go's crypto/x509 reads on Linux; the release image and the
   installation bundle stay unchanged;
4. recreates the TLS sidecar, which shares the network namespace of `api` and must follow the new
   container;
5. runs `console-browser` with `E2E_MODE=admin`; its exit code is the result of the phase;
6. reads the access log of the receiver (`compose logs --no-log-prefix webhook-receiver`) and fails
   unless it holds a POST answered 2xx with a `webhook-id` header; it prints the number of requests
   and the paths, never an ID.

In the browser (`admin_suites.py`, Chromium only, because only Chromium has the virtual
authenticator):

1. one context with the collectors of phase 1, the virtual authenticator, the permissions
   `clipboard-read` and `clipboard-write` for the console origin, a download listener and the binding
   `__patrol__platformHandler`; then the console URL (the bundle shows nothing until a test pumps
   the app);
2. from the page with the cookie: Alex logs in with password and TOTP (`loginWithPassword`, client
   `console`), does the step-up with password and TOTP and at once registers a passkey, so every
   later step-up uses the passkey; a request answered with `401 step_up_required` gets one passkey
   step-up and is repeated once with the same `Idempotency-Key`;
3. the seed through the API as Alex: the spaces Support (channels general, requests), Marketing
   (Kampagnen, Intern: the names of the golden example of 07 9.3) and Engineering (builds); the person Sam with an enrollment code; the group
   Support Team with Sam; a grant of level write for Sam on Support; the agent Support Agent
   (operator of Support, credential holder Alex, billing mode personal) and the agent Texter
   (billing mode organization, operator of Marketing/Kampagnen and Marketing/Intern);
4. Sam's waiting device: two devices of Sam call the API without the cookie, with DPoP proofs
   signed by WebCrypto keys of the page that are not extractable. The first redeems the enrollment
   code, sets a random password and enrolls TOTP; the second logs in with password and TOTP as an
   app and stays `pending`; its verification code comes from `getDevice` as that device;
5. the ids and names make the document of `window.__e2eSeed`, a JSON string (`owner`, `sam`, `pendingDevice` with
   `code`, `spaces`, `channels`, `agents`, `group`; every inner object has `id` and `name`), which
   the Dart tests read;
6. patrol's web protocol: list the tests with `__patrol__getTests` (the internal
   `patrol_test_explorer` is left out), then per test in the listed order a fresh load of the
   bundle, `window.__e2eSeed` again, the initialization and `__patrol__runTest`, each bounded by
   `TEST_TIMEOUT_MS` of `headless/admin_suites.py`. The fresh load is patrol's own rule (its web runner does the same): one load runs every
   test of the bundle once and executes only the requested one. Console lines starting with `PATROL_LOG` (patrol's step log) go to stderr. The
   platform handler answers `startTest`, `verifyFileDownloads`, `getClipboard`, `setClipboard`,
   `grantPermissions`, `enableDarkMode`, `disableDarkMode` and `e2eSetManifestFile`; any other
   action fails the run with its name. `e2eSetManifestFile` with `{"name": "repo-bridge.json"}`
   waits until the console has appended its file input
   (`input[data-bothy-file-input="manifest"]`) and sets the file on it: without a user activation
   Chromium opens no file dialog for the console's `click()`. The file is the manifest of
   `repo_bridge_manifest` (the app Repo Bridge: required scope `messages:write`, optional
   `messages:read` and `events:subscribe`, the event `message.created` delivered to
   `https://webhook-receiver:8443/hooks/events`, one requested grant of level write on a channel,
   `min_api_version` 0.2); the runner serves no other name.

The suites of brief M6-08:

- A7 installs Repo Bridge from that file with the consent of 11 4.1 (the optional scopes and the
  event kept, the grant on Engineering/builds, a token for 30 days), sees the server's preview of
  the grant, confirms with step-up and sees the app token and the key ID once; after the close no
  widget shows any part of the token. On the app page it creates a subscription to
  `https://webhook-receiver:8443/hooks/second` for `message.created`, posts a message into
  Engineering/builds with the console session (`createMessage`, cookie and `X-Console-Request: 1`)
  and reloads the deliveries of the subscription until one shows succeeded.
- A8 creates an incoming webhook on Support/general with a new incoming app (the owner's option of
  11 8.1), reads its address from the once-only dialog into a local variable, closes it, posts a
  native message to the address from the page without any credential (`201`), reads the message
  through the API with the console session, checks that the reopened screen shows no part of the
  token, revokes the webhook with the confirmation and expects `404` for the next post.

A token or the webhook address is never printed: the Dart helpers keep it out of every failure
message (`hideInFailures` in `support.dart`), and a check that fails names only the kind of secret.

The phase fails on a failed or skipped test, on an empty test list, on an unsupported platform
action, when the webhook receiver logged no delivery with a `webhook-id` header, on any CSP violation, console CSP error or foreign origin, and when no download ends with
`.csv` and has `occurred_at` in its first line (the export of A6). The result is
`out/results/admin.json`: per test its name, result and details (cut to `DETAILS_MAX` characters), the
platform actions called, the downloads (file name and first line), violations, console CSP errors,
foreign origins and the console output of the page (the reason of a failed expectation; patrol
itself often reports a failure without details); each failed test also leaves a screenshot
`out/results/admin-<n>-failure.png`. `E2E_ARTIFACTS` copies the JSON files and the screenshots.
Only patrol's step log goes to stderr; no secret, code or token is printed.

## Variables

| Variable | Default | Meaning |
|---|---|---|
| `E2E_SERVER_IMAGE` | the tag of `make image` | server image to test |
| `E2E_BROWSERS` | `chromium,firefox,webkit` | browsers to run |
| `E2E_KEEP` | `0` | `1` keeps the stack and the temp directory (they hold the seed login; remove them) |
| `E2E_ARTIFACTS` | none | directory for the result files of a failed run |

`E2E_MODE` and `E2E_ADMIN_BUNDLE` are set by the runner for phase 2; an exported `E2E_MODE` of the
calling shell is ignored.

Every exported `BOTHY_*`, `COMPOSE_*` and `OTEL_*` variable of the calling shell is removed before
Compose runs, so the `.env` of an installation is never read.

## What is not proven

Safari itself (WebKit in Playwright stands in for it), a browser on the host (the browsers run in the
Linux Playwright image locally as in CI), the passkey step-up in Firefox and WebKit, and the
terminal stream under the CSP (M6-18 runs this test with an open terminal).

Phase 2 runs only in Chromium, so the administration screens are not proven in Firefox or WebKit;
the manifest file reaches the console through Playwright's file input API, not through a file
dialog a person uses; the webhook receiver checks no signature (the server's signing is proven by
the tests of M2-26);
it tests the bundle of the suites, which shares the code but not the entry point of the console
build in the image; and the seed creates its objects through the API, not through the screens.
