# SPDX-License-Identifier: Apache-2.0
"""Capture of the web console from the demo instance (brief M7-28 Design note 1).

The console of the demo's server runs in headless Chromium (Playwright of the tools environment)
at the demo's address. Alex logs in with a password and TOTP that `bothy-smoke seed` gives him
with a fresh enrollment code (the probe of the console test of M6-03, built from
`services/server/Dockerfile` target `smoke` and run in the network namespace of the demo's
`api`), confirms the step-up the console asks for at login, and the run captures the views of
`VIEWS` in the light and the dark colour scheme: the console follows the scheme of the browser,
which Playwright emulates. A screenshot of the page is the viewport only: no browser frame and
nothing of the build host. The login stays in the temp folder of the run and is never printed.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import struct
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from playwright.sync_api import Page, ViewportSize

from capture import Demo, StepError, demo_container, docker, say

# English texts of the console (packages/l10n app_en.arb); the context locale is en-US. Labels
# that name a password are labels, not passwords.
T = {
    "login_name": "Login name",
    "password_and_code": "Password and one-time code",  # nosec B105
    "password": "Password",  # nosec B105
    "code": "One-time code",
    "login": "Log in",
    "stepup_password": "Confirm with password and one-time code",  # nosec B105
    "confirm": "Confirm",
    "tv": "TV mode",
}
# The views of the showcase: (navigation entry, image name); TV mode is opened from Live.
VIEWS: tuple[tuple[str, str], ...] = (
    ("Live", "live"),
    ("Statistics", "statistics"),
    ("Hosts and terminal", "hosts"),
    ("Agents and costs", "agents-costs"),
    ("People", "people"),
    ("Audit", "audit"),
)
VIEWPORT: ViewportSize = {"width": 1440, "height": 900}
STEP_TIMEOUT_MS = 120_000
# A view is captured once its first frames are drawn and the console asked no more data for
# this long (the queries of a page on a local demo answer well within it).
SETTLE_MS = 2_500
TOTP_PERIOD = 30


@dataclass
class Login:
    """Alex's console login from the seed.

    Attributes:
        name: the login name.
        password: the password.
        secret: the TOTP secret (raw bytes).
        last_step: the last TOTP step the server accepted.
    """

    name: str
    password: str
    secret: bytes
    last_step: int

    def code(self) -> str:
        """Return a TOTP code of a step above every step used so far (waits for it)."""
        step = int(time.time()) // TOTP_PERIOD
        while step <= self.last_step:
            time.sleep(0.5)
            step = int(time.time()) // TOTP_PERIOD
        self.last_step = step
        return totp(self.secret, step)


def totp(secret: bytes, step: int) -> str:
    """Return the six-digit TOTP code of RFC 6238 (HMAC-SHA-1) for `step`.

    Example: `totp(b"1234567890" * 2, 1)` is `"287082"` (RFC 6238 test vector, T=59 s).
    """
    digest = hmac.new(secret, struct.pack(">Q", step), hashlib.sha1).digest()
    offset = digest[-1] & 0x0F
    value = struct.unpack(">I", digest[offset : offset + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"


def parse_login(text: str) -> Login:
    """Parse the console-login.json of `bothy-smoke seed`.

    Raises:
        StepError: a key is missing or the secret is no base32.
    """
    try:
        data = json.loads(text)
        secret = data["totp_secret"].upper()
        raw = base64.b32decode(secret + "=" * (-len(secret) % 8))
        return Login(data["login_name"], data["password"], raw, int(data["last_step"]))
    except (KeyError, ValueError, TypeError, AttributeError) as exc:
        raise StepError(f"console: the seed wrote no valid login ({type(exc).__name__})") from exc


def seed_login(root: Path, demo: Demo, code: str, work: Path) -> Login:
    """Give Alex a password and TOTP with `bothy-smoke seed` and return the login."""
    version = (root / "services/server/VERSION").read_text(encoding="utf-8").strip()
    tag = f"bothy-smoke:showcase-{os.getpid()}"
    out = work / "console-seed"
    out.mkdir(mode=0o700)
    (out / "setup.out").write_text(code + "\n", encoding="utf-8")
    (out / "setup.out").chmod(0o600)
    say("console: building the probe bothy-smoke")
    subprocess.run(
        [
            "docker",
            "buildx",
            "build",
            "--quiet",
            "--load",
            "--target",
            "smoke",
            "--build-arg",
            f"VERSION={version}",
            "--file",
            "services/server/Dockerfile",
            "--tag",
            tag,
            ".",
        ],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )
    try:
        docker(
            "run",
            "--rm",
            "--network",
            f"container:{demo_container(demo, 'api')}",
            "--user",
            f"{os.getuid()}:{os.getgid()}",
            "--volume",
            f"{out}:/out",
            tag,
            "seed",
            "--public-url",
            demo.url,
            "--out",
            "/out",
        )
    finally:
        docker("image", "rm", "--force", tag, check=False)
    return parse_login((out / "console-login.json").read_text(encoding="utf-8"))


def capture(root: Path, demo: Demo, code: str, work: Path) -> list[tuple[Path, str, str, str]]:
    """Capture the console views in both colour schemes; returns (file, kind, view, theme)."""
    from playwright.sync_api import sync_playwright  # noqa: PLC0415 - only for this platform

    login = seed_login(root, demo, code, work)
    out = work / "console"
    out.mkdir()
    files: list[tuple[Path, str, str, str]] = []
    with sync_playwright() as pw:
        browser = pw.chromium.launch()
        try:
            # The demo CA is trusted only in this context of the run's browser.
            context = browser.new_context(
                viewport=VIEWPORT,
                device_scale_factor=1,
                locale="en-US",
                timezone_id="UTC",
                color_scheme="light",
                ignore_https_errors=True,
            )
            context.set_default_timeout(STEP_TIMEOUT_MS)
            page = context.new_page()
            log_in(page, demo, login)
            for theme in ("light", "dark"):
                page.emulate_media(color_scheme=theme)
                files += capture_views(page, out, theme)
            context.close()
        finally:
            browser.close()
    return files


def log_in(page: Page, demo: Demo, login: Login) -> None:
    """Log Alex in with the console form and confirm the step-up of the login."""
    page.goto(demo.url + "/console/")
    wait_for_console(page)
    fill(page, T["login_name"], login.name)
    press(page, T["password_and_code"])
    fill(page, T["password"], login.password)
    fill(page, T["code"], login.code())
    press(page, T["login"])
    step_up = page.get_by_role("button", name=T["stepup_password"], exact=True)
    live = page.get_by_role("button", name=VIEWS[0][0], exact=True)
    step_up.or_(live).first.wait_for()
    if step_up.is_visible():
        press(page, T["stepup_password"])
        fill(page, T["password"], login.password)
        fill(page, T["code"], login.code())
        press(page, T["confirm"])
    live.wait_for()


def wait_for_console(page: Page) -> None:
    """Wait until the Flutter view and its semantics tree are in the document."""
    page.wait_for_selector("flutter-view", state="attached")
    page.wait_for_function(
        "document.querySelectorAll('flt-semantics').length > 2", timeout=STEP_TIMEOUT_MS
    )


def fill(page: Page, label: str, value: str) -> None:
    """Type `value` into the text field labelled `label` and check that it took it."""
    field = page.get_by_role("textbox", name=label, exact=True)
    for _ in range(3):
        field.click()
        page.keyboard.press("ControlOrMeta+A")
        page.keyboard.press("Backspace")
        page.keyboard.type(value, delay=20)
        got = page.evaluate("() => document.activeElement ? document.activeElement.value : null")
        if got == value:
            return
    raise StepError(f"console: the field {label} did not take the typed text")


def press(page: Page, label: str) -> None:
    """Tap the button labelled `label`."""
    page.get_by_role("button", name=label, exact=True).click()


def settle(page: Page) -> None:
    """Wait until the console drew its frames and loaded the data of the view."""
    page.wait_for_load_state("networkidle", timeout=STEP_TIMEOUT_MS)
    page.wait_for_timeout(SETTLE_MS)
    page.evaluate("new Promise((r) => requestAnimationFrame(() => r(true)))")


def capture_views(page: Page, out: Path, theme: str) -> list[tuple[Path, str, str, str]]:
    """Open every view of VIEWS and TV mode and screenshot them in `theme`."""
    files = []
    for entry, view in VIEWS:
        press(page, entry)
        settle(page)
        path = out / f"{view}-{theme}.png"
        page.screenshot(path=str(path), animations="disabled", caret="hide")
        files.append((path, "image", view, theme))
        if view == "live":
            press(page, T["tv"])
            settle(page)
            path = out / f"tv-mode-{theme}.png"
            page.screenshot(path=str(path), animations="disabled", caret="hide")
            files.append((path, "image", "tv-mode", theme))
            page.keyboard.press("Escape")
            settle(page)
    if not files:
        raise StepError("console: no view captured")
    return files
