# tools/console-e2e: SEC-T05, the console under its CSP

The headless browser test of the web console (brief M6-03, [19-security.md](../../docs/spec/19-security.md)
5 and 14, SEC-T05): the console of the real server image runs with its Content Security Policy
enforced in Chromium, Firefox and WebKit, and the test fails on any violation, on any request to an
origin other than the console origin and on a WebSocket the policy blocks. `make console-e2e` runs it;
it is not part of `make gates`. The CI job `console-e2e` of `.github/workflows/ci.yml` runs it on
every change of the component `apps` or `server` and feeds `ci-ok`.

## Files

| File | Content |
|---|---|
| `console_e2e.py` | the runner on the host: preflight, temp directory, test CA, Compose stack, seed, sidecar, browsers, report check, teardown |
| `headless/console_headless.py` | the browser flow; runs inside the Playwright image (service `console-browser`) |
| `tests/test_console_e2e.py` | tests of the runner (teardown after a failure, preconditions, pins of the overlay, the CI job); part of `make test` |

The stack is the Compose smoke stack of M1-73 (`infra/compose/compose.yaml`,
`infra/compose/test/compose.smoke.yaml`, `smoke.env`) with the overlay
`infra/compose/test/compose.console-e2e.yaml` and `console-e2e.env`.

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
   PostgreSQL and the sidecar image.
4. `compose up --wait api` (secrets init, PostgreSQL, migrations).
5. `bothy setup bootstrap-owner --handle alex` in the running `api`; the enrollment code goes to
   `out/setup.out` (mode 0600, never printed).
6. Seed (`bothy-smoke seed`): the owner's device redeems the code, sets a random password, enrolls
   TOTP and confirms it; the login goes to `out/console-login.json` (mode 0600, never printed).
7. The TLS sidecar `tls` (Caddy) in the network namespace of `api` on port 8443.
8. The browsers (`console-browser`, also in the namespace of `api`) open `https://localhost:8443/console/`.
9. The server log must hold the report of the negative test (`csp violation reported` with
   `script-src-elem` and `inline`).
10. Teardown, always: `compose down --volumes --remove-orphans --rmi local`, the two images of the
    run, the temp directory; objects left with the project label fail the run.

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

## Variables

| Variable | Default | Meaning |
|---|---|---|
| `E2E_SERVER_IMAGE` | the tag of `make image` | server image to test |
| `E2E_BROWSERS` | `chromium,firefox,webkit` | browsers to run |
| `E2E_KEEP` | `0` | `1` keeps the stack and the temp directory (they hold the seed login; remove them) |
| `E2E_ARTIFACTS` | none | directory for the result files of a failed run |

Every exported `BOTHY_*`, `COMPOSE_*` and `OTEL_*` variable of the calling shell is removed before
Compose runs, so the `.env` of an installation is never read.

## What is not proven

Safari itself (WebKit in Playwright stands in for it), a browser on the host (the browsers run in the
Linux Playwright image locally as in CI), the passkey step-up in Firefox and WebKit, and the
terminal stream under the CSP (M6-18 runs this test with an open terminal).
