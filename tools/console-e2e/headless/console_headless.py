# SPDX-License-Identifier: Apache-2.0
"""Headless test SEC-T05 of the web console (brief M6-03 Design note 5).

Runs inside the pinned Playwright image (service ``console-browser`` of
``infra/compose/test/compose.console-e2e.yaml``) in the network namespace of ``api`` and opens the
console at ``E2E_ORIGIN`` through the TLS sidecar, with the Content Security Policy of the server
enforced. Per browser of ``E2E_BROWSERS``:

* Chromium: log in with the console form (password and TOTP of ``/out/console-login.json``), the
  step-up with password and TOTP, a passkey registered through the API of the console session
  with the virtual authenticator of the browser (``userVerification`` on), a second login whose
  step-up uses that passkey, every navigation entry with its first frame, a WebSocket to the
  gateway of the own origin, ``crossOriginIsolated``, and the negative test: an inline script in a
  copy of ``index.html`` (served with the headers of the server) is blocked and reported.
* Firefox and WebKit: the same login, step-up with password and TOTP and navigation as a smoke test.

With ``E2E_MODE=admin`` the module runs phase 2 of brief M6-04 instead (``admin_suites.py``).

Every browser fails on any ``securitypolicyviolation`` event, on any request to an origin other
than the console origin and on a WebSocket the policy blocks. The result of each browser goes to
``/out/results/<browser>.json`` (violations, foreign requests, the observed Trusted Types policy
names and the browser version; never a secret). Exit code 0 when every browser passed.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import struct
import sys
import time
import traceback
from pathlib import Path
from typing import Any, cast
from urllib.parse import urlsplit

from playwright.sync_api import (
    Browser,
    BrowserContext,
    ConsoleMessage,
    Page,
    Playwright,
    WebSocket,
    sync_playwright,
)

ORIGIN = os.environ.get("E2E_ORIGIN", "https://localhost:8443").rstrip("/")
CONSOLE = ORIGIN + "/console/"
OUT = Path(os.environ.get("E2E_OUT", "/out"))
RESULTS = OUT / "results"
# Waits of one step; the stack runs on a CI host, so they are generous but bounded.
STEP_TIMEOUT_MS = 60_000
FRAME_WAIT_MS = 1_500
TOTP_PERIOD = 30

# English texts of the console (packages/l10n app_en.arb); the context locale is en-US. The
# labels that name a password are labels, not passwords (nosec B105).
T = {
    "login_name": "Login name",
    "password_and_code": "Password and one-time code",  # nosec B105
    "password": "Password",  # nosec B105
    "code": "One-time code",
    "login": "Log in",
    "stepup_password": "Confirm with password and one-time code",  # nosec B105
    "stepup_passkey": "Confirm with passkey",
    "confirm": "Confirm",
    "logout": "Log out",
}
NAVIGATION = [
    "Live",
    "Statistics",
    "Hosts and terminal",
    "People",
    "Groups and access grants",
    "Devices",
    "Agents and costs",
    "Apps and webhooks",
    "Audit",
    "Import",
    "Instance and branding",
    "Status",
]

# Collects every violation event and every Trusted Types policy the page creates, before any
# script of the page runs (Playwright init script, main world): in arrays of the document (the
# negative test reads them) and through the bindings of the context into Python, so nothing is
# lost when a navigation replaces the document.
INIT_SCRIPT = """
(() => {
  window.__e2eViolations = [];
  window.__e2ePolicies = [];
  document.addEventListener('securitypolicyviolation', (e) => {
    const v = {directive: e.effectiveDirective, blocked: e.blockedURI,
      disposition: e.disposition, sample: (e.sample || '').slice(0, 80)};
    window.__e2eViolations.push(v);
    if (window.__e2eRecordViolation) window.__e2eRecordViolation(v);
  }, true);
  if (window.trustedTypes && window.trustedTypes.createPolicy) {
    const create = window.trustedTypes.createPolicy.bind(window.trustedTypes);
    window.trustedTypes.createPolicy = (name, rules) => {
      window.__e2ePolicies.push(name);
      if (window.__e2eRecordPolicy) window.__e2eRecordPolicy(name);
      return create(name, rules);
    };
  }
})();
"""


# Waits until the editing element of the field labelled ``label`` has the focus.
FOCUSED_JS = """(label) => document.activeElement
  && document.activeElement.getAttribute('aria-label') === label"""

# The roles and labels of the semantics tree, for the diagnosis of a failed step.
SCREEN_JS = """Array.from(document.querySelectorAll('flt-semantics [role], flt-semantics[role]'))
  .map(e => (e.getAttribute('role') || '') + ':'
    + (e.getAttribute('aria-label') || e.textContent || '').slice(0, 60))"""

# navigator.credentials.create over the JSON options of the server (binary members base64url) and
# the credential in JSON form for completePasskeyRegistration.
CREATE_PASSKEY_JS = """async (options) => {
  const b64u = (s) => Uint8Array.from(
    atob(s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4)),
    (c) => c.charCodeAt(0));
  const enc = (buf) => btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\\+/g, '-').replace(/\\//g, '_').replace(/=+$/, '');
  const pk = options.publicKey || options;
  pk.challenge = b64u(pk.challenge);
  pk.user.id = b64u(pk.user.id);
  (pk.excludeCredentials || []).forEach((c) => { c.id = b64u(c.id); });
  const cred = await navigator.credentials.create({publicKey: pk});
  return {id: cred.id, rawId: enc(cred.rawId), type: cred.type,
    response: {clientDataJSON: enc(cred.response.clientDataJSON),
      attestationObject: enc(cred.response.attestationObject),
      transports: cred.response.getTransports ? cred.response.getTransports() : []},
    clientExtensionResults: cred.getClientExtensionResults()};
}"""


class CheckError(Exception):
    """A condition of SEC-T05 failed."""


def totp(secret_b32: str, step: int) -> str:
    """The six-digit TOTP code of ``secret_b32`` (base32, no padding) for ``step`` (RFC 6238)."""
    secret = base64.b32decode(secret_b32 + "=" * (-len(secret_b32) % 8))
    digest = hmac.new(secret, struct.pack(">Q", step), hashlib.sha1).digest()
    off = digest[-1] & 0x0F
    value = struct.unpack(">I", digest[off : off + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"


class Login:
    """The local account of the seed; hands out each TOTP step once (08 2.5)."""

    def __init__(self, path: Path) -> None:
        """Read the seed file at ``path``."""
        data = json.loads(path.read_text(encoding="utf-8"))
        self.name: str = data["login_name"]
        self.password: str = data["password"]
        self._secret: str = data["totp_secret"]
        self._last: int = int(data["last_step"])

    def code(self) -> str:
        """Waits for the next unused step and returns its code."""
        while int(time.time()) // TOTP_PERIOD <= self._last:
            time.sleep(1)
        self._last = int(time.time()) // TOTP_PERIOD
        return totp(self._secret, self._last)

    def skip_current_step(self) -> None:
        """Treats the current step as used.

        A later run of the same seed (phase 2 of console_e2e.py) does not know which steps an
        earlier one took.
        """
        self._last = max(self._last, int(time.time()) // TOTP_PERIOD)


class Session:
    """One browser context with the collectors of SEC-T05."""

    def __init__(self, browser: Browser, name: str, spki: str) -> None:
        """Open a context of ``browser`` for the browser ``name``."""
        del spki
        self.name = name
        self.context: BrowserContext = browser.new_context(
            locale="en-US",
            ignore_https_errors=name != "chromium",
            viewport={"width": 1280, "height": 900},
        )
        self.requests: list[str] = []
        self.websockets: list[str] = []
        self.console_errors: list[str] = []
        self.all_violations: list[dict[str, str]] = []
        self.all_policies: set[str] = set()
        self.context.expose_function("__e2eRecordViolation", self._record_violation)
        self.context.expose_function("__e2eRecordPolicy", self._record_policy)
        self.context.add_init_script(INIT_SCRIPT)
        # Every request of every page of the context, the browser's own included.
        self.context.on("request", lambda r: self.requests.append(r.url))
        self.page: Page = self.context.new_page()
        self.page.set_default_timeout(STEP_TIMEOUT_MS)
        self.page.on("console", self._console)
        self.page.on("websocket", self._websocket)

    def _record_violation(self, violation: dict[str, str]) -> None:
        self.all_violations.append(violation)

    def _record_policy(self, name: str) -> None:
        self.all_policies.add(name)

    def _console(self, msg: ConsoleMessage) -> None:
        text = msg.text
        if msg.type == "error" and (
            "Content Security Policy" in text or "TrustedType" in text or "Trusted Type" in text
        ):
            self.console_errors.append(text[:300])

    def _websocket(self, ws: WebSocket) -> None:
        self.websockets.append(ws.url)
        self.requests.append(ws.url)

    def violations(self) -> list[dict[str, str]]:
        """The violation events of every document of the session so far."""
        return list(self.all_violations)

    def policies(self) -> list[str]:
        """The Trusted Types policy names every document of the session created so far."""
        return sorted(self.all_policies)

    def foreign(self) -> list[str]:
        """Requests to an origin other than the console origin (data: and blob: are local)."""
        out = []
        for url in self.requests:
            parts = urlsplit(url)
            if parts.scheme in ("data", "blob"):
                continue
            # A WebSocket of the own origin uses ws or wss for the scheme of the page.
            scheme = {"wss": "https", "ws": "http"}.get(parts.scheme, parts.scheme)
            if f"{scheme}://{parts.netloc}" == ORIGIN:
                continue
            out.append(f"{parts.scheme}://{parts.netloc}")
        return sorted(set(out))


def wait_for_console(page: Page) -> None:
    """Waits until the Flutter view and its semantics tree are in the document."""
    page.wait_for_selector("flutter-view", state="attached")
    page.wait_for_function(
        "document.querySelectorAll('flt-semantics').length > 2", timeout=STEP_TIMEOUT_MS
    )


def fill(page: Page, label: str, value: str) -> None:
    """Types ``value`` into the text field labelled ``label`` key by key and checks the result.

    Flutter keeps the text in its own editing element, which it focuses a moment after the tap;
    a key typed before that is lost. The test waits for the focus, types like a person and
    compares the field's value, and types again once when a key was lost.
    """
    field = page.get_by_role("textbox", name=label, exact=True)
    for _ in range(3):
        field.click()
        page.wait_for_function(FOCUSED_JS, arg=label)
        page.keyboard.press("ControlOrMeta+A")
        page.keyboard.press("Backspace")
        page.keyboard.type(value, delay=20)
        got = page.evaluate("() => document.activeElement ? document.activeElement.value : null")
        if got == value:
            return
    raise CheckError(f"the field {label} did not take the typed text")


def press(page: Page, label: str) -> None:
    """Taps the button labelled ``label``."""
    page.get_by_role("button", name=label, exact=True).click()


def login_with_password(page: Page, login: Login) -> None:
    """Logs in with the console form: name, password and one-time code."""
    page.goto(CONSOLE)
    wait_for_console(page)
    fill(page, T["login_name"], login.name)
    press(page, T["password_and_code"])
    fill(page, T["password"], login.password)
    fill(page, T["code"], login.code())
    press(page, T["login"])


def step_up_with_password(page: Page, login: Login) -> None:
    """Confirms with password and one-time code on the step-up screen."""
    press(page, T["stepup_password"])
    fill(page, T["password"], login.password)
    fill(page, T["code"], login.code())
    press(page, T["confirm"])


def wait_for_shell(page: Page) -> None:
    """Waits until the navigation of the shell is there (the step-up succeeded)."""
    page.get_by_role("button", name=NAVIGATION[0], exact=True).wait_for()


def visit_navigation(page: Page) -> list[str]:
    """Opens every navigation entry and waits for its first frame; returns the opened paths."""
    paths = []
    for label in NAVIGATION:
        page.get_by_role("button", name=label, exact=True).click()
        page.wait_for_timeout(FRAME_WAIT_MS)
        page.evaluate("new Promise((r) => requestAnimationFrame(() => r(true)))")
        paths.append(urlsplit(page.url).path)
    return paths


def console_fetch(page: Page, method: str, path: str, body: object | None = None) -> dict[str, Any]:
    """A request of the console session from the page: cookie, CSRF header, same origin."""
    answer: dict[str, Any] = page.evaluate(
        """async ([method, path, body]) => {
          const headers = {'X-Console-Request': '1', 'Accept': 'application/json'};
          if (body !== null) headers['Content-Type'] = 'application/json';
          const r = await fetch(path, {method, credentials: 'same-origin', headers,
            body: body === null ? undefined : JSON.stringify(body)});
          let json = null;
          try { json = await r.json(); } catch (e) { json = null; }
          return {status: r.status, json};
        }""",
        [method, path, body],
    )
    return answer


def register_passkey(page: Page) -> None:
    """Registers a passkey of the console session with the virtual authenticator (Chromium).

    The console has no page for passkeys yet (M6-04); the test drives the two registration
    operations from the page of the console session, so navigator.credentials.create runs under
    the CSP of the console (docs/DECISIONS.md D-M6-03-6).
    """
    begin = console_fetch(page, "POST", "/api/v1/me/passkeys/begin")
    if begin["status"] != 200:
        raise CheckError(f"beginPasskeyRegistration answered {begin['status']}")
    data = begin["json"]
    if not isinstance(data, dict):
        raise CheckError("beginPasskeyRegistration answered no object")
    created = page.evaluate(CREATE_PASSKEY_JS, data["public_key"])
    done = console_fetch(
        page,
        "POST",
        "/api/v1/me/passkeys/complete",
        {"challenge_id": data["challenge_id"], "credential": created, "name": "console e2e"},
    )
    if done["status"] not in (200, 201):
        raise CheckError(f"completePasskeyRegistration answered {done['status']}")


def websocket_check(session: Session) -> None:
    """Opens a WebSocket to the gateway of the own origin from the page and waits for its end.

    The server may accept or refuse the connection; the check is that the policy did not block
    it (no violation of connect-src) and that the handshake reached the server (19 5 rule 5).
    """
    outcome = session.page.evaluate(
        """() => new Promise((resolve) => {
          const url = location.origin.replace(/^http/, 'ws') + '/api/v1/gateway';
          let ws;
          try { ws = new WebSocket(url); } catch (e) { resolve({thrown: String(e)}); return; }
          const t = setTimeout(() => {
            try { ws.close(); } catch (e) {}
            resolve({timeout: true});
          }, 15000);
          ws.onmessage = () => { clearTimeout(t); ws.close(1000); resolve({message: true}); };
          ws.onclose = (e) => { clearTimeout(t); resolve({closed: e.code}); };
        })"""
    )
    blocked = [v for v in session.violations() if v.get("directive") == "connect-src"]
    if blocked or "thrown" in outcome:
        raise CheckError(f"the policy blocked the WebSocket: {outcome} {blocked}")
    # A frame of the server or a close code other than 1006 (no handshake) shows that the
    # handshake reached the server; Playwright's websocket event is a second, not always timely,
    # witness (Firefox).
    reached = (
        outcome.get("message") is True
        or outcome.get("closed") not in (None, 1006)
        or any(url.endswith("/api/v1/gateway") for url in session.websockets)
    )
    if not reached:
        raise CheckError(f"the WebSocket never reached the network: {outcome}")


def negative_inline_script(session: Session) -> None:
    """Serves index.html with an inline script and the server's headers; the script must not run.

    The page first fetches ``/console/csp-negative`` from the server (the SPA fallback, so
    index.html with a fresh nonce) and reads its headers; Playwright then answers the navigation
    to that path with exactly those headers and the body plus one inline script. The CSP is the
    server's own (the sidecar rewrite of the brief is replaced by Playwright's route,
    docs/DECISIONS.md D-M6-03-6). The browser must block the script, fire a violation event and
    send a report to the endpoint, which console_e2e.py then finds in the server log.
    """
    page = session.page
    marker = "e2e-inline-" + os.urandom(4).hex()
    target = ORIGIN + "/console/csp-negative"
    served = page.evaluate(
        """async (path) => {
          const r = await fetch(path, {credentials: 'same-origin'});
          const headers = {};
          r.headers.forEach((v, k) => { headers[k] = v; });
          return {status: r.status, headers, body: await r.text()};
        }""",
        "/console/csp-negative",
    )
    if "content-security-policy" not in served["headers"]:
        raise CheckError("the server sent index.html without a CSP")
    body = served["body"].replace(
        "</body>", f"<script>window.__e2eInline = '{marker}';</script></body>"
    )

    def inject(route: object) -> None:
        fulfill = route.fulfill  # type: ignore[attr-defined]
        fulfill(status=served["status"], headers=served["headers"], body=body)

    page.route(target, inject)
    try:
        page.goto(target, wait_until="domcontentloaded")
        page.wait_for_timeout(2_000)
        ran = page.evaluate("window.__e2eInline || null")
        violations = page.evaluate("window.__e2eViolations || []")
    finally:
        page.unroute(target)
    if ran is not None:
        raise CheckError("the inline script ran under the CSP")
    if not any(v.get("directive") == "script-src-elem" for v in violations):
        raise CheckError(f"no violation event for the inline script: {violations}")
    # Chromium sends the queued report after the short reporting delay (launch flag).
    page.wait_for_timeout(5_000)
    # Control of the collector: after the next navigation the violation of this document must
    # still be in the session, as the violations of the login and the navigation would be.
    page.goto(CONSOLE)
    wait_for_console(page)
    if not any(v.get("directive") == "script-src-elem" for v in session.violations()):
        raise CheckError("the collector lost a violation when the document changed")


def launch(pw: Playwright, name: str, spki: str) -> Browser:
    """Start browser ``name``.

    Chromium trusts the leaf of the test CA by its SPKI hash and runs as the full browser in its
    new headless mode (channel chromium): the headless shell never delivers Reporting API reports,
    and --short-reporting-delay sends them after a short delay instead of Chromium's default
    interval of a minute, so the report of the negative test reaches the server while the stack
    runs.
    """
    if name == "chromium":
        args = [f"--ignore-certificate-errors-spki-list={spki}", "--short-reporting-delay"]
        return pw.chromium.launch(args=args, channel="chromium")
    return cast("Browser", getattr(pw, name).launch())


def add_virtual_authenticator(session: Session) -> None:
    """A platform authenticator with resident keys and user verification (Chromium, CDP)."""
    cdp = session.context.new_cdp_session(session.page)
    cdp.send("WebAuthn.enable")
    options = {
        "protocol": "ctap2",
        "transport": "internal",
        "hasResidentKey": True,
        "hasUserVerification": True,
        "isUserVerified": True,
        "automaticPresenceSimulation": True,
    }
    cdp.send("WebAuthn.addVirtualAuthenticator", {"options": options})


def passkey_round(page: Page, login: Login) -> None:
    """Registers a passkey, logs out, logs in again and does the step-up with the passkey."""
    register_passkey(page)
    press(page, T["logout"])
    page.wait_for_timeout(1_000)
    login_with_password(page, login)
    press(page, T["stepup_passkey"])
    wait_for_shell(page)


def assert_clean(session: Session, result: dict[str, object]) -> None:
    """Records violations, console errors and foreign origins; raises CheckError on any."""
    result["violations"] = session.violations()
    result["console_errors"] = session.console_errors
    result["foreign_origins"] = session.foreign()
    if result["violations"] or result["console_errors"]:
        raise CheckError(f"CSP violations: {result['violations']} {result['console_errors']}")
    if result["foreign_origins"]:
        raise CheckError(f"requests to foreign origins: {result['foreign_origins']}")


def dump_screen(page: Page) -> None:
    """Prints the roles and labels of the semantics tree after a failed step."""
    try:
        labels = page.evaluate(SCREEN_JS)
        print("console-headless: screen: " + "; ".join(labels[:60]), file=sys.stderr, flush=True)
    except Exception as dump:
        print(f"console-headless: no screen dump: {dump}", file=sys.stderr, flush=True)


def run_browser(pw: Playwright, name: str, login: Login, spki: str) -> dict[str, object]:
    """The flow of one browser; returns its result and raises CheckError on a failed condition."""
    browser = launch(pw, name, spki)
    result: dict[str, object] = {"browser": name, "version": browser.version}
    session = Session(browser, name, spki)
    page = session.page
    try:
        if name == "chromium":
            add_virtual_authenticator(session)
        login_with_password(page, login)
        step_up_with_password(page, login)
        wait_for_shell(page)
        policies = set(session.policies())
        result["paths"] = visit_navigation(page)
        result["cross_origin_isolated"] = page.evaluate("crossOriginIsolated")
        if result["cross_origin_isolated"] is not True:
            raise CheckError("the console is not cross-origin isolated")
        if name == "chromium":
            passkey_round(page, login)
            result["passkey_step_up"] = True
        websocket_check(session)
        result["websocket"] = "not blocked, reached the server"
        result["trusted_types_policies"] = sorted(policies | set(session.policies()))
        assert_clean(session, result)
        if name == "chromium":
            negative_inline_script(session)
            result["negative_inline_script"] = "blocked and reported"
    except Exception:
        dump_screen(page)
        raise
    finally:
        session.context.close()
        browser.close()
    result["ok"] = True
    return result


def main() -> int:
    """Runs every browser of E2E_BROWSERS and writes the results; 0 when all passed.

    ``E2E_MODE=admin`` runs phase 2 instead, the patrol suites of the console administration in
    Chromium (admin_suites.py, brief M6-04 Design note 4).
    """
    if os.environ.get("E2E_MODE") == "admin":
        import admin_suites  # noqa: PLC0415 - phase 2 only; it imports this module

        return admin_suites.main()
    RESULTS.mkdir(parents=True, exist_ok=True)
    login = Login(OUT / "console-login.json")
    spki = os.environ.get("E2E_SPKI", "")
    failed = []
    with sync_playwright() as pw:
        for name in [
            b.strip()
            for b in os.environ.get("E2E_BROWSERS", "chromium,firefox,webkit").split(",")
            if b.strip()
        ]:
            try:
                result = run_browser(pw, name, login, spki)
                print(
                    f"console-headless: {name} {result['version']}: ok", file=sys.stderr, flush=True
                )
            except Exception as e:
                failed.append(name)
                result = {"browser": name, "ok": False, "error": f"{type(e).__name__}: {e}"[:2000]}
                print(
                    f"console-headless: {name}: FAIL {result['error']}", file=sys.stderr, flush=True
                )
                traceback.print_exc(limit=3)
            (RESULTS / f"{name}.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
