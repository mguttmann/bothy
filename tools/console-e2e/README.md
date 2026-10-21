# tools/console-e2e: SEC-T05, the administration suites and the terminal, the console under its CSP

The headless browser test of the web console (brief M6-03, [19-security.md](../../docs/spec/19-security.md)
5 and 14, SEC-T05): the console of the real server image runs with its Content Security Policy
enforced in Chromium, Firefox and WebKit, and the test fails on any violation, on any request to an
origin other than the console origin and on a WebSocket the policy blocks (phase 1). Phase 2 (brief
M6-04 Design note 4) runs the `patrol` web suites of the console administration (A1 to A6, A7
and A8 of brief M6-08: apps, outgoing and incoming webhooks, and A9 and A10 of brief M6-09:
instance settings, branding, feature flags and login providers) in Chromium under the same enforced
CSP against the same stack. Phase 3 (brief M6-18 Design notes 5 and 6) opens a terminal in the
console on a real host service container in Chromium, checks the console's terminal rules (ID-T28,
TM-T07, TM-T12, the `Origin` check of the stream, the pin store at logout, SEC-T05 with an open
terminal) and measures the terminal view. `make console-e2e` runs all three;
it is not part of `make gates`. The CI job `console-e2e` of `.github/workflows/ci.yml` runs it on
every change of the component `apps` or `server` and feeds `ci-ok`.

## Files

| File | Content |
|---|---|
| `console_e2e.py` | the runner on the host: preflight, temp directory, test CA, Compose stack, seed, sidecar, browsers, report check, teardown |
| `headless/console_headless.py` | the browser flow of phase 1; runs inside the Playwright image (service `console-browser`) and hands over to `admin_suites.py` with `E2E_MODE=admin` |
| `headless/admin_suites.py` | phase 2: login, seed through the API and patrol's web protocol in Chromium |
| `headless/terminal_suites.py` | phase 3: login, the seed of the host with the runner's pairing, the checks a to h and the patrol test of the terminal with its two synchronisation points |
| `host-bin.Dockerfile` | test only: builds `bothy-host` (with the mark of the platform suite) and the Fake-claude from `services/host` for the host container of phase 3 |
| `tests/test_console_e2e.py` | tests of the runner (teardown after a failure, preconditions, pins of the overlays, the CI job, phases 2 and 3, the Dockerfile targets) and of the pure helpers of `admin_suites.py` and `terminal_suites.py`; part of `make test` |

The stack is the Compose smoke stack of M1-73 (`infra/compose/compose.yaml`,
`infra/compose/test/compose.smoke.yaml`, `smoke.env`) with the overlay
`infra/compose/test/compose.console-e2e.yaml` and `console-e2e.env`; phase 2 adds
`infra/compose/test/compose.console-e2e-admin.yaml` and starts the worker and the webhook receiver,
phase 3 replaces the admin overlay with `infra/compose/test/compose.console-e2e-terminal.yaml`.

## Prerequisites

- Docker Engine 26 or newer and Docker Compose 2.26 or newer (as `make portability`).
- The server image: `make image` loads `bothy-server:<services/server/VERSION>-local`, the default;
  `E2E_SERVER_IMAGE` names another loaded image. The image carries the console build, so the test
  needs neither Flutter nor Go on the host.
- `uv` and the tools environment (`make bootstrap-ci`): the runner and the hashes of the browser
  packages come from `tools/uv.lock`.
- `TMPDIR` in a directory the Docker runtime shares with its VM (with colima: below the home
  directory), because the containers mount the temp directory of the run.
- For phase 3 the prerequisites of the terminal suite of M2-48 (`infra/compose/test/README.md`):
  `docker buildx`, a Docker host on cgroup version 2 that starts privileged containers, and
  `kernel.yama.ptrace_scope` of at least 1 (below it the host service refuses to start); the builds
  of the host image and of the host binaries fetch the Debian snapshot and the Go modules.

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
11. Phase 3, only after phase 2 passed (see below).
12. Teardown, always: `compose down --volumes --remove-orphans --rmi local` (with the host
    container and its network), the two images of the run, the host image when this run built it,
    the temp directory with the bundles and the host binaries; objects left with the project label
    fail the run.

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
   answers the file (name, MIME type and text) and touches no page: the binding runs inside the
   pending `page.evaluate` of the test, where a nested call of the sync API never returns. The
   suite (`chooseManifest` of `integration_test/admin/support.dart`) sets the file on the input the
   console appended (`input[data-bothy-file-input="manifest"]`) with a `DataTransfer` and fires its
   `change` event. The runner intercepts the page's file choosers with a listener that does
   nothing (`intercept_file_choosers`): otherwise headless Chromium answers the console's `click()`
   with an immediate `cancel` and the console removes its input. The file is the manifest of
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

The suites of brief M6-09 run after A8:

- A9 changes `locale.default` on the instance page with step-up and sees the new value after
  opening the console again; sets the accent `heather` and sees the console's theme switch to its
  accent roles once the console has read discovery again; switches the flag `dm.enabled` on with
  a reason and sees `getInstance` report it for the owner.
- A10 adds an OIDC provider "Example login" with the issuer `https://login.example.com` and a
  secret reference (adding reads nothing from the issuer, 08 2.6), logs the console out, starts it
  again and sees the button "Log in with Example login" on E2. It ends the session, so it is the
  last suite of the bundle.

The import suite of brief M8-08 (`import_test.dart`) runs after A9 and before A10. The seed
creates nothing for it: the flow adds a source from a white-label mapping (the YAML is built in
the flow and set on the page's file input, a wrong handle is shown at its place first), starts
it, posts a structure batch and the tool's counters through the API with the console session as
the import tool would, sees the counters and two mirrored channels, lets Bothy and then Discord
lead in a channel with the operator step each, sees completion held back until Bothy leads in
both channels and its retention warning, completes, and pauses and cancels a second source;
every step with the step-up.

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


## Phase 3: the terminal in the console

The stage `console-e2e-terminal-web` of the server Dockerfile holds the bundle of
`apps/console/integration_test/terminal/terminal_bundle.dart`, built like the bundle of phase 2;
it registers the one test `integration_test/terminal/terminal_test.dart`. The runner then:

1. builds the bundle into `terminal-bundle/` of the temp directory; the host image of the terminal
   suite of M2-48 (`termsuite-host:<first 12 hex of the SHA-256 of
   infra/compose/test/host/Dockerfile>`, the same tag as the suite, built with BuildKit only when
   it is missing and then removed at the teardown); and `bothy-host` and the Fake-claude from
   `services/host` with `host-bin.Dockerfile` (the Go image of the server Dockerfile, the builds of
   M2-48's `BuildHostBinaries`), exported to `host-bin/`;
2. replaces the admin overlay with `compose.console-e2e-terminal.yaml` and recreates `api`, then
   the TLS sidecar. The overlay serves the terminal bundle as the console (`BOTHY_CONSOLE__ASSETS_DIR`)
   and puts `api` on the internal network `consolehosts`; it sets no other key, so
   `terminal.quick_reattach_window` and `auth.step_up_max_age` keep their defaults;
3. starts the host container `console-host` (the host image, systemd as process 1, privileged and
   root like `termhost` of M2-48, only on `consolehosts`, no port), waits for systemd, copies the
   binaries in, adds the test CA of the run (mounted read-only) to the trust store and runs
   `bothy-host install --session-user bothy-s-alex`;
4. starts `console-browser` with `E2E_MODE=terminal` detached and waits for the pairing request of
   the seed (`out/terminal-pairing.json`, mode 0600). It removes the request, runs `bothy-host pair
   --server https://api:8443 --code <code>` in the host container (the code only as an argument of
   `docker exec`, as M2-48 passes it) and answers with the printed fingerprint in
   `out/terminal-paired.json`. The host service reaches the sidecar under the name `api`, which the
   certificate names, over TLS; it never talks plaintext to the server;
5. waits for the browser container, prints its output, removes it and checks
   `out/results/terminal.json`: every check passed; then it adds the desktop targets and the
   versions (see "The measurement").

In the browser (`terminal_suites.py`, Chromium only):

1. one context with the collectors of phase 1, the virtual authenticator (user verification on),
   patrol's binding and listeners for the status of every open, attach and close of the terminal;
   Alex logs in with password and TOTP, does the step-up with password and TOTP and registers a
   passkey, as in phase 2;
2. the seed as Alex: `createHost` "Build host" (assigned to Alex), the pairing request to the
   runner, `confirmHost` with the fingerprint prefix, `llm_credentials: none`,
   `declared_server_host: true` (the host cannot detect it behind the name `api`; the test does not
   assert `reported_is_server_host`) and the session user `bothy-s-alex` for Alex, then `getHost`
   until the host is active, connected and reports the session user and the profile `shell`. The
   document of `window.__e2eSeed` holds `owner` and `host` (`id` and `name`);
3. the checks, each recorded under its name; any failed check fails the phase:

| Check | What it proves |
|---|---|
| `a_open_with_uv_passkey` | the patrol test opens a terminal through the console's UI with a passkey step-up: the open answers 201, the stream connects and the shell's output arrives (the Noise handshake passed) |
| `b_password_totp_refused` | after a step-up with password and TOTP, `openTerminalSession` from the page answers `401 step_up_required` with `reason: passkey_required`, and no session exists afterwards (ID-T28, TM-T07) |
| `c_passkey_without_uv_refused` | with `isUserVerified` false (CDP `WebAuthn.setUserVerified`) the page asks for an assertion with `userVerification: discouraged`; the server refuses the step-up, or the following open answers `401 passkey_required`; which of the two is recorded; no session either way |
| `d_no_quick_reattach` | after the detach, within `terminal.quick_reattach_window`, an attach without a new passkey answers `401 passkey_required` (TM-T12); the reattach of the patrol test with a new passkey answers 200 and the scrollback is replayed (`replay_begin` seen or the marker text in the new terminal model) |
| `e_foreign_origin_upgrade` | an upgrade of the stream path with the console cookie and `Origin: https://localhost:9443` (same site, another origin, so the browser would send the cookie) answers 403 before the upgrade (a raw TLS request with the cookie of the context), and a page of that origin, answered by Playwright's route, never opens the stream |
| `f_logout_clears_pins` | an IndexedDB database of the pin store existed before the logout, and after the logout in the console none remains; the name is matched by the constant `PIN_STORE_HINT` (contains `pin`) |
| `g_csp_with_open_terminal` | with the terminal open, and over the whole run, no violation, no console CSP error and no foreign origin (the deliberate origin of e aside): SEC-T05 with an open terminal |
| `h_measurement` | the measurement below was made |

4. the patrol test drives the UI: the route "Hosts and terminal", the host, the open with the
   profile `shell`, the measurement, then `window.__e2ePhase = "open"`, where the runner records
   the collectors (g) and acknowledges with `window.__e2eAck`; a scrollback marker, the detach,
   then `detached`, where the runner checks d (first half), e and the pin store; then the reattach
   with a new passkey, the replay, a numbers line in the browser console
   (`E2E_TERMINAL_METRICS {...}`, no terminal content) and the logout.

The result `out/results/terminal.json` holds the checks with their details, the patrol result, the
measurement, the collectors and the console output of the page; a failed patrol test also leaves
`out/results/terminal-failure.png`. Nothing in it is a secret, a cookie, a code, a token or
terminal content.

### The measurement

Design note 6 and docs/spec/24-terminal.md 12: in the headless Chromium with the CSP enforced the
patrol test switches the frame policy to fully live and types two commands into the shell of the
host container. The first prints numbered markers with the host's clock in milliseconds, at short
intervals; per marker the test records the time from the write of its data frame into the terminal
model (the frame is decrypted at that point) to the end of the next Flutter frame
(`frame_to_paint_p95_ms`, the definition of `client.perf.terminal_frame_to_paint_p95`), and from the
host's clock value to that end (`host_to_paint_p95_ms`, the containers share the kernel clock). The
second writes a burst of a fixed size (`head -c`, `tr`, `fold`) as fast as the flow control lets it
and an end marker that carries the host time of its start; the throughput is its size divided by
the time from that start to the end of the frame that painted the end marker
(`throughput_bytes_per_s`). The image of the host has no output generator of its own and no `pv`,
so the burst is unthrottled; the flow control of the terminal is the limiter.

The runner records the values next to the desktop targets, read from the defaults of
`client.perf.terminal_frame_to_paint_p95` and `client.perf.terminal_throughput_min` in
`docs/operations/configuration.md` (`desktop_targets`, `within_desktop_targets`), and the versions
of `xterm` and `cryptography` from the workspace's `pubspec.lock` (`versions`). It prints them in one line
`console-e2e: phase 3: measurement {...}`, because the result file goes with the temp directory. The console has no
target of its own: the comparison is a measurement for 24 12, not a gate.

## Variables

| Variable | Default | Meaning |
|---|---|---|
| `E2E_SERVER_IMAGE` | the tag of `make image` | server image to test |
| `E2E_BROWSERS` | `chromium,firefox,webkit` | browsers to run |
| `E2E_KEEP` | `0` | `1` keeps the stack and the temp directory (they hold the seed login; remove them) |
| `E2E_ARTIFACTS` | none | directory for the result files of a failed run |

`E2E_MODE`, `E2E_ADMIN_BUNDLE`, `E2E_TERMINAL_BUNDLE` and `E2E_HOST_IMAGE` are set by the runner
for phases 2 and 3; an exported `E2E_MODE` or `E2E_HOST_IMAGE` of the calling shell is ignored.

Every exported `BOTHY_*`, `COMPOSE_*` and `OTEL_*` variable of the calling shell is removed before
Compose runs, so the `.env` of an installation is never read.

## What is not proven

Safari itself (WebKit in Playwright stands in for it), a browser on the host (the browsers run in the
Linux Playwright image locally as in CI) and the passkey step-up in Firefox and WebKit.

Phase 2 runs only in Chromium, so the administration screens are not proven in Firefox or WebKit;
the manifest file reaches the console through Playwright's file input API, not through a file
dialog a person uses; the webhook receiver checks no signature (the server's signing is proven by
the tests of M2-26);
it tests the bundle of the suites, which shares the code but not the entry point of the console
build in the image; and the seed creates its objects through the API, not through the screens.

Phase 3 runs only in Chromium: Firefox and WebKit have no virtual authenticator in Playwright, so
the passkey rules of the terminal (with and without user verification) are not proven there. It
does not prove protection against malicious browser extensions or script that runs in the
console's origin despite the CSP: such code acts with the person's session, may ask for the
passkey and reads the terminal; this is the residual risk T-TM-07 of the threat model, which the
CSP, Trusted Types and the passkey with user verification for every open and attach reduce but do
not remove. Pinning in the browser only notices a key change the server reports (24 5.1); the test
does not and cannot show protection against the server itself. The measurement is one run on one
machine and no statement about other browsers or hardware; the open advisories of `xterm` and
`cryptography` are not queried here (no network lookup in the test). The host is declared as the
server host instead of detecting it, and the raw upgrade request of e stands in for a browser's
own handshake, whose status the page cannot read.
