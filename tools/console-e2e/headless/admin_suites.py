# SPDX-License-Identifier: Apache-2.0
"""Phase 2 of the console E2E test: the patrol web suites of the console administration.

Brief M6-04 Design note 4. Runs inside the pinned Playwright image (service ``console-browser``
with ``E2E_MODE=admin``) against the api of the overlay ``compose.console-e2e-admin.yaml``, which
serves the bundle of ``apps/console/integration_test/admin/admin_bundle.dart`` as the console and
holds a step-up for 3 s. Chromium only, because only Chromium has the virtual authenticator the
step-up of every flow needs. Steps:

1. One browser context with the collectors of SEC-T05 (``console_headless.Session``), the virtual
   authenticator, clipboard permissions for the console origin, a listener for downloads and the
   binding ``__patrol__platformHandler`` of patrol's web protocol; then the console URL.
2. From the page, with the console cookie: Alex (``/out/console-login.json``) logs in with password
   and TOTP, does the step-up with password and TOTP and registers a passkey at once, so every
   later step-up uses the passkey (``ConsoleAdmin``).
3. The seed through the API as Alex (spaces, channels, Sam, a group, a grant, two agents) and
   Sam's waiting device: Sam's devices call the API without the cookie with DPoP proofs signed by
   WebCrypto keys of the page (``PageDevice``). The ids and names go to ``window.__e2eSeed``.
4. patrol's protocol: list the tests with ``__patrol__getTests``, then per test a fresh load of
   the bundle with the seed and the initialisation, and ``__patrol__runTest`` (up to 5 minutes).
   The suite of brief M6-08 asks for its manifest file with the platform action
   ``e2eSetManifestFile``: the runner answers the manifest of ``repo_bridge_manifest`` (events to
   the receiver ``webhook-receiver`` of the Compose network), and the suite sets it on the
   console's file input.

The result goes to ``/out/results/admin.json`` (per test its name, result and details, the
violations, console CSP errors, foreign origins and downloads; never a secret, code or token).
Exit code 0 when every test passed, the A6 export produced a CSV download and the session is
clean.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import secrets
import sys
import time
import traceback
import uuid
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlsplit

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
    assert_clean,
    console_fetch,
    dump_screen,
    launch,
    register_passkey,
    totp,
)
from playwright.sync_api import (
    ConsoleMessage,
    Download,
    Page,
    Playwright,
    sync_playwright,
)

API = "/api/v1"
RESULT_FILE = "admin.json"
# Bound of one patrol test (brief M6-04): the flows click through several screens with step-ups.
TEST_TIMEOUT_MS = 300_000
DETAILS_MAX = 2000
# patrol's own test that lists the tests on a device; it has no meaning on the web.
EXPLORER_TEST = "patrol_test_explorer"
PERMISSIONS = ["clipboard-read", "clipboard-write"]
# The lifecycle actions of patrol's web protocol that need no answer; everything else is either
# implemented in PlatformHandler or fails the run with its name.
LIFECYCLE_ACTIONS = frozenset({"startTest"})
TOTP_PERIOD = 30
# The receiver of the outgoing webhooks inside the Compose network (brief M6-08); the E2E
# configuration lists it in webhook.allowed_hosts.
RECEIVER_URL = "https://webhook-receiver:8443"


# ---- pure helpers (unit-tested in tools/console-e2e/tests) ---------------------------------


@dataclass(frozen=True)
class PatrolTest:
    """One test of ``__patrol__getTests``.

    The full name ``__patrol__runTest`` takes and the skip flag of the test or of a group around it.
    """

    name: str
    skip: bool


def flatten_tests(
    entry: Mapping[str, Any], parent: str = "", skip: bool = False
) -> list[PatrolTest]:
    """The tests of a patrol group tree in its order, as patrol's own web runner names them.

    A full name is the names of the enclosing groups and the test joined with a space; the root
    group usually has an empty name, which adds nothing (patrol web_runner/tests/setup.ts
    mapEntry). The internal test ``patrol_test_explorer`` is left out.

    Example: a root ``""`` with group ``a1_people_test`` holding test ``A1 create`` gives
    ``[PatrolTest("a1_people_test A1 create", False)]``.
    """
    name = str(entry.get("name") or "")
    full = f"{parent} {name}" if parent else name
    skipped = skip or bool(entry.get("skip"))
    out: list[PatrolTest] = []
    if entry.get("type") == "test" and name != EXPLORER_TEST:
        out.append(PatrolTest(full, skipped))
    for child in entry.get("entries") or []:
        out += flatten_tests(child, full, skipped)
    return out


def b64url(data: bytes) -> str:
    """base64url without padding (RFC 7515 2)."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def uuid7(now_ms: int | None = None) -> str:
    """A fresh UUIDv7 (RFC 9562 5.7), the form of device_id and Idempotency-Key in the contract."""
    ms = int(time.time() * 1000) if now_ms is None else now_ms
    raw = bytearray(ms.to_bytes(6, "big") + secrets.token_bytes(10))
    raw[6] = (raw[6] & 0x0F) | 0x70
    raw[8] = (raw[8] & 0x3F) | 0x80
    return str(uuid.UUID(bytes=bytes(raw)))


def request_htu(origin: str, path: str) -> str:
    """The htu of a REST proof.

    The public origin plus the request path without query and fragment (08 7.3; RESTHTU of the smoke
    probe), never the loopback address.
    """
    parts = urlsplit(path)
    if parts.scheme or parts.netloc or not parts.path.startswith("/"):
        raise ValueError("the request path must be absolute and without origin")
    return origin.rstrip("/") + parts.path


def proof_header(jwk: Mapping[str, str]) -> dict[str, Any]:
    """The JOSE header of a DPoP proof (RFC 9449 4.2) with only the public members of ``jwk``."""
    public = {k: jwk[k] for k in ("kty", "crv", "x", "y")}
    return {"typ": "dpop+jwt", "alg": "ES256", "jwk": public}


def proof_claims(
    method: str, htu: str, *, nonce: str, access_token: str, now: int, jti: str
) -> dict[str, Any]:
    """The claims of a DPoP proof as the server checks them (dpop.go of the smoke probe).

    Jti, htm, htu, iat; nonce when the server sent one; ath (base64url SHA-256 of the token) when
    the request carries a token.
    """
    claims: dict[str, Any] = {"jti": jti, "htm": method, "htu": htu, "iat": now}
    if nonce:
        claims["nonce"] = nonce
    if access_token:
        claims["ath"] = b64url(hashlib.sha256(access_token.encode()).digest())
    return claims


def totp_secret(uri: str) -> str:
    """The base32 secret (upper case, no padding) of the otpauth URI of setMyLocalCredential."""
    values = parse_qs(urlsplit(uri).query).get("secret") or [""]
    secret = values[0].upper().rstrip("=")
    if not secret:
        raise CheckError("the totp_secret_uri has no secret")
    return secret


def named(obj: Mapping[str, Any], name_key: str = "name") -> dict[str, str]:
    """``{"id", "name"}`` of an API object (``display_name`` for people and agents)."""
    return {"id": str(obj["id"]), "name": str(obj[name_key])}


SEED_SHAPE: dict[str, tuple[str, ...]] = {
    "owner": (),
    "sam": (),
    "pendingDevice": (),
    "spaces": ("support", "marketing", "engineering"),
    "channels": (
        "supportGeneral",
        "supportRequests",
        "marketingCampaigns",
        "marketingInternal",
        "engineeringBuilds",
    ),
    "agents": ("supportAgent", "texter"),
    "group": (),
}


def check_seed(seed: Mapping[str, Any]) -> None:
    """Checks the shape of ``window.__e2eSeed`` that the Dart tests read.

    Exactly the members of SEED_SHAPE, every inner object with string ``id`` and ``name``, the
    pending device also with its verification code in digits. Raises CheckError naming the member.
    """
    if set(seed) != set(SEED_SHAPE):
        raise CheckError(f"seed members {sorted(seed)} differ from {sorted(SEED_SHAPE)}")
    objects: list[tuple[str, Any]] = []
    for key, inner in SEED_SHAPE.items():
        if not inner:
            objects.append((key, seed[key]))
            continue
        if not isinstance(seed[key], dict) or set(seed[key]) != set(inner):
            raise CheckError(f"seed member {key} must hold exactly {list(inner)}")
        objects += [(f"{key}.{k}", seed[key][k]) for k in inner]
    for path, obj in objects:
        if not isinstance(obj, dict) or not all(
            isinstance(obj.get(k), str) and obj[k] for k in ("id", "name")
        ):
            raise CheckError(f"seed member {path} needs a string id and name")
    code = seed["pendingDevice"].get("code")
    if not isinstance(code, str) or not code.isdigit():
        raise CheckError("seed member pendingDevice needs the verification code in digits")


def repo_bridge_manifest() -> dict[str, Any]:
    """The manifest of the app "Repo Bridge" that the A7 suite installs (11 3.1, brief M6-08).

    White label, with the example texts of 11 3.3; the events go to the receiver of the Compose
    network. No commands and no components, which would need an ``interactions_url`` (11 3.2).
    """
    return {
        "schema_version": 1,
        "slug": "repo-bridge",
        "name": "Repo Bridge",
        "description": "Brings events from a code hosting service into a forum.",
        "publisher": "Example Org",
        "version": "1.2.0",
        "homepage_url": "https://bridge.example.com",
        "min_api_version": "0.2",
        "scopes": {
            "required": ["messages:write"],
            "optional": ["messages:read", "events:subscribe"],
        },
        "events": ["message.created"],
        "delivery": {"mode": "webhook", "events_url": RECEIVER_URL + "/hooks/events"},
        "requested_grants": [
            {
                "target_kind": "channel",
                "level": "write",
                "capabilities": [],
                "purpose": "Post build results",
            }
        ],
        "data_egress": (
            "Messages of the granted channel go to the receiver of the build service if "
            "messages:read is granted."
        ),
    }


# The manifest files a suite may ask for by name (platform action e2eSetManifestFile).
MANIFESTS: dict[str, Callable[[], dict[str, Any]]] = {"repo-bridge.json": repo_bridge_manifest}


def manifest_file(name: str) -> dict[str, str]:
    """The file ``name`` of MANIFESTS as the answer of ``e2eSetManifestFile``.

    The answer holds the name, the MIME type and the text (JSON). Raises CheckError for any other
    name, so a suite can never make the runner read a path.
    """
    build = MANIFESTS.get(name)
    if build is None:
        raise CheckError(f"no manifest file {name!r} in the runner")
    return {
        "name": name,
        "mimeType": "application/json",
        "text": json.dumps(build(), indent=2),
    }


@dataclass
class DownloadRecord:
    """A download of the page: the suggested file name and the first line of the file."""

    name: str
    first_line: str


def csv_export_seen(downloads: Sequence[DownloadRecord]) -> bool:
    """Whether the A6 export produced a CSV file whose header line names occurred_at."""
    return any(d.name.lower().endswith(".csv") and "occurred_at" in d.first_line for d in downloads)


def problem_code(answer: Mapping[str, Any]) -> str:
    """The ``code`` of a problem body (06 "Errors"), or "" for any other answer."""
    body = answer.get("json")
    return str(body.get("code") or "") if isinstance(body, dict) else ""


def problem_text(answer: Mapping[str, Any]) -> str:
    """Code and reason of a problem body for an error message.

    Never the rest of the body, which may echo a value.
    """
    body = answer.get("json")
    reason = str(body.get("reason") or "") if isinstance(body, dict) else ""
    return " ".join(p for p in (problem_code(answer), reason) if p)


# ---- page scripts ------------------------------------------------------------------------

# A request of the console session (cookie, CSRF header) with extra headers; returns the status,
# the JSON body and the ETag.
ADMIN_FETCH_JS = """async ([method, path, headers, body]) => {
  const h = Object.assign({'X-Console-Request': '1', 'Accept': 'application/json'}, headers);
  if (body !== null) h['Content-Type'] = 'application/json';
  const r = await fetch(path, {method, credentials: 'same-origin', headers: h,
    body: body === null ? undefined : JSON.stringify(body)});
  let json = null;
  try { json = await r.json(); } catch (e) { json = null; }
  return {status: r.status, json, etag: r.headers.get('ETag')};
}"""

# A request of a device of Sam: no cookie, the DPoP and Authorization headers of the caller;
# returns the status, the JSON body and the newest DPoP-Nonce.
DEVICE_FETCH_JS = """async ([method, path, headers, body]) => {
  const h = Object.assign({'Accept': 'application/json'}, headers);
  if (body !== null) h['Content-Type'] = 'application/json';
  const r = await fetch(path, {method, credentials: 'omit', headers: h,
    body: body === null ? undefined : JSON.stringify(body)});
  let json = null;
  try { json = await r.json(); } catch (e) { json = null; }
  return {status: r.status, json, nonce: r.headers.get('DPoP-Nonce')};
}"""

# A P-256 key pair whose private key never leaves the page (not extractable); returns the
# public JWK. The public key of a pair is always exportable.
NEW_KEY_JS = """async (name) => {
  const pair = await crypto.subtle.generateKey({name: 'ECDSA', namedCurve: 'P-256'}, false,
    ['sign']);
  window.__e2eKeys = window.__e2eKeys || {};
  window.__e2eKeys[name] = pair.privateKey;
  const jwk = await crypto.subtle.exportKey('jwk', pair.publicKey);
  return {kty: jwk.kty, crv: jwk.crv, x: jwk.x, y: jwk.y};
}"""

# The compact JWS of header and claims signed with the key ``name``; WebCrypto's ECDSA signature
# is already R || S of 32 bytes each, the form of ES256 (RFC 7518 3.4).
SIGN_JS = """async ([name, header, claims]) => {
  const enc = (bytes) => btoa(String.fromCharCode(...bytes))
    .replace(/\\+/g, '-').replace(/\\//g, '_').replace(/=+$/, '');
  const text = (obj) => enc(new TextEncoder().encode(JSON.stringify(obj)));
  const input = text(header) + '.' + text(claims);
  const sig = await crypto.subtle.sign({name: 'ECDSA', hash: 'SHA-256'}, window.__e2eKeys[name],
    new TextEncoder().encode(input));
  return input + '.' + enc(new Uint8Array(sig));
}"""

# navigator.credentials.get over the passkey options of a step-up challenge with
# userVerification required (08 6.2), and the assertion in the JSON form of the server.
GET_PASSKEY_JS = """async (options) => {
  const b64u = (s) => Uint8Array.from(
    atob(s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4)),
    (c) => c.charCodeAt(0));
  const enc = (buf) => btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\\+/g, '-').replace(/\\//g, '_').replace(/=+$/, '');
  const pk = Object.assign({}, options.publicKey || options);
  pk.challenge = b64u(pk.challenge);
  pk.allowCredentials = (pk.allowCredentials || []).map((c) => Object.assign({}, c,
    {id: b64u(c.id)}));
  pk.userVerification = 'required';
  const cred = await navigator.credentials.get({publicKey: pk});
  const response = {clientDataJSON: enc(cred.response.clientDataJSON),
    authenticatorData: enc(cred.response.authenticatorData),
    signature: enc(cred.response.signature)};
  if (cred.response.userHandle && cred.response.userHandle.byteLength > 0) {
    response.userHandle = enc(cred.response.userHandle);
  }
  return {id: cred.id, rawId: enc(cred.rawId), type: cred.type, response,
    clientExtensionResults: cred.getClientExtensionResults()};
}"""

# patrol's initialisation (web_runner/tests/initialise.ts).
INITIALISE_JS = """() => {
  if (!window.__patrol__onInitialised) return false;
  window.__patrol__onInitialised();
  return true;
}"""

# One patrol test, bounded so a hanging test fails with its name instead of blocking the run; a
# rejected run (a Dart exception, for example after an earlier test left the app broken) is a
# failure of this test, so the later tests still run and report.
RUN_TEST_JS = """async ([name, ms]) => Promise.race([
  window.__patrol__runTest(name).catch((e) => ({result: 'failure',
    details: 'runTest rejected: ' + String((e && e.error) || e)})),
  new Promise((resolve) => setTimeout(
    () => resolve({result: 'failure', details: 'timeout after ' + ms + ' ms'}), ms)),
])"""


# ---- API clients from the page -----------------------------------------------------------


def say(message: str) -> None:
    """One progress line of the admin suites on stderr."""
    print(f"console-headless: admin: {message}", file=sys.stderr, flush=True)


class ConsoleAdmin:
    """Alex's console session in the page: requests with the cookie and the CSRF header.

    A changing request takes a fresh Idempotency-Key (UUIDv7); on ``401 step_up_required`` it does
    one passkey step-up from the page and repeats once with the same key (Design note 2, the
    order of the console's step-up adapter).
    """

    def __init__(self, page: Page) -> None:
        """Keeps the page whose session the requests use."""
        self.page = page

    def fetch(
        self,
        method: str,
        path: str,
        body: object | None = None,
        headers: dict[str, str] | None = None,
    ) -> dict[str, Any]:
        """One request; returns status, json and etag."""
        answer: dict[str, Any] = self.page.evaluate(
            ADMIN_FETCH_JS, [method, API + path, headers or {}, body]
        )
        return answer

    def call(
        self,
        op: str,
        method: str,
        path: str,
        body: object | None = None,
        *,
        idempotent: bool = False,
        if_match: str | None = None,
    ) -> dict[str, Any]:
        """Runs operation ``op`` with the step-up retry.

        Returns the answer of a 2xx status and raises CheckError naming ``op``, the status and the
        problem code otherwise.
        """
        headers: dict[str, str] = {}
        if idempotent:
            headers["Idempotency-Key"] = uuid7()
        if if_match is not None:
            headers["If-Match"] = if_match
        answer = self.fetch(method, path, body, headers)
        if answer["status"] == 401 and problem_code(answer) == "step_up_required":
            self.step_up_with_passkey()
            answer = self.fetch(method, path, body, headers)
        if not 200 <= answer["status"] < 300:
            raise CheckError(f"{op} answered {answer['status']} {problem_text(answer)}")
        return answer

    def step_up_with_passkey(self) -> None:
        """One passkey step-up of the console session.

        createStepUpChallenge, the passkey with userVerification required, then
        completeStepUpWithPasskey; the server rotates the cookie.
        """
        challenge = self.fetch("POST", "/auth/step-up/challenge")
        data = challenge["json"]
        if challenge["status"] != 200 or not isinstance(data, dict):
            raise CheckError(f"createStepUpChallenge answered {challenge['status']}")
        if not data.get("passkey_options"):
            raise CheckError("the step-up challenge offers no passkey")
        credential = self.page.evaluate(GET_PASSKEY_JS, data["passkey_options"])
        done = self.fetch(
            "POST",
            "/auth/step-up/passkey",
            {"challenge_id": data["challenge_id"], "credential": credential},
        )
        if done["status"] != 200:
            raise CheckError(f"completeStepUpWithPasskey answered {done['status']}")

    def etag(self, op: str, path: str) -> str:
        """The If-Match value of the current version of ``path``.

        The ETag of a fresh read with operation ``op`` (06 3: the server names the version, the
        client treats it as opaque).
        """
        answer = self.call(op, "GET", path)
        tag = answer.get("etag")
        if not tag:
            raise CheckError(f"{op} answered without an ETag")
        return str(tag)


def log_in(page: Page, login: Login) -> None:
    """Alex's console login with password and TOTP.

    Then the step-up with password and TOTP and at once the passkey registration, which needs the
    step-up of the last 3 s.
    """
    done = console_fetch(
        page,
        "POST",
        API + "/auth/password/login",
        {
            "client": "console",
            "login_name": login.name,
            "password": login.password,
            "totp_code": login.code(),
        },
    )
    if done["status"] != 200:
        raise CheckError(f"loginWithPassword (console) answered {done['status']}")
    # The TOTP wait comes before the challenge, so the challenge and the step-up stay within
    # their windows.
    code = login.code()
    challenge = console_fetch(page, "POST", API + "/auth/step-up/challenge")
    if challenge["status"] != 200 or not isinstance(challenge["json"], dict):
        raise CheckError(f"createStepUpChallenge answered {challenge['status']}")
    step = console_fetch(
        page,
        "POST",
        API + "/auth/step-up/password",
        {
            "challenge_id": challenge["json"]["challenge_id"],
            "password": login.password,
            "totp_code": code,
        },
    )
    if step["status"] != 200:
        raise CheckError(f"stepUpWithPassword answered {step['status']}")
    register_passkey(page)


@dataclass
class PageDevice:
    """A device of Sam whose DPoP key lives in the page (WebCrypto, not extractable).

    Requests go without the cookie; each carries a fresh proof with the newest DPoP-Nonce of the
    server and, with a token, ``Authorization: DPoP`` and ath. A ``401 use_dpop_nonce`` is
    repeated once with the new nonce (06 3.1). The token stays in this object and is never
    printed.
    """

    page: Page
    key: str
    jwk: dict[str, str] = field(default_factory=dict)
    token: str = ""
    nonce: str = ""

    @classmethod
    def create(cls, page: Page, key: str) -> PageDevice:
        """A device with a new key pair ``key`` in the page."""
        return cls(page=page, key=key, jwk=new_key(page, key))

    def registration(self, name: str, step_up_jwk: Mapping[str, str] | None) -> dict[str, Any]:
        """The DeviceRegistration of this device (a new device_id) with an optional step-up key."""
        reg: dict[str, Any] = {
            "device_id": uuid7(),
            "name": name,
            "platform": "web",
            "alg": "ES256",
            "public_jwk": dict(self.jwk),
        }
        if step_up_jwk is not None:
            reg["step_up_key"] = {"alg": "ES256", "public_jwk": dict(step_up_jwk)}
        return reg

    def call(
        self, op: str, method: str, path: str, body: object | None = None, *, want: int = 200
    ) -> dict[str, Any]:
        """Runs ``op``; returns the JSON body of status ``want`` or raises CheckError."""
        for attempt in range(2):
            claims = proof_claims(
                method,
                request_htu(ORIGIN, API + path),
                nonce=self.nonce,
                access_token=self.token,
                now=int(time.time()),
                jti=b64url(secrets.token_bytes(16)),
            )
            proof = self.page.evaluate(SIGN_JS, [self.key, proof_header(self.jwk), claims])
            headers = {"DPoP": proof}
            if self.token:
                headers["Authorization"] = "DPoP " + self.token
            answer: dict[str, Any] = self.page.evaluate(
                DEVICE_FETCH_JS, [method, API + path, headers, body]
            )
            if answer.get("nonce"):
                self.nonce = str(answer["nonce"])
            if (
                attempt == 0
                and answer["status"] == 401
                and problem_code(answer) == "use_dpop_nonce"
            ):
                continue
            if answer["status"] != want or not isinstance(answer["json"], dict):
                raise CheckError(f"{op} answered {answer['status']} {problem_text(answer)}")
            body_json: dict[str, Any] = answer["json"]
            return body_json
        raise CheckError(f"{op} kept asking for a DPoP nonce")  # pragma: no cover


def new_key(page: Page, name: str) -> dict[str, str]:
    """A new P-256 key pair ``name`` in the page; returns its public JWK."""
    jwk: dict[str, str] = page.evaluate(NEW_KEY_JS, name)
    return jwk


def fresh_code(secret: str, used: int) -> tuple[str, int]:
    """The TOTP code of the first step after ``used`` (a step is valid once, 08 2.5)."""
    while int(time.time()) // TOTP_PERIOD <= used:
        time.sleep(1)
    step = int(time.time()) // TOTP_PERIOD
    return totp(secret, step), step


def seed_sam_devices(page: Page, enrollment_code: str) -> dict[str, str]:
    """Sam's first device redeems the enrollment code, sets a random password and enrolls TOTP.

    Then Sam logs in as an app with a second device, which waits for approval. Returns the pending
    device as ``{"id", "name", "code"}`` with its verification code (getDevice as that device, 08
    5.2).
    """
    first = PageDevice.create(page, "sam-first")
    step_up = new_key(page, "sam-first-step-up")
    redeemed = first.call(
        "redeemEnrollmentCode",
        "POST",
        "/auth/enroll/redeem",
        {
            "enrollment_code": enrollment_code,
            "device": first.registration("Sam phone", step_up),
        },
        want=201,
    )
    first.token = str(redeemed["access_token"])
    password = secrets.token_hex(16) + "-Aa1!"
    enrolled = first.call(
        "setMyLocalCredential",
        "PUT",
        "/me/local-credential",
        {"password": password, "totp_enroll": True},
    )
    secret = totp_secret(str(enrolled.get("totp_secret_uri") or ""))
    code, used = fresh_code(secret, 0)
    confirmed = first.call(
        "setMyLocalCredential", "PUT", "/me/local-credential", {"totp_code": code}
    )
    if confirmed.get("totp_enabled") is not True:
        raise CheckError("the TOTP enrollment of Sam was not confirmed")
    second = PageDevice.create(page, "sam-second")
    code, _ = fresh_code(secret, used)
    name = "Sam laptop"
    login = second.call(
        "loginWithPassword (app)",
        "POST",
        "/auth/password/login",
        {
            "client": "app",
            "login_name": "sam",
            "password": password,
            "totp_code": code,
            "device": second.registration(name, None),
        },
        want=201,
    )
    second.token = str(login["access_token"])
    device_id = str(login["device"]["id"])
    device = second.call("getDevice", "GET", f"/devices/{device_id}")
    if device.get("state") != "pending" or not str(device.get("verification_code") or "").isdigit():
        raise CheckError(f"Sam's second device is {device.get('state')} without a code")
    return {"id": device_id, "name": name, "code": str(device["verification_code"])}


def seed(page: Page, admin: ConsoleAdmin) -> dict[str, Any]:
    """The instance of the flows through the API as Alex.

    Returns the document of ``window.__e2eSeed`` (SEED_SHAPE).
    """
    me = admin.call("getConsoleSession", "GET", "/console/session")["json"]
    owner = {"id": str(me["user_id"]), "name": str(me["display_name"])}
    spaces: dict[str, dict[str, Any]] = {}
    channels: dict[str, dict[str, Any]] = {}
    layout = {
        "support": ("Support", {"supportGeneral": "general", "supportRequests": "requests"}),
        "marketing": (
            "Marketing",
            {"marketingCampaigns": "Kampagnen", "marketingInternal": "Intern"},
        ),
        "engineering": ("Engineering", {"engineeringBuilds": "builds"}),
    }
    for key, (space_name, space_channels) in layout.items():
        spaces[key] = admin.call(
            "createSpace", "POST", "/spaces", {"name": space_name}, idempotent=True
        )
        for ckey, cname in space_channels.items():
            channels[ckey] = admin.call(
                "createChannel",
                "POST",
                f"/spaces/{spaces[key]['json']['id']}/channels",
                {"kind": "text", "name": cname},
                idempotent=True,
            )
    created = admin.call(
        "createUser",
        "POST",
        "/users",
        {"handle": "sam", "display_name": "Sam", "locale": "en", "local_account": True},
        idempotent=True,
    )["json"]
    sam = named(created["user"], "display_name")
    group = admin.call("createGroup", "POST", "/groups", {"name": "Support Team"}, idempotent=True)[
        "json"
    ]
    admin.call(
        "addGroupMember",
        "POST",
        f"/groups/{group['id']}/members",
        {"user_id": sam["id"]},
        idempotent=True,
    )
    support_id = spaces["support"]["json"]["id"]
    admin.call(
        "createGrant",
        "POST",
        "/grants",
        {
            "subject_kind": "user",
            "subject_id": sam["id"],
            "target_kind": "space",
            "target_id": support_id,
            "level": "write",
            "reason": "Sam works in support",
        },
        idempotent=True,
    )
    support_agent = admin.call(
        "createAgent",
        "POST",
        "/agents",
        {"handle": "support-agent", "display_name": "Support Agent", "adapter_kind": "hermes"},
        idempotent=True,
    )
    agent_id = support_agent["json"]["id"]
    admin.call(
        "setAgentBillingMode",
        "PUT",
        f"/agents/{agent_id}/billing-mode",
        {
            "billing_mode": "personal",
            "credential_holder_id": owner["id"],
            "reason": "Runs on the subscription of Alex",
        },
        if_match=admin.etag("getAgent", f"/agents/{agent_id}"),
    )
    admin.call(
        "setSpaceOperator",
        "PUT",
        f"/spaces/{support_id}/operator",
        {"mode": "agent", "agent_id": agent_id, "reason": "Support Agent answers in Support"},
        if_match=admin.etag("getSpace", f"/spaces/{support_id}"),
    )
    texter = admin.call(
        "createAgent",
        "POST",
        "/agents",
        {"handle": "texter", "display_name": "Texter", "adapter_kind": "hermes"},
        idempotent=True,
    )
    if texter["json"]["billing_mode"] != "organization":
        texter = admin.call(
            "setAgentBillingMode",
            "PUT",
            f"/agents/{texter['json']['id']}/billing-mode",
            {"billing_mode": "organization", "reason": "Texter runs for the organization"},
            if_match=admin.etag("getAgent", f"/agents/{texter['json']['id']}"),
        )
    for ckey in ("marketingCampaigns", "marketingInternal"):
        channel_id = channels[ckey]["json"]["id"]
        admin.call(
            "setChannelOperator",
            "PUT",
            f"/channels/{channel_id}/operator",
            {"mode": "agent", "agent_id": texter["json"]["id"], "reason": "Texter writes here"},
            if_match=admin.etag("getChannel", f"/channels/{channel_id}"),
        )
    enrollment = str(created["enrollment"]["code"])
    pending = seed_sam_devices(page, enrollment)
    return {
        "owner": owner,
        "sam": sam,
        "pendingDevice": pending,
        "spaces": {k: named(v["json"]) for k, v in spaces.items()},
        "channels": {k: named(v["json"]) for k, v in channels.items()},
        "agents": {
            "supportAgent": named(support_agent["json"], "display_name"),
            "texter": named(texter["json"], "display_name"),
        },
        "group": named(group),
    }


# ---- patrol's platform handler and the run -----------------------------------------------


class PlatformHandler:
    """The binding ``__patrol__platformHandler`` of patrol's web protocol.

    The Dart side calls it with ``{"action", "params"}``. Every action is logged; the lifecycle
    actions answer None, the clipboard, download and permission actions are answered like
    patrol's own web runner (web_runner/tests/actions), ``e2eSetManifestFile`` answers the manifest
    ``params.name`` (name, MIME type, text) for the Dart side to set on the console's file input
    (brief M6-08), and an unknown action raises in the
    page and is recorded, which fails the run with its name.
    """

    def __init__(self, page: Page, log: Callable[[str], None] = say) -> None:
        """Keeps the page and the log and starts with no recorded action."""
        self.page = page
        self.log = log
        self.downloads: list[str] = []
        self.unknown: list[str] = []
        self.actions: list[str] = []

    def __call__(self, source: object, request: Mapping[str, Any]) -> object:
        """Answers one platform action of the Dart side, as described on the class."""
        del source
        action = str(request.get("action") or "")
        params = request.get("params") or {}
        self.actions.append(action)
        self.log(f"platform action {action}")
        if action in LIFECYCLE_ACTIONS:
            self.downloads.clear()
            return None
        if action == "verifyFileDownloads":
            return list(self.downloads)
        if action == "getClipboard":
            return self.page.evaluate("() => navigator.clipboard.readText()")
        if action == "setClipboard":
            self.page.evaluate("(t) => navigator.clipboard.writeText(t)", str(params.get("text")))
            return None
        if action == "grantPermissions":
            self.page.context.grant_permissions(
                list(params.get("permissions") or []), origin=str(params.get("origin") or ORIGIN)
            )
            return None
        if action in ("enableDarkMode", "disableDarkMode"):
            self.page.emulate_media(color_scheme="dark" if action == "enableDarkMode" else "light")
            return None
        if action == "e2eSetManifestFile":
            # Answer only: the binding runs inside the pending page.evaluate of the test, where a
            # nested call of the sync API on the page never returns. The Dart side sets the file
            # on the console's input (support.dart chooseManifest).
            return manifest_file(str(params.get("name") or ""))
        self.unknown.append(action)
        raise CheckError(f"patrol platform action {action} is not supported by the runner")


def record_download(
    handler: PlatformHandler, downloads: list[Download]
) -> Callable[[Download], None]:
    """The page's download listener.

    Keeps the Download for the check after the run and its suggested name for verifyFileDownloads.
    """

    def on_download(download: Download) -> None:
        handler.downloads.append(download.suggested_filename)
        downloads.append(download)

    return on_download


def first_line(download: Download) -> str:
    """The first line of a finished download, read through Playwright's download API."""
    path = download.path()
    with Path(path).open(encoding="utf-8", errors="replace") as f:
        return f.readline().rstrip("\r\n")[:500]


def open_bundle(page: Page, seed_json: str) -> None:
    """Loads the bundle afresh, sets ``window.__e2eSeed`` and runs patrol's initialisation.

    The Dart side runs every test of the bundle once per page load and executes only the one
    requested through ``__patrol__runTest``; the others pass through without their body. So each
    test needs its own load, as patrol's web runner does (web_runner/tests/test.spec.ts). The
    seed is set before the initialisation, which is what the Dart side waits for.
    """
    page.goto(CONSOLE)
    page.evaluate("(s) => { window.__e2eSeed = s; }", seed_json)
    page.evaluate("window.__patrol__isInitialised = true")
    page.wait_for_function(INITIALISE_JS, timeout=STEP_TIMEOUT_MS)
    page.wait_for_function("() => !!window.__patrol__runTest", timeout=TEST_TIMEOUT_MS)


def run_tests(page: Page, seed_json: str) -> list[dict[str, Any]]:
    """Patrol's protocol.

    List the tests, then run each in order on its own page load; returns the results.
    """
    open_bundle(page, seed_json)
    tree = page.evaluate("() => window.__patrol__getTests()")
    tests = flatten_tests(tree["group"])
    results: list[dict[str, Any]] = []
    for index, test in enumerate(tests):
        if test.skip:
            results.append({"name": test.name, "result": "skipped", "details": None})
            say(f"{test.name}: skipped")
            continue
        if index > 0:
            open_bundle(page, seed_json)
        say(f"{test.name}: running")
        outcome = page.evaluate(RUN_TEST_JS, [test.name, TEST_TIMEOUT_MS])
        details = outcome.get("details") if isinstance(outcome, dict) else str(outcome)
        result = outcome.get("result") if isinstance(outcome, dict) else "failure"
        results.append(
            {
                "name": test.name,
                "result": result,
                "details": None if details is None else str(details)[:DETAILS_MAX],
            }
        )
        say(f"{test.name}: {result}")
        if result != "success":
            # patrol often reports a failure without details; the picture of the page is the
            # diagnosis (results/, copied with E2E_ARTIFACTS).
            shot = RESULTS / f"admin-{len(results):02d}-failure.png"
            page.screenshot(path=str(shot), full_page=True)
            results[-1]["screenshot"] = shot.name
    return results


def judge(
    tests: Sequence[Mapping[str, Any]], unknown: Sequence[str], records: Sequence[DownloadRecord]
) -> None:
    """Judges the run of the patrol suites.

    Raises CheckError when no test ran, an unsupported platform action was called, a test did not
    pass (a skipped one included) or the A6 export produced no CSV download.
    """
    if not tests:
        raise CheckError("the bundle lists no patrol test")
    if unknown:
        raise CheckError(f"unsupported patrol platform actions: {sorted(set(unknown))}")
    failures = [t["name"] for t in tests if t["result"] != "success"]
    if failures:
        raise CheckError(f"patrol tests did not pass: {failures}")
    if not csv_export_seen(records):
        raise CheckError("no CSV download with the header occurred_at (A6 export)")


def run_admin(pw: Playwright, login: Login, spki: str) -> dict[str, Any]:
    """The whole phase in Chromium; returns the result and raises CheckError on a failure."""
    browser = launch(pw, "chromium", spki)
    result: dict[str, Any] = {"browser": "chromium", "version": browser.version}
    session = Session(browser, "chromium", spki)
    page = session.page
    handler = PlatformHandler(page)
    downloads: list[Download] = []
    try:
        session.context.grant_permissions(PERMISSIONS, origin=ORIGIN)
        session.context.expose_binding("__patrol__platformHandler", handler)
        page.on("console", console_log(result.setdefault("console", [])))
        page.on("download", record_download(handler, downloads))
        add_virtual_authenticator(session)
        page.goto(CONSOLE)
        say("log in as the owner, step-up, passkey")
        log_in(page, login)
        admin = ConsoleAdmin(page)
        # The seed is usually done within the 3 s of the first step-up; one passkey step-up here
        # proves the path the seed and the flows rely on later.
        admin.step_up_with_passkey()
        say("seed")
        document = seed(page, admin)
        check_seed(document)
        say("patrol suites")
        tests = run_tests(page, json.dumps(document))
        result["tests"] = tests
        result["platform_actions"] = sorted(set(handler.actions))
        records = [DownloadRecord(d.suggested_filename, first_line(d)) for d in downloads]
        result["downloads"] = [{"name": r.name, "first_line": r.first_line} for r in records]
        assert_clean(session, result)
        judge(tests, handler.unknown, records)
        result["ok"] = True
    except Exception as e:
        dump_screen(page)
        result["ok"] = False
        result["error"] = f"{type(e).__name__}: {e}"[:DETAILS_MAX]
        result.setdefault("violations", session.violations())
        result.setdefault("console_errors", session.console_errors)
        result.setdefault("foreign_origins", session.foreign())
        raise
    finally:
        session.context.close()
        browser.close()
        (RESULTS / RESULT_FILE).write_text(json.dumps(result, indent=2), encoding="utf-8")
    return result


CONSOLE_KEEP = 400
CONSOLE_LINE_MAX = 500


def console_log(lines: list[str]) -> Callable[[ConsoleMessage], None]:
    """The page's console listener.

    Patrol's step log (lines starting with PATROL_LOG) goes to stderr; every other line (the output
    of flutter_test, for example the reason of a failed expectation) goes to ``lines``, the member
    ``console`` of admin.json, never to stderr, cut to the last CONSOLE_KEEP lines of
    CONSOLE_LINE_MAX characters.
    """

    def on_console(msg: ConsoleMessage) -> None:
        if msg.text.startswith("PATROL_LOG"):
            print(msg.text, file=sys.stderr, flush=True)
            return
        lines.append(f"{msg.type}: {msg.text}"[:CONSOLE_LINE_MAX])
        del lines[:-CONSOLE_KEEP]

    return on_console


def main() -> int:
    """Runs phase 2 and returns 0 when it passed."""
    RESULTS.mkdir(parents=True, exist_ok=True)
    login = Login(OUT / "console-login.json")
    # Phase 1 used TOTP steps the file does not know; the next code must come from a later step.
    login.skip_current_step()
    spki = os.environ.get("E2E_SPKI", "")
    with sync_playwright() as pw:
        try:
            run_admin(pw, login, spki)
        except Exception as e:
            say(f"FAIL {type(e).__name__}: {e}"[:2000])
            traceback.print_exc(limit=3)
            return 1
    say("ok")
    return 0
