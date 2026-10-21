# tools/console-e2e: SEC-T05 and the administration suites, the console under its CSP

The headless browser test of the web console (brief M6-03, [19-security.md](../../docs/spec/19-security.md)
5 and 14, SEC-T05): the console of the real server image runs with its Content Security Policy
enforced in Chromium, Firefox and WebKit, and the test fails on any violation, on any request to an
origin other than the console origin and on a WebSocket the policy blocks (phase 1). Phase 2 (brief
M6-04 Design note 4) runs the `patrol` web suites of the console administration (A1 to A6) in
Chromium under the same enforced CSP against the same stack. `make console-e2e` runs both;
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
`infra/compose/test/compose.console-e2e-admin.yaml`.

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
   the hashes of `tools/uv.lock`); a test CA and a leaf certificate for `localhost`, `api` and
   127.0.0.1, valid for one day, created for this run only.
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

1. adds the overlay `compose.console-e2e-admin.yaml` to the files of the project and recreates `api`
   (`--no-deps --force-recreate --wait`); the overlay mounts the bundle read-only at
   `/usr/share/bothy/console-e2e` and sets two existing configuration keys:
   `BOTHY_CONSOLE__ASSETS_DIR` to that path (the server serves the bundle with the headers and the
   CSP of the console) and `BOTHY_AUTH__STEP_UP_MAX_AGE=3s`, so every changing action of a flow needs
   a fresh step-up through the console's step-up adapter;
2. recreates the TLS sidecar, which shares the network namespace of `api` and must follow the new
   container;
3. runs `console-browser` with `E2E_MODE=admin`; its exit code is the result of the phase.

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
5. the ids and names go to `window.__e2eSeed` as a JSON string (`owner`, `sam`, `pendingDevice` with
   `code`, `spaces`, `channels`, `agents`, `group`; every inner object has `id` and `name`), which
   the Dart tests read;
6. patrol's web protocol: initialise, list the tests with `__patrol__getTests` (the internal
   `patrol_test_explorer` is left out), run each with `__patrol__runTest` in the listed order, up to
   5 minutes each. Console lines starting with `PATROL_LOG` (patrol's step log) go to stderr. The
   platform handler answers `startTest`, `verifyFileDownloads`, `getClipboard`, `setClipboard`,
   `grantPermissions`, `enableDarkMode` and `disableDarkMode`; any other action fails the run with
   its name.

The phase fails on a failed or skipped test, on an empty test list, on an unsupported platform
action, on any CSP violation, console CSP error or foreign origin, and when no download ends with
`.csv` and has `occurred_at` in its first line (the export of A6). The result is
`out/results/admin.json`: per test its name, result and details (cut to 2000 characters), the
platform actions called, the downloads (file name and first line), violations, console CSP errors
and foreign origins. No secret, code or token is printed or written there.

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
it tests the bundle of the suites, which shares the code but not the entry point of the console
build in the image; and the seed creates its objects through the API, not through the screens.
