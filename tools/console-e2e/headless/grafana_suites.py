# SPDX-License-Identifier: Apache-2.0
"""Phase 1b of the console E2E test: the Grafana embedding (brief M6-17 Design note 4).

Runs in the browser container with ``E2E_MODE=grafana`` after ``console_e2e.py`` recreated
``api`` with the overlay ``compose.console-e2e-grafana.yaml``: ``console.grafana_embed_url`` names
the dashboard of the test proxy ``grafana-proxy`` (``E2E_EMBED_URL``, on the host
``grafana.localhost``, which Chromium resolves to loopback and which never gets the console's
cookie for ``localhost``), which answers with the headers of
docs/spec/25-visualization.md 9.2 item 3. In Chromium, with the console's CSP enforced:

* VZ-T12 and SEC-T19 with the key: the console's CSP has exactly one ``frame-src`` naming exactly
  the embed origin, and ``frame-ancestors 'none'`` and ``X-Frame-Options: DENY`` stay;
* the owner logs in (password and TOTP, step-up), sees "Operations (Grafana)" in the navigation,
  opens it, and the iframe carries ``sandbox="allow-scripts allow-same-origin"``,
  ``referrerpolicy="no-referrer"`` and exactly the embed URL; the test dashboard loads inside the
  cross-origin isolated console;
* SEC-T05 with the embedded dashboard: no violation, no console CSP error, no request to an origin
  other than the console and the embed origin, and the embed origin only from the iframe;
* the owner's ``getInstance`` names the URL; a person who is no owner (Robin Ops, handle
  ``robin-ops``, which no later phase uses, enrolled through the API with a DPoP key of the page)
  gets no ``grafana_embed_url``;
* negative: an iframe of a second origin of the proxy (``E2E_BLOCKED_ORIGIN``), which the CSP does
  not name, is blocked with a ``frame-src`` violation and never loads.

The result goes to ``/out/results/grafana.json``; it holds no secret, code or token.
"""

from __future__ import annotations

import json
import os
import sys
import traceback
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlsplit

from admin_suites import ConsoleAdmin, PageDevice
from console_headless import (
    CONSOLE,
    ORIGIN,
    OUT,
    RESULTS,
    STEP_TIMEOUT_MS,
    CheckError,
    Login,
    Session,
    add_virtual_authenticator,
    console_fetch,
    dump_screen,
    launch,
    login_with_password,
    register_passkey,
    step_up_with_password,
    wait_for_shell,
)
from playwright.sync_api import Page, Playwright, Request, sync_playwright

RESULT_FILE = "grafana.json"
# The navigation entry of the page (packages/l10n app_en.arb consoleNavGrafana).
GRAFANA_LABEL = "Operations (Grafana)"
# The element of the test dashboard (infra/compose/test/console-e2e/grafana-dashboard).
DASHBOARD_MARKER = "#bothy-test-dashboard"
SANDBOX = "allow-scripts allow-same-origin"
REFERRER_POLICY = "no-referrer"
# The violation events of the current document (console_headless.INIT_SCRIPT); the negative test
# waits for the frame-src event of the blocked origin with STEP_TIMEOUT_MS as the only bound.
BLOCKED_SEEN_JS = """(origin) => (window.__e2eViolations || []).some(
  (v) => v.directive === 'frame-src' && (v.blocked || '').startsWith(origin))"""
DETAILS_MAX = 2000
# The person who is no owner. Every phase runs on the same database, so the handle must be one no
# later phase creates (phase 2 creates sam and robin; a test of the runner holds this).
NON_OWNER_HANDLE = "robin-ops"


def origin_of(url: str) -> str:
    """The origin ``scheme://host[:port]`` of ``url`` in lower case."""
    parts = urlsplit(url)
    return f"{parts.scheme.lower()}://{parts.netloc.lower()}"


def frame_sources(csp: str) -> list[list[str]]:
    """The source lists of every ``frame-src`` directive of the policy ``csp``, in order."""
    out = []
    for directive in csp.split(";"):
        tokens = directive.split()
        if tokens and tokens[0].lower() == "frame-src":
            out.append(tokens[1:])
    return out


def check_policy(headers: Mapping[str, str], embed_origin: str) -> None:
    """Raises CheckError unless the console's headers allow exactly the embed origin as a frame.

    Exactly one ``frame-src`` with exactly ``embed_origin`` (VZ-T12, SEC-T19), the console itself
    still not embeddable: ``frame-ancestors 'none'`` and ``X-Frame-Options: DENY``.
    """
    lower = {k.lower(): v for k, v in headers.items()}
    csp = lower.get("content-security-policy", "")
    sources = frame_sources(csp)
    if sources != [[embed_origin]]:
        raise CheckError(f"frame-src of the console is {sources}, want exactly [{embed_origin}]")
    if "frame-ancestors 'none'" not in [d.strip() for d in csp.split(";")]:
        raise CheckError("the console's CSP lost frame-ancestors 'none'")
    if lower.get("x-frame-options", "").upper() != "DENY":
        raise CheckError("the console lost X-Frame-Options: DENY")


@dataclass(frozen=True)
class Seen:
    """One request of the context: its origin and whether a subframe (the iframe) made it."""

    origin: str
    from_frame: bool


def foreign_requests(seen: Sequence[Seen], console: str, embed: str) -> list[str]:
    """The requests that break SEC-T05 with an embedded dashboard, as ``origin (main frame)``.

    Every origin other than the console's is foreign, except the embed origin when a subframe
    requested it (the iframe loads the dashboard; the console itself never talks to Grafana).
    The recorder leaves ``data:`` and ``blob:`` URLs out; they are local.
    """
    out = set()
    for s in seen:
        if s.origin == console:
            continue
        if s.origin == embed and s.from_frame:
            continue
        out.add(f"{s.origin} ({'frame' if s.from_frame else 'main frame'})")
    return sorted(out)


def say(message: str) -> None:
    """One progress line of phase 1b on stderr."""
    print(f"console-headless: grafana: {message}", file=sys.stderr, flush=True)


class Requests:
    """Records the origin of every request of a context and whether a subframe made it."""

    def __init__(self) -> None:
        """Starts empty."""
        self.seen: list[Seen] = []

    def __call__(self, request: Request) -> None:
        """The request listener of the context."""
        parts = urlsplit(request.url)
        if parts.scheme in ("data", "blob"):
            return
        scheme = {"wss": "https", "ws": "http"}.get(parts.scheme, parts.scheme)
        try:
            from_frame = request.frame.parent_frame is not None
        except Exception:  # a request of a worker has no frame
            from_frame = False
        self.seen.append(Seen(f"{scheme}://{parts.netloc}", from_frame))


HEADERS_JS = """async (path) => {
  const r = await fetch(path, {credentials: 'same-origin'});
  const headers = {};
  r.headers.forEach((v, k) => { headers[k] = v; });
  return headers;
}"""

BLOCKED_FRAME_JS = """(url) => {
  const f = document.createElement('iframe');
  f.id = 'e2e-blocked-frame';
  f.src = url;
  document.body.appendChild(f);
}"""


def open_dashboard(page: Page, embed_url: str) -> dict[str, Any]:
    """Opens the page from the navigation and waits for the test dashboard in the iframe.

    Returns the iframe's attributes; raises CheckError when they differ from 25 9.2 item 5 or the
    dashboard does not load. The locators pierce the open shadow root of the Flutter view, where
    the engine may put the element of a platform view.
    """
    page.get_by_role("button", name=GRAFANA_LABEL, exact=True).click()
    frames = page.locator("iframe")
    frames.first.wait_for(state="attached")
    if frames.count() != 1:
        raise CheckError(f"want exactly one iframe on the page, got {frames.count()}")
    element = frames.first
    attrs = {name: element.get_attribute(name) for name in ("sandbox", "referrerpolicy", "src")}
    if attrs["sandbox"] != SANDBOX or attrs["referrerpolicy"] != REFERRER_POLICY:
        raise CheckError(f"iframe attributes {attrs}")
    if attrs["src"] != embed_url:
        raise CheckError("the iframe does not load exactly grafana_embed_url")
    frame = element.element_handle().content_frame()
    if frame is None:
        raise CheckError("the iframe has no frame")
    frame.wait_for_selector(DASHBOARD_MARKER, state="attached")
    if origin_of(frame.url) != origin_of(embed_url):
        raise CheckError("the frame of the dashboard is not on the embed origin")
    return attrs


def check_members(page: Page, embed_url: str) -> None:
    """The owner's getInstance names the URL; Robin, who is no owner, gets no member.

    Robin is created by the owner (createUser; a passkey step-up when the step-up of the login
    has expired) and
    redeems the enrollment code with a device whose DPoP key lives in the page; the getInstance of
    that device must not carry ``grafana_embed_url`` (25 9.2 item 6, brief M6-17 Design note 3).
    """
    owner = console_fetch(page, "GET", "/api/v1/instance")
    if owner["status"] != 200 or (owner["json"] or {}).get("grafana_embed_url") != embed_url:
        raise CheckError(f"the owner's getInstance answered {owner['status']} without the URL")
    created = ConsoleAdmin(page).call(
        "createUser",
        "POST",
        "/users",
        {
            "handle": NON_OWNER_HANDLE,
            "display_name": "Robin Ops",
            "locale": "en",
            "local_account": True,
        },
        idempotent=True,
    )
    code = str(created["json"]["enrollment"]["code"])
    device = PageDevice.create(page, "robin-grafana")
    redeemed = device.call(
        "redeemEnrollmentCode",
        "POST",
        "/auth/enroll/redeem",
        {"enrollment_code": code, "device": device.registration("Robin Ops phone", None)},
        want=201,
    )
    device.token = str(redeemed["access_token"])
    info = device.call("getInstance", "GET", "/instance")
    if "grafana_embed_url" in info:
        raise CheckError("getInstance of a person who is no owner names grafana_embed_url")


def check_blocked(session: Session, page: Page, blocked_origin: str) -> None:
    """An iframe of an origin the CSP does not name is blocked and reported as frame-src.

    The test waits for the violation event in the document (bounded by STEP_TIMEOUT_MS, never a
    fixed time); the browser fires it when it refuses the navigation, so no frame of that origin
    may exist afterwards. The proxy's access log (console_e2e.check_proxy_requests) proves that no
    request reached it.
    """
    page.evaluate(BLOCKED_FRAME_JS, blocked_origin + "/d/blocked")
    try:
        page.wait_for_function(BLOCKED_SEEN_JS, arg=blocked_origin, timeout=STEP_TIMEOUT_MS)
    except Exception as e:
        raise CheckError(
            f"no frame-src violation for the second origin: {session.violations()}"
        ) from e
    for f in page.frames:
        if origin_of(f.url) == blocked_origin:
            raise CheckError("the frame of the second origin loaded")


def run_grafana(pw: Playwright, login: Login, spki: str) -> dict[str, Any]:
    """The whole phase in Chromium; returns the result and raises CheckError on a failure."""
    embed_url = os.environ["E2E_EMBED_URL"]
    blocked = os.environ["E2E_BLOCKED_ORIGIN"].rstrip("/")
    embed = origin_of(embed_url)
    browser = launch(pw, "chromium", spki)
    result: dict[str, Any] = {"browser": "chromium", "version": browser.version, "embed": embed}
    session = Session(browser, "chromium", spki)
    requests = Requests()
    session.context.on("request", requests)
    page = session.page
    try:
        add_virtual_authenticator(session)
        say("log in as the owner, step-up")
        login_with_password(page, login)
        step_up_with_password(page, login)
        wait_for_shell(page)
        say("the CSP names exactly the embed origin")
        check_policy(page.evaluate(HEADERS_JS, "/console/"), embed)
        say("open Operations (Grafana)")
        result["iframe"] = open_dashboard(page, embed_url)
        result["cross_origin_isolated"] = page.evaluate("crossOriginIsolated")
        if result["cross_origin_isolated"] is not True:
            raise CheckError("the console is not cross-origin isolated with the dashboard")
        say("getInstance of the owner and of a person who is no owner")
        register_passkey(page)
        check_members(page, embed_url)
        result["violations"] = session.violations()
        result["console_errors"] = session.console_errors
        result["foreign_requests"] = foreign_requests(requests.seen, ORIGIN, embed)
        if result["violations"] or result["console_errors"]:
            raise CheckError(f"CSP violations: {result['violations']} {result['console_errors']}")
        if result["foreign_requests"]:
            raise CheckError(f"requests to foreign origins: {result['foreign_requests']}")
        say("negative: a frame of another origin is blocked")
        check_blocked(session, page, blocked)
        result["negative_frame"] = "blocked and reported"
        result["ok"] = True
    except Exception as e:
        dump_screen(page)
        result["ok"] = False
        result["error"] = f"{type(e).__name__}: {e}"[:DETAILS_MAX]
        result.setdefault("violations", session.violations())
        result.setdefault("console_errors", session.console_errors)
        raise
    finally:
        session.context.close()
        browser.close()
        (RESULTS / RESULT_FILE).write_text(json.dumps(result, indent=2), encoding="utf-8")
    return result


def main() -> int:
    """Runs phase 1b and returns 0 when it passed."""
    RESULTS.mkdir(parents=True, exist_ok=True)
    login = Login(OUT / "console-login.json")
    # Phase 1 used TOTP steps the file does not know; the next code must come from a later step.
    login.skip_current_step()
    spki = os.environ.get("E2E_SPKI", "")
    with sync_playwright() as pw:
        try:
            run_grafana(pw, login, spki)
        except Exception as e:
            say(f"FAIL {type(e).__name__}: {e}"[:DETAILS_MAX])
            traceback.print_exc(limit=3)
            return 1
    say(f"ok ({CONSOLE})")
    return 0
