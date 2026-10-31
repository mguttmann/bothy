# SPDX-License-Identifier: Apache-2.0
"""Phase 3 of the console E2E test: the terminal in the browser (brief M6-18 Design notes 5, 6).

Runs inside the pinned Playwright image (service ``console-browser`` with ``E2E_MODE=terminal``)
against the api of the overlay ``compose.console-e2e-terminal.yaml``, which serves the bundle of
``apps/console/integration_test/terminal/terminal_bundle.dart`` as the console. Chromium only:
only Chromium has the virtual authenticator, here with ``isUserVerified`` true or false per case.

1. One browser context with the collectors of SEC-T05 (``console_headless.Session``), the virtual
   authenticator and patrol's binding; Alex logs in with password and TOTP, does the step-up with
   password and TOTP and registers a passkey (``admin_suites.log_in``).
2. The seed through the API as Alex: the host "Build host" (createHost), whose one-time pairing
   code goes to the runner in ``/out/terminal-pairing.json`` (mode 0600); the runner pairs the host
   service container and answers with the fingerprint ``bothy-host pair`` printed; then
   confirmHost declares the host as the server host and gives Alex the session user
   ``bothy-s-alex``, and the seed waits until the host is active with its report. The ids go to
   ``window.__e2eSeed`` (``owner`` and ``host``).
3. The checks of the brief, each recorded under its name in CHECKS; any failed check fails the
   phase:

   a. the open with a passkey step-up with user verification answers 201 and the stream works
      (the patrol test sees the output of the shell);
   b. a step-up with password and TOTP, then openTerminalSession: 401 ``step_up_required`` with
      ``reason: passkey_required`` and no session;
   c. a passkey step-up while the authenticator does no user verification: the step-up is refused
      or the following open gives 401 ``passkey_required``; either way no session (recorded);
   d. after the detach an attach without a new passkey gives 401 ``passkey_required`` (no quick
      reattach in the console, 24 6.2); with a new passkey the attach answers 200 and the
      scrollback is replayed;
   e. a WebSocket upgrade of the stream path with the console cookie and the ``Origin`` of another
      origin (FOREIGN_ORIGIN) is answered 403 before the upgrade, and a page of that origin never
      opens the stream;
   f. after the logout in the console no IndexedDB database of the pin store remains (and one
      existed before);
   g. with the terminal open the collectors of SEC-T05 hold no violation, no console CSP error and
      no foreign origin (the deliberate one of e aside);
   h. the measurement of Design note 6: frame-to-paint p95 and throughput of the terminal view.

The patrol test drives the console's own UI (open, measure, detach, reattach, log out) and stops at
two synchronisation points (``window.__e2ePhase`` ``open`` and ``detached``), where this module
records g, d and e and then acknowledges (``window.__e2eAck``). The result goes to
``/out/results/terminal.json`` (the checks, the patrol result, the measurement and the collectors;
never a secret, a cookie, a pairing code, a token or terminal content). Exit code 0 when every
check and the patrol test passed.
"""

from __future__ import annotations

import base64
import hashlib
import json
import math
import os
import re
import secrets
import socket
import ssl
import sys
import time
import traceback
from collections.abc import Callable, Mapping, Sequence
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

from admin_suites import (
    API,
    ConsoleAdmin,
    PlatformHandler,
    b64url,
    console_log,
    flatten_tests,
    log_in,
    open_bundle,
    problem_code,
    problem_text,
    uuid7,
)
from console_headless import (
    CONSOLE,
    ORIGIN,
    OUT,
    RESULTS,
    CheckError,
    Login,
    Session,
    console_fetch,
    dump_screen,
    launch,
)
from playwright.sync_api import (
    CDPSession,
    ConsoleMessage,
    Page,
    Playwright,
    Response,
    Route,
    sync_playwright,
)

RESULT_FILE = "terminal.json"
# The hand-over with the runner (console_e2e.py PAIRING_FILE and PAIRED_FILE; a test holds both
# equal).
PAIRING_FILE = "terminal-pairing.json"
PAIRED_FILE = "terminal-paired.json"
CHECKS = (
    "a_open_with_uv_passkey",
    "b_password_totp_refused",
    "c_passkey_without_uv_refused",
    "d_no_quick_reattach",
    "e_foreign_origin_upgrade",
    "f_logout_clears_pins",
    "g_csp_with_open_terminal",
    "h_measurement",
)
HOST_NAME = "Build host"
SESSION_USER = "bothy-s-alex"
PROFILE = "shell"
# The IndexedDB database of the console's pin store (apps/console/lib/terminal,
# IndexedDbHostKeyPinStore) has a name that contains this text, compared without regard to case.
PIN_STORE_HINT = "pin"
# The second test origin of e: same site as the console (so the browser sends the cookie on the
# upgrade, as in a cross-site WebSocket hijacking from a sibling origin), another origin. Playwright
# answers its page; nothing listens there.
FOREIGN_ORIGIN = "https://localhost:9443"
# The test CA of the run, mounted by the terminal overlay, for the raw upgrade of e.
CA_FILE = "/e2e-ca/ca.pem"
# The line the patrol test prints with its numbers (never terminal content).
METRICS_PREFIX = "E2E_TERMINAL_METRICS "
# The default of terminal.fingerprint_confirm_chars (24 17); the stack leaves the key unset.
FINGERPRINT_CONFIRM_CHARS = 8
# The share of the frame-to-paint markers that must have been measured.
MIN_SAMPLE_SHARE = 0.9
# Bounds of the waits: the pairing by the runner, the host's first report, the patrol test (it
# measures and waits for the passkey prompts) and the removal of the pin store after the logout.
PAIRING_WAIT_S = 600
HOST_READY_WAIT_S = 180
TERMINAL_TEST_TIMEOUT_MS = 600_000
PIN_REMOVAL_WAIT_S = 15
CONFIRM_RETRIES = 5
POLL_S = 1.0
DETAILS_MAX = 2000


# ---- pure helpers (unit-tested in tools/console-e2e/tests) ---------------------------------


def p95(values: Sequence[float]) -> float:
    """The 95th percentile of ``values`` by the nearest rank; CheckError when there is none."""
    if not values:
        raise CheckError("no samples for a percentile")
    ordered = sorted(values)
    return float(ordered[math.ceil(0.95 * len(ordered)) - 1])


def measurement(metrics: Mapping[str, Any]) -> dict[str, Any]:
    """The numbers of Design note 6 from the report of the patrol test.

    ``write_to_paint_ms`` are the times from the write of a decrypted data frame into the terminal
    model to the end of the next frame (the definition of
    ``client.perf.terminal_frame_to_paint_p95``), ``host_to_paint_ms`` the same from the clock
    value the host wrote into the marker (shared kernel clock of the containers);
    ``throughput`` gives the bytes of the burst, the host time of its start and the end of the
    frame that painted its end marker. Raises CheckError when fewer than MIN_SAMPLE_SHARE of the
    markers were measured or the burst has no positive duration.
    """
    expected = int(metrics.get("marks_expected") or 0)
    writes = [float(v) for v in metrics.get("write_to_paint_ms") or []]
    hosts = [float(v) for v in metrics.get("host_to_paint_ms") or []]
    if expected <= 0 or min(len(writes), len(hosts)) < MIN_SAMPLE_SHARE * expected:
        raise CheckError(f"{len(writes)} of {expected} frame-to-paint markers were measured")
    burst = metrics.get("throughput")
    if not isinstance(burst, Mapping):
        raise CheckError("the patrol test measured no throughput")
    span_ms = float(burst["paint_end_ms"]) - float(burst["host_start_ms"])
    if span_ms <= 0:
        raise CheckError("the burst of the throughput measurement has no duration")
    size = int(burst["bytes"])
    return {
        "samples": len(writes),
        "marks_expected": expected,
        "frame_to_paint_p95_ms": p95(writes),
        "host_to_paint_p95_ms": p95(hosts),
        "throughput_bytes": size,
        "throughput_seconds": span_ms / 1000,
        "throughput_bytes_per_s": size / (span_ms / 1000),
    }


def parse_metrics_line(text: str) -> dict[str, Any] | None:
    """The object of a METRICS_PREFIX line of the browser console, or None for any other line."""
    if not text.startswith(METRICS_PREFIX):
        return None
    try:
        parsed = json.loads(text[len(METRICS_PREFIX) :])
    except ValueError:
        return None
    return parsed if isinstance(parsed, dict) else None


def expect_passkey_required(answer: Mapping[str, Any], op: str) -> None:
    """Raises CheckError naming ``op`` unless ``answer`` is a passkey_required refusal.

    The refusal is 401 ``step_up_required`` with ``reason: passkey_required`` (08 6.3 item 3).
    """
    body = answer.get("json")
    reason = str(body.get("reason") or "") if isinstance(body, Mapping) else ""
    if (
        answer.get("status") != 401
        or problem_code(answer) != "step_up_required"
        or reason != "passkey_required"
    ):
        raise CheckError(f"{op} answered {answer.get('status')} {problem_text(answer)}")


def check_terminal_seed(seed: Mapping[str, Any]) -> None:
    """Checks the shape of ``window.__e2eSeed`` the patrol test reads.

    Exactly ``owner`` and ``host``, each with a string ``id`` and ``name``, the host named
    HOST_NAME.
    """
    if set(seed) != {"owner", "host"}:
        raise CheckError(f"seed members {sorted(seed)} differ from ['host', 'owner']")
    for key in ("owner", "host"):
        obj = seed[key]
        if not isinstance(obj, Mapping) or not all(
            isinstance(obj.get(k), str) and obj[k] for k in ("id", "name")
        ):
            raise CheckError(f"seed member {key} needs a string id and name")
    if seed["host"]["name"] != HOST_NAME:
        raise CheckError(f"the host of the seed must be named {HOST_NAME}")


def pin_databases(databases: Sequence[Mapping[str, Any]]) -> list[str]:
    """The names of ``indexedDB.databases()`` that belong to the pin store (PIN_STORE_HINT)."""
    return [
        str(d["name"])
        for d in databases
        if isinstance(d.get("name"), str) and PIN_STORE_HINT in str(d["name"]).lower()
    ]


def upgrade_request(path: str, *, host: str, origin: str, cookie: str, key: str) -> bytes:
    """The HTTP/1.1 request of a WebSocket upgrade (RFC 6455 4.1) of ``path``.

    It carries ``Origin`` and ``Cookie``. Raises ValueError when a value holds a line break (no
    header injection).
    """
    for value in (path, host, origin, cookie, key):
        if "\r" in value or "\n" in value:
            raise ValueError("a header value holds a line break")
    lines = [
        f"GET {path} HTTP/1.1",
        f"Host: {host}",
        "Upgrade: websocket",
        "Connection: Upgrade",
        f"Sec-WebSocket-Key: {key}",
        "Sec-WebSocket-Version: 13",
        f"Origin: {origin}",
        f"Cookie: {cookie}",
    ]
    return ("\r\n".join(lines) + "\r\n\r\n").encode()


def status_code(head: bytes) -> int:
    """The status code of an HTTP/1.1 response head, or 0 when it is none."""
    first = head.split(b"\r\n", 1)[0].split(b" ")
    if len(first) < 2 or not first[0].startswith(b"HTTP/") or not first[1].isdigit():
        return 0
    return int(first[1])


def cookie_header(cookies: Sequence[Mapping[str, Any]], host: str) -> str:
    """The ``Cookie`` value of the cookies of the console host (the session cookie of the console).

    Kept in memory only.
    """
    return "; ".join(
        f"{c['name']}={c['value']}"
        for c in cookies
        if str(c.get("domain") or "").lstrip(".") == host
    )


def fingerprint_prefix(fingerprint: str) -> str:
    """The first FINGERPRINT_CONFIRM_CHARS characters of the fingerprint without separators.

    In the form confirmHost takes them.
    """
    plain = fingerprint.replace("-", "")
    if len(plain) < FINGERPRINT_CONFIRM_CHARS:
        raise CheckError("the fingerprint of bothy-host pair is too short")
    return plain[:FINGERPRINT_CONFIRM_CHARS]


def host_ready(host: Mapping[str, Any]) -> bool:
    """Whether getHost shows the host active and connected, with its user and profile.

    The host must report SESSION_USER and offer the profile PROFILE.
    """
    reported = {str(r.get("os_user")) for r in host.get("reported_session_users") or []}
    profiles = {str(p.get("name")) for p in host.get("profiles") or []}
    return (
        host.get("state") == "active"
        and host.get("connected") is True
        and SESSION_USER in reported
        and PROFILE in profiles
    )


_OPEN = re.compile(r"/api/v1/hosts/[^/]+/terminal/sessions")
_SESSION_OP = re.compile(r"/api/v1/terminal/sessions/[^/]+/(attach|close)")


def terminal_op(method: str, url: str) -> str | None:
    """``open``, ``attach`` or ``close`` for a request of those operations, else None."""
    if method != "POST":
        return None
    path = urlsplit(url).path
    if _OPEN.fullmatch(path):
        return "open"
    found = _SESSION_OP.fullmatch(path)
    return found.group(1) if found else None


def session_ids(page_json: object, host_id: str) -> list[str]:
    """The ids of the sessions on ``host_id`` in a page of listTerminalSessions."""
    if not isinstance(page_json, Mapping):
        return []
    return [str(s["id"]) for s in page_json.get("items") or [] if s.get("host_id") == host_id]


def key_hash() -> str:
    """A ``client_ephemeral_key_hash`` of a random key.

    Base64url SHA-256, as the contract wants; the requests of b, c and d never get far enough to use
    it.
    """
    return b64url(hashlib.sha256(secrets.token_bytes(32)).digest())


def foreign_except(origins: Sequence[str]) -> list[str]:
    """``origins`` without FOREIGN_ORIGIN, which check e opens on purpose."""
    return [o for o in origins if o != FOREIGN_ORIGIN]


def failed_checks(checks: Mapping[str, Any]) -> list[str]:
    """The names of CHECKS that are missing or did not pass."""
    return [n for n in CHECKS if not (checks.get(n) or {}).get("ok")]


# ---- page scripts ------------------------------------------------------------------------

# navigator.credentials.get over the passkey options of a step-up challenge with the
# userVerification of the caller (c asks with "discouraged", so the authenticator without user
# verification still answers and the server decides); an error of the browser comes back as data.
GET_PASSKEY_UV_JS = """async ([options, uv]) => {
  const b64u = (s) => Uint8Array.from(
    atob(s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4)),
    (c) => c.charCodeAt(0));
  const enc = (buf) => btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\\+/g, '-').replace(/\\//g, '_').replace(/=+$/, '');
  const pk = Object.assign({}, options.publicKey || options);
  pk.challenge = b64u(pk.challenge);
  pk.allowCredentials = (pk.allowCredentials || []).map((c) => Object.assign({}, c,
    {id: b64u(c.id)}));
  pk.userVerification = uv;
  let cred;
  try { cred = await navigator.credentials.get({publicKey: pk}); }
  catch (e) { return {error: String((e && e.name) || e)}; }
  const response = {clientDataJSON: enc(cred.response.clientDataJSON),
    authenticatorData: enc(cred.response.authenticatorData),
    signature: enc(cred.response.signature)};
  if (cred.response.userHandle && cred.response.userHandle.byteLength > 0) {
    response.userHandle = enc(cred.response.userHandle);
  }
  return {credential: {id: cred.id, rawId: enc(cred.rawId), type: cred.type, response,
    clientExtensionResults: cred.getClientExtensionResults()}};
}"""

# Starts one patrol test without waiting for it; its outcome lands in window.__e2eRunDone, so this
# module can serve the synchronisation points while the test runs.
START_TEST_JS = """([name, ms]) => {
  window.__e2eRunDone = undefined;
  window.__e2ePhase = undefined;
  window.__e2eAck = undefined;
  Promise.race([
    window.__patrol__runTest(name).catch((e) => ({result: 'failure',
      details: 'runTest rejected: ' + String((e && e.error) || e)})),
    new Promise((resolve) => setTimeout(
      () => resolve({result: 'failure', details: 'timeout after ' + ms + ' ms'}), ms)),
  ]).then((r) => { window.__e2eRunDone = r; });
  return true;
}"""

PHASE_JS = "(phase) => window.__e2ePhase === phase || window.__e2eRunDone !== undefined"

# A WebSocket of a page of FOREIGN_ORIGIN to the stream path of the console origin; whether it
# ever opened and its close code.
FOREIGN_WS_JS = """(url) => new Promise((resolve) => {
  let opened = false;
  let ws;
  try { ws = new WebSocket(url); } catch (e) { resolve({opened, thrown: String(e)}); return; }
  const t = setTimeout(() => { try { ws.close(); } catch (e) {} resolve({opened, timeout: true}); },
    15000);
  ws.onopen = () => { opened = true; ws.close(); };
  ws.onclose = (e) => { clearTimeout(t); resolve({opened, code: e.code}); };
})"""

DATABASES_JS = """async () => (await indexedDB.databases())
  .map((d) => ({name: d.name, version: d.version}))"""


# ---- the run ------------------------------------------------------------------------------


def say(message: str) -> None:
    """One progress line of the terminal suites on stderr."""
    print(f"console-headless: terminal: {message}", file=sys.stderr, flush=True)


class Recorder:
    """The page's listeners.

    The status of every open, attach and close of the terminal (no body), the numbers line of the
    patrol test, and the other console lines for the result.
    """

    def __init__(self, lines: list[str]) -> None:
        """Collects into ``lines``, the console lines of the result."""
        self.calls: list[dict[str, Any]] = []
        self.metrics: dict[str, Any] | None = None
        self._log = console_log(lines)

    def on_response(self, response: Response) -> None:
        """Records the operation and status of a terminal call."""
        op = terminal_op(response.request.method, response.url)
        if op is not None:
            self.calls.append({"op": op, "status": response.status})

    def on_console(self, msg: ConsoleMessage) -> None:
        """Keeps the numbers of the patrol test; every other line goes to console_log."""
        parsed = parse_metrics_line(msg.text)
        if parsed is not None:
            self.metrics = parsed
            return
        self._log(msg)


def add_authenticator(page: Page) -> tuple[CDPSession, str]:
    """A platform authenticator with resident keys and user verification (Chromium, CDP).

    Returns the CDP session and the authenticator id, so c can switch the verification off.
    """
    cdp = page.context.new_cdp_session(page)
    cdp.send("WebAuthn.enable")
    options = {
        "protocol": "ctap2",
        "transport": "internal",
        "hasResidentKey": True,
        "hasUserVerification": True,
        "isUserVerified": True,
        "automaticPresenceSimulation": True,
    }
    added = cdp.send("WebAuthn.addVirtualAuthenticator", {"options": options})
    return cdp, str(added["authenticatorId"])


def run_check(checks: dict[str, Any], name: str, step: Callable[[], dict[str, Any]]) -> None:
    """Runs one check and records its outcome.

    A failure is recorded, not raised, so the later checks still run and report.
    """
    try:
        checks[name] = {"ok": True, **step()}
    except Exception as e:
        checks[name] = {"ok": False, "error": f"{type(e).__name__}: {e}"[:DETAILS_MAX]}
        say(f"{name}: FAIL {checks[name]['error']}")
    else:
        say(f"{name}: ok")


def write_request(path: Path, doc: Mapping[str, str]) -> None:
    """Writes the pairing request with mode 0600 through a temporary file.

    The runner polls for the final name, so it never reads half of it.
    """
    part = path.with_suffix(".part")
    fd = os.open(part, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump(dict(doc), f)
    part.replace(path)


class TerminalRun:
    """The state of phase 3 in one browser context.

    The page, Alex's console session, the authenticator, the listeners and the checks with their
    parts.
    """

    def __init__(self, session: Session, login: Login, recorder: Recorder) -> None:
        """Binds the checks to the browser session, its login and the recorder."""
        self.session = session
        self.page = session.page
        self.login = login
        self.recorder = recorder
        self.admin = ConsoleAdmin(session.page)
        self.cdp, self.authenticator = add_authenticator(session.page)
        self.checks: dict[str, Any] = {}
        self.host_id = ""
        self.session_id = ""
        self.open_snapshot: dict[str, Any] | None = None
        self.pins_before: list[str] = []
        self.reattach_mark = -1
        self.patrol_mark = 0

    # -- seed -----------------------------------------------------------------------------

    def seed(self) -> dict[str, Any]:
        """createHost, the pairing by the runner, confirmHost and the wait for the report.

        Returns the document of window.__e2eSeed.
        """
        me = self.admin.call("getConsoleSession", "GET", "/console/session")["json"]
        owner = {"id": str(me["user_id"]), "name": str(me["display_name"])}
        created = self.admin.call(
            "createHost",
            "POST",
            "/hosts",
            {"name": HOST_NAME, "assigned_user_id": owner["id"], "reason": "Terminal test"},
            idempotent=True,
        )["json"]
        self.host_id = str(created["host_id"])
        say("seed: the runner pairs the host service")
        write_request(
            OUT / PAIRING_FILE,
            {"host_id": self.host_id, "pairing_code": str(created["pairing_code"])},
        )
        self.confirm(self.wait_paired(), owner["id"])
        self.wait_ready()
        return {"owner": owner, "host": {"id": self.host_id, "name": HOST_NAME}}

    def wait_paired(self) -> str:
        """The fingerprint the runner wrote after bothy-host pair."""
        answer = OUT / PAIRED_FILE
        deadline = time.monotonic() + PAIRING_WAIT_S
        while not answer.exists():
            if time.monotonic() > deadline:
                raise CheckError("the runner did not pair the host")
            time.sleep(POLL_S)
        fingerprint = str(json.loads(answer.read_text(encoding="utf-8"))["fingerprint"])
        answer.unlink()
        return fingerprint

    def confirm(self, fingerprint: str, owner_id: str) -> None:
        """Calls confirmHost.

        The fingerprint prefix, no LLM login, the declaration as the server host (the host cannot
        detect it behind the name api) and Alex's session user. A report of the host raises the
        version meanwhile, so a 412 reads the host again.
        """
        body = {
            "fingerprint_prefix": fingerprint_prefix(fingerprint),
            "llm_credentials": "none",
            "declared_server_host": True,
            "session_users": [{"user_id": owner_id, "os_user": SESSION_USER}],
            "reason": "Terminal test",
        }
        path = f"/hosts/{self.host_id}/confirm"
        for attempt in range(CONFIRM_RETRIES + 1):
            headers = {
                "Idempotency-Key": uuid7(),
                "If-Match": self.admin.etag("getHost", f"/hosts/{self.host_id}"),
            }
            answer = self.admin.fetch("POST", path, body, headers)
            if answer["status"] == 401 and problem_code(answer) == "step_up_required":
                self.admin.step_up_with_passkey()
                answer = self.admin.fetch("POST", path, body, headers)
            if answer["status"] == 412 and attempt < CONFIRM_RETRIES:
                continue
            if answer["status"] != 200:
                raise CheckError(f"confirmHost answered {answer['status']} {problem_text(answer)}")
            return

    def wait_ready(self) -> None:
        """Polls getHost until host_ready."""
        deadline = time.monotonic() + HOST_READY_WAIT_S
        while True:
            host = self.admin.call("getHost", "GET", f"/hosts/{self.host_id}")["json"]
            if isinstance(host, dict) and host_ready(host):
                return
            if time.monotonic() > deadline:
                state = host.get("state") if isinstance(host, dict) else None
                raise CheckError(f"the host did not become ready (state {state})")
            time.sleep(POLL_S)

    # -- requests of the checks ------------------------------------------------------------

    def sessions(self, state: str = "") -> list[str]:
        """The ids of the sessions on the host (listTerminalSessions), optionally in ``state``."""
        query = f"?host_id={self.host_id}&limit=100" + (f"&state={state}" if state else "")
        page = self.admin.call("listTerminalSessions", "GET", "/terminal/sessions" + query)
        return session_ids(page["json"], self.host_id)

    def open_session(self) -> dict[str, Any]:
        """Calls openTerminalSession for PROFILE from the page (cookie, CSRF header)."""
        body = {"profile": PROFILE, "cols": 80, "rows": 24, "client_ephemeral_key_hash": key_hash()}
        return console_fetch(
            self.page, "POST", f"{API}/hosts/{self.host_id}/terminal/sessions", body
        )

    def challenge(self) -> dict[str, Any]:
        """createStepUpChallenge; CheckError unless it answers 200 with an object."""
        answer = self.admin.fetch("POST", "/auth/step-up/challenge")
        if answer["status"] != 200 or not isinstance(answer["json"], dict):
            raise CheckError(f"createStepUpChallenge answered {answer['status']}")
        data: dict[str, Any] = answer["json"]
        return data

    # -- the checks ------------------------------------------------------------------------

    def check_password_totp(self) -> dict[str, Any]:
        """b: a step-up with password and TOTP does not open a terminal from the console."""
        before = self.sessions()
        code = self.login.code()
        challenge = self.challenge()
        step = self.admin.fetch(
            "POST",
            "/auth/step-up/password",
            {
                "challenge_id": challenge["challenge_id"],
                "password": self.login.password,
                "totp_code": code,
            },
        )
        if step["status"] != 200:
            raise CheckError(f"stepUpWithPassword answered {step['status']}")
        expect_passkey_required(self.open_session(), "openTerminalSession after password+TOTP")
        if self.sessions() != before:
            raise CheckError("the refused open created a session")
        return {"status": 401, "reason": "passkey_required"}

    def check_passkey_without_uv(self) -> dict[str, Any]:
        """c: a passkey assertion without user verification; records where it was refused."""
        before = self.sessions()
        self.set_user_verified(False)
        try:
            challenge = self.challenge()
            if not challenge.get("passkey_options"):
                raise CheckError("the step-up challenge offers no passkey")
            got = self.page.evaluate(
                GET_PASSKEY_UV_JS, [challenge["passkey_options"], "discouraged"]
            )
            if "error" in got:
                raise CheckError(f"the browser gave no assertion without UV: {got['error']}")
            done = self.admin.fetch(
                "POST",
                "/auth/step-up/passkey",
                {"challenge_id": challenge["challenge_id"], "credential": got["credential"]},
            )
            if done["status"] == 200:
                expect_passkey_required(self.open_session(), "openTerminalSession without UV")
                outcome = {"refused_by": "open", "step_up_status": 200, "open_status": 401}
            elif 400 <= done["status"] < 500:
                outcome = {
                    "refused_by": "step_up",
                    "step_up_status": done["status"],
                    "problem": problem_text(done),
                }
            else:
                raise CheckError(f"completeStepUpWithPasskey answered {done['status']}")
        finally:
            self.set_user_verified(True)
        if self.sessions() != before:
            raise CheckError("the refused open created a session")
        return outcome

    def set_user_verified(self, verified: bool) -> None:
        """Switches the user verification of the virtual authenticator."""
        self.cdp.send(
            "WebAuthn.setUserVerified",
            {"authenticatorId": self.authenticator, "isUserVerified": verified},
        )

    def at_open(self) -> None:
        """Synchronisation point ``open``: the collectors with the terminal open (g)."""
        self.open_snapshot = {
            "violations": self.session.violations(),
            "console_errors": list(self.session.console_errors),
            "foreign_origins": self.session.foreign(),
        }

    def at_detached(self) -> None:
        """Synchronisation point ``detached``.

        The attach without a new passkey (d), the upgrade from the foreign origin (e) and the pin
        store before the logout (f).
        """
        run_check(self.checks, "d_no_quick_reattach", self.no_quick_reattach)
        run_check(self.checks, "e_foreign_origin_upgrade", self.foreign_origin_upgrade)
        self.pins_before = pin_databases(self.page.evaluate(DATABASES_JS))
        self.reattach_mark = len(self.recorder.calls)

    def no_quick_reattach(self) -> dict[str, Any]:
        """d, first half: within terminal.quick_reattach_window an attach without a passkey."""
        detached = self.sessions("detached")
        if len(detached) != 1:
            raise CheckError(f"{len(detached)} detached sessions on the host, not one")
        self.session_id = detached[0]
        body = {"cols": 80, "rows": 24, "client_ephemeral_key_hash": key_hash()}
        answer = console_fetch(
            self.page, "POST", f"{API}/terminal/sessions/{self.session_id}/attach", body
        )
        expect_passkey_required(answer, "attachTerminalSession without a new passkey")
        return {"without_passkey": 401}

    def foreign_origin_upgrade(self) -> dict[str, Any]:
        """e: the upgrade with the console cookie and a foreign Origin."""
        if not self.session_id:
            raise CheckError("no session to aim at (d found none)")
        path = f"{API}/terminal/sessions/{self.session_id}/stream"
        host = urlsplit(ORIGIN).hostname or "localhost"
        cookie = cookie_header(self.session.context.cookies(ORIGIN), host)
        if not cookie:
            raise CheckError("the context holds no cookie of the console")
        status = upgrade_status(path, cookie)
        page = self.session.context.new_page()
        try:
            page.route(FOREIGN_ORIGIN + "/**", serve_foreign)
            page.goto(FOREIGN_ORIGIN + "/")
            attempt = page.evaluate(FOREIGN_WS_JS, ORIGIN.replace("https", "wss", 1) + path)
        finally:
            page.close()
        if status != 403:
            raise CheckError(f"the upgrade with a foreign Origin answered {status}, not 403")
        if attempt.get("opened"):
            raise CheckError("a page of the foreign origin opened the stream")
        return {"status": 403, "page_opened": False, "page_close_code": attempt.get("code")}

    # -- after the patrol test -------------------------------------------------------------

    def finish(self, outcome: Mapping[str, Any]) -> None:
        """Records a, d (second half), f, g and h from the listeners and the patrol report."""
        metrics = self.recorder.metrics or {}
        calls = self.recorder.calls
        passed = outcome.get("result") == "success"
        run_check(self.checks, "a_open_with_uv_passkey", lambda: self.opened(calls, metrics))
        if self.checks.get("d_no_quick_reattach", {}).get("ok"):
            run_check(self.checks, "d_no_quick_reattach", lambda: self.reattached(calls, metrics))
        run_check(self.checks, "f_logout_clears_pins", lambda: self.pins_gone(passed))
        run_check(self.checks, "g_csp_with_open_terminal", self.clean)
        run_check(self.checks, "h_measurement", lambda: measurement(metrics))

    def opened(
        self, calls: Sequence[Mapping[str, Any]], metrics: Mapping[str, Any]
    ) -> dict[str, Any]:
        """a: the open of the patrol test answered 201 and the stream carried the shell."""
        opens = [c["status"] for c in calls[self.patrol_mark :] if c["op"] == "open"]
        if 201 not in opens:
            raise CheckError(f"the open of the console answered {opens}, not 201")
        if not any(url.endswith("/stream") for url in self.session.websockets):
            raise CheckError("the console never connected the stream")
        if metrics.get("opened") is not True:
            raise CheckError("the patrol test saw no output of the shell")
        return {"open_status": 201}

    def reattached(
        self, calls: Sequence[Mapping[str, Any]], metrics: Mapping[str, Any]
    ) -> dict[str, Any]:
        """d, second half: the attach with a new passkey answered 200 and replayed."""
        attaches = [c["status"] for c in calls[self.reattach_mark :] if c["op"] == "attach"]
        if 200 not in attaches:
            raise CheckError(f"the attach with a new passkey answered {attaches}, not 200")
        frames = metrics.get("replay_frames") is True
        seen = metrics.get("text_seen_again") is True
        if not (frames or seen):
            raise CheckError("the scrollback was not replayed")
        return {
            "without_passkey": 401,
            "with_passkey": 200,
            "replay_frames": frames,
            "text_seen_again": seen,
        }

    def pins_gone(self, passed: bool) -> dict[str, Any]:
        """f: a pin store existed before the logout and none remains after it."""
        if not self.pins_before:
            raise CheckError("no IndexedDB database of the pin store existed before the logout")
        if not passed:
            raise CheckError("the patrol test did not reach the logout")
        deadline = time.monotonic() + PIN_REMOVAL_WAIT_S
        while True:
            left = pin_databases(self.page.evaluate(DATABASES_JS))
            if not left:
                return {"before_logout": self.pins_before, "after_logout": []}
            if time.monotonic() > deadline:
                raise CheckError(f"the pin store survived the logout: {left}")
            time.sleep(POLL_S)

    def clean(self) -> dict[str, Any]:
        """g: the collectors at the synchronisation point and over the whole run."""
        snapshot = self.open_snapshot
        if snapshot is None:
            raise CheckError("the patrol test never reached an open terminal")
        if any(snapshot.values()):
            raise CheckError(f"with the terminal open: {snapshot}")
        final = {
            "violations": self.session.violations(),
            "console_errors": list(self.session.console_errors),
            "foreign_origins": foreign_except(self.session.foreign()),
        }
        if any(final.values()):
            raise CheckError(f"during the run: {final}")
        return {"violations": 0, "console_errors": 0, "foreign_origins": 0}


def serve_foreign(route: Route) -> None:
    """Playwright's answer for the page of FOREIGN_ORIGIN: an empty document."""
    route.fulfill(status=200, content_type="text/html", body="<!doctype html><title>e2e</title>")


def upgrade_tls_context(cafile: str) -> ssl.SSLContext:
    """The TLS context of the upgrade request: trusts only ``cafile``, refuses TLS below 1.2."""
    context = ssl.create_default_context(cafile=cafile)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    return context


def upgrade_status(path: str, cookie: str) -> int:
    """Sends the upgrade request of ``path`` from FOREIGN_ORIGIN and returns the answer's status.

    The request carries the console cookie and goes over TLS, trusting the test CA.
    """
    parts = urlsplit(ORIGIN)
    host = parts.hostname or "localhost"
    context = upgrade_tls_context(CA_FILE)
    key = base64.b64encode(secrets.token_bytes(16)).decode()
    request = upgrade_request(
        path, host=parts.netloc, origin=FOREIGN_ORIGIN, cookie=cookie, key=key
    )
    with (
        socket.create_connection((host, parts.port or 443), timeout=15) as raw,
        context.wrap_socket(raw, server_hostname=host) as tls,
    ):
        tls.sendall(request)
        head = b""
        while b"\r\n\r\n" not in head and len(head) < 16384:
            chunk = tls.recv(4096)
            if not chunk:
                break
            head += chunk
    return status_code(head)


def drive_patrol(run: TerminalRun, seed_json: str) -> dict[str, Any]:
    """Loads the bundle, starts its one patrol test and serves the synchronisation points.

    Returns the outcome of the test.
    """
    page = run.page
    open_bundle(page, seed_json)
    tree = page.evaluate("() => window.__patrol__getTests()")
    tests = [t for t in flatten_tests(tree["group"]) if not t.skip]
    if len(tests) != 1:
        raise CheckError(f"the terminal bundle lists {len(tests)} patrol tests, not one")
    run.patrol_mark = len(run.recorder.calls)
    page.evaluate(START_TEST_JS, [tests[0].name, TERMINAL_TEST_TIMEOUT_MS])
    for phase, handle in (("open", run.at_open), ("detached", run.at_detached)):
        page.wait_for_function(PHASE_JS, arg=phase, timeout=TERMINAL_TEST_TIMEOUT_MS)
        if not page.evaluate("(p) => window.__e2ePhase === p", phase):
            break
        say(f"patrol test at {phase}")
        handle()
        page.evaluate("(p) => { window.__e2eAck = p; }", phase)
    page.wait_for_function(
        "() => window.__e2eRunDone !== undefined", timeout=TERMINAL_TEST_TIMEOUT_MS
    )
    outcome = page.evaluate("() => window.__e2eRunDone")
    result = outcome if isinstance(outcome, dict) else {"result": "failure", "details": outcome}
    details = result.get("details")
    say(f"patrol test: {result.get('result')}")
    return {
        "name": tests[0].name,
        "result": result.get("result"),
        "details": None if details is None else str(details)[:DETAILS_MAX],
    }


def run_terminal(pw: Playwright, login: Login, spki: str) -> dict[str, Any]:
    """The whole phase in Chromium.

    Returns the result, whose ``ok`` says whether every check and the patrol test passed. Raises on
    a failure of the setup (login, seed, bundle).
    """
    browser = launch(pw, "chromium", spki)
    result: dict[str, Any] = {"browser": "chromium", "version": browser.version, "ok": False}
    session = Session(browser, "chromium", spki)
    page = session.page
    recorder = Recorder(result.setdefault("console", []))
    run: TerminalRun | None = None
    try:
        session.context.expose_binding("__patrol__platformHandler", PlatformHandler(page))
        page.on("console", recorder.on_console)
        page.on("response", recorder.on_response)
        run = TerminalRun(session, login, recorder)
        result["checks"] = run.checks
        page.goto(CONSOLE)
        say("log in as the owner, step-up, passkey")
        log_in(page, login)
        say("seed: the host Build host")
        seed = run.seed()
        check_terminal_seed(seed)
        run_check(run.checks, "b_password_totp_refused", run.check_password_totp)
        run_check(run.checks, "c_passkey_without_uv_refused", run.check_passkey_without_uv)
        say("patrol: open, measure, detach, reattach, log out")
        patrol = drive_patrol(run, json.dumps(seed))
        result["patrol"] = patrol
        run.finish(patrol)
        if patrol["result"] != "success":
            shot = RESULTS / "terminal-failure.png"
            page.screenshot(path=str(shot), full_page=True)
            result["screenshot"] = shot.name
        result["measurement"] = run.checks.get("h_measurement", {})
        result["failed_checks"] = failed_checks(run.checks)
        result["ok"] = patrol["result"] == "success" and not result["failed_checks"]
    except Exception as e:
        dump_screen(page)
        result["error"] = f"{type(e).__name__}: {e}"[:DETAILS_MAX]
        raise
    finally:
        result["violations"] = session.violations()
        result["console_errors"] = list(session.console_errors)
        result["foreign_origins"] = session.foreign()
        session.context.close()
        browser.close()
        (RESULTS / RESULT_FILE).write_text(json.dumps(result, indent=2), encoding="utf-8")
    return result


def main() -> int:
    """Runs phase 3 and returns 0 when every check and the patrol test passed."""
    RESULTS.mkdir(parents=True, exist_ok=True)
    login = Login(OUT / "console-login.json")
    # Phases 1 and 2 used TOTP steps the file does not know; the next code must come later.
    login.skip_current_step()
    spki = os.environ.get("E2E_SPKI", "")
    with sync_playwright() as pw:
        try:
            result = run_terminal(pw, login, spki)
        except Exception as e:
            say(f"FAIL {type(e).__name__}: {e}"[:DETAILS_MAX])
            traceback.print_exc(limit=3)
            return 1
    if not result["ok"]:
        say(f"FAIL checks {result['failed_checks']}, patrol {result['patrol']['result']}")
        return 1
    say("ok")
    return 0
