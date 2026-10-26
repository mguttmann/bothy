# SPDX-License-Identifier: Apache-2.0
"""DPoP authentication of one G7 pass: credentials from the broker, a fresh proof per request.

:class:`PassAuth` keeps the session of a pass (owner or plain user): it fetches credentials from
the broker, performs a step-up right away, and renews both before ``auth.step_up_max_age`` or
``auth.access_token_ttl`` runs out. It keeps the newest ``DPoP-Nonce`` and probes ``getMe`` for one
when none is known or it may be older than ``auth.dpop_nonce_ttl``. A ``401`` with
``session_revoked`` or ``unauthenticated`` (a generated call revoked the session) makes it fetch new
credentials. :func:`provider_class` wraps it as a Schemathesis auth provider (``get``/``set``).

The security scheme ``pushPreviewToken`` (``getPushPreview``, 13 6.4) takes no access token: for
each request :meth:`PassAuth.preview_headers` registers a push target of the pass's current device
(``registerPush``) and sends the fresh preview token as ``Authorization: DPoP <token>`` with a proof
of the key of the session that registered, its ``ath`` over the preview token.
"""

from __future__ import annotations

import json
import secrets
import time
import uuid
from collections.abc import Callable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from broker_client import BrokerClient, Credentials, RawResponse, http_request
from dpop import DPoPKey

# Problem codes that end a session for the pass: fetch new credentials and repeat.
RENEW_CODES = frozenset({"session_revoked", "unauthenticated", "token_expired"})
NONCE_CODE = "use_dpop_nonce"
STEP_UP_CODE = "step_up_required"
# Renew at this share of the shortest lifetime, so a session never expires mid-request.
RENEW_SHARE = 0.5
# registerPush, relative to the API base (06, 13 6.4).
PUSH_REGISTRATIONS_PATH = "/me/devices/current/push-registrations"
# The security scheme of getPushPreview in openapi.yaml.
PREVIEW_SCHEME = "pushPreviewToken"


class RegistrationError(RuntimeError):
    """registerPush did not create a registration for the pass."""


def problem_code(body: bytes) -> str | None:
    """Return the member ``code`` of a JSON problem body, or None."""
    try:
        data = json.loads(body)
    except ValueError:
        return None
    return str(data["code"]) if isinstance(data, dict) and "code" in data else None


def uuid7() -> str:
    """Return a fresh UUIDv7 (RFC 9562) in canonical lower-case form."""
    ms = time.time_ns() // 1_000_000
    rand = uuid.uuid4().int & ((1 << 74) - 1)
    value = (
        (ms & ((1 << 48) - 1)) << 80
        | 0x7 << 76
        | (rand >> 62) << 64
        | 0b10 << 62
        | (rand & ((1 << 62) - 1))
    )
    return str(uuid.UUID(int=value))


# Request headers that net/http judges itself before any handler: a bad Host or Content-Length
# gets a plain text 400, an unknown Transfer-Encoding 501, an Expect other than 100-continue 417.
TRANSPORT_HEADERS = frozenset({"host", "content-length", "transfer-encoding", "expect"})


def without_transport_headers(headers: Mapping[str, str]) -> dict[str, str]:
    """Return ``headers`` without the headers of ``TRANSPORT_HEADERS`` in any spelling.

    The negative mode of Schemathesis adds arbitrary headers, among them ``Host`` with a value
    that is no host or ``Content-Length`` with an empty value; ``net/http`` answers those itself
    before any handler, which says nothing about the contract. The transport sets the real ones.
    """
    return {k: v for k, v in headers.items() if k.lower() not in TRANSPORT_HEADERS}


@dataclass
class Session:
    """The current credentials of a pass with their key and age."""

    credentials: Credentials
    key: DPoPKey
    key_file: Path
    fetched_at: float


@dataclass(frozen=True)
class PushRegistration:
    """A push registration of the pass with its one-time preview token and its session."""

    registration_id: str
    preview_token: str
    session: Session


@dataclass
class PassAuth:
    """Authentication state of one pass against the server at ``api_base`` (``.../api/v1``)."""

    broker: BrokerClient
    kind: str
    api_base: str
    key_dir: Path
    step_up_max_age: float
    access_token_ttl: float
    nonce_ttl: float
    clock: Callable[[], float] = time.monotonic
    session: Session | None = None
    nonce: str | None = None
    nonce_at: float = 0.0
    renewals: int = field(default=0)
    # The APNs topic the server under test enables (push.apns.topics); registerPush needs it.
    push_app_id: str = ""

    @property
    def renew_after(self) -> float:
        """Seconds after which a session is renewed."""
        return RENEW_SHARE * min(self.step_up_max_age, self.access_token_ttl)

    def current(self) -> Session:
        """Return a session younger than :attr:`renew_after`, fetching and stepping up a new one."""
        if self.session is None or self.clock() - self.session.fetched_at >= self.renew_after:
            creds = self.broker.principal(self.kind)
            self.broker.step_up(creds.session_id)
            key = DPoPKey.from_jwk(creds.dpop_private_jwk)
            key_file = self.key_dir / f"{self.kind}-{creds.session_id}.jwk.json"
            key_file.write_text(json.dumps(creds.dpop_private_jwk), encoding="utf-8")
            key_file.chmod(0o600)
            self.session = Session(creds, key, key_file, self.clock())
            self.renewals += 1
        return self.session

    def invalidate(self) -> None:
        """Forget the session; the next request fetches new credentials."""
        self.session = None

    def step_up(self) -> None:
        """Perform a fresh step-up for the current session."""
        self.broker.step_up(self.current().credentials.session_id)

    def observe(self, status: int, headers: Mapping[str, str], body: bytes) -> str | None:
        """Take the nonce of an answer and return what the caller should do before repeating.

        Returns ``"nonce"`` for ``use_dpop_nonce``, ``"renew"`` for an ended session (the session
        is forgotten), ``"step_up"`` for ``step_up_required``, or None.
        """
        for name, value in headers.items():
            if name.lower() == "dpop-nonce" and value:
                self.nonce, self.nonce_at = value, self.clock()
        if status != 401:
            return None
        code = problem_code(body)
        if code == NONCE_CODE:
            return "nonce"
        if code == STEP_UP_CODE:
            return "step_up"
        if code in RENEW_CODES:
            self.invalidate()
            return "renew"
        return None

    def current_nonce(self) -> str | None:
        """Return the newest nonce, probing ``getMe`` when none is known or it may have expired."""
        if self.nonce is None or self.clock() - self.nonce_at >= self.nonce_ttl:
            url = self.api_base + "/me"
            session = self.current()
            token = session.credentials.access_token
            proof = session.key.proof("GET", url, access_token=token)
            resp = http_request("GET", url, {"Authorization": "DPoP " + token, "DPoP": proof})
            self.observe(resp.status, resp.headers, resp.body)
        return self.nonce

    def headers(self, method: str, url: str, *, authenticated: bool, proof: bool) -> dict[str, str]:
        """Return the auth headers of one request to ``url``.

        Args:
            method: HTTP method.
            url: full request URL.
            authenticated: send ``Authorization: DPoP <token>`` and a proof with ``ath``.
            proof: for an anonymous request, send a ``DPoP`` proof without ``ath`` (login flows).
        """
        session = self.current()
        if authenticated:
            token = session.credentials.access_token
            return {
                "Authorization": "DPoP " + token,
                "DPoP": session.key.proof(
                    method, url, access_token=token, nonce=self.current_nonce()
                ),
            }
        if proof:
            return {"DPoP": session.key.proof(method, url, nonce=self.current_nonce())}
        return {}

    def register_push(self) -> PushRegistration:
        """Register a fresh APNs target of the current device and return it.

        The device token is 32 random bytes as lower-case hex (the APNs form); the registration
        belongs to the session that sent the request, whose key must sign every proof of its
        preview token.

        Raises:
            RegistrationError: no push app ID is configured, or registerPush did not answer 201
                (the message names status and problem code, never a token).
        """
        if not self.push_app_id:
            raise RegistrationError("registerPush: no push app ID (BOTHY_CONTRACT_PUSH_APP_ID)")
        body = {"provider": "apns", "token": secrets.token_hex(32), "app_id": self.push_app_id}
        resp = self.send(
            "POST",
            self.api_base + PUSH_REGISTRATIONS_PATH,
            {"Content-Type": "application/json", "Idempotency-Key": uuid7()},
            json.dumps(body).encode(),
            authenticated=True,
        )
        # send renews before a request, never after a 201, so this is the registering session.
        session = self.session
        if resp.status != 201 or session is None:
            raise RegistrationError(
                f"registerPush answered {resp.status} {problem_code(resp.body)}"
            )
        data = json.loads(resp.body)
        return PushRegistration(str(data["id"]), str(data["preview_token"]), session)

    def preview_headers(self, method: str, url: str) -> dict[str, str]:
        """Return the headers of the scheme ``pushPreviewToken`` for one request to ``url``.

        Each call registers a new push target, so every request carries a fresh preview token:
        ``Authorization: DPoP <preview token>`` and a proof of the registering session's key with
        ``ath`` over the preview token and the newest nonce.
        """
        reg = self.register_push()
        token = reg.preview_token
        return {
            "Authorization": "DPoP " + token,
            "DPoP": reg.session.key.proof(
                method, url, access_token=token, nonce=self.current_nonce()
            ),
        }

    def send(
        self,
        method: str,
        url: str,
        headers: Mapping[str, str],
        body: bytes | None,
        *,
        authenticated: bool,
    ) -> RawResponse:
        """Send a raw request with fresh auth headers; repeat after nonce, renewal or step-up."""
        resp = http_request(
            method,
            url,
            {**headers, **self.headers(method, url, authenticated=authenticated, proof=True)},
            body,
        )
        action = self.observe(resp.status, resp.headers, resp.body)
        if action == "step_up":
            self.step_up()
        if action is not None:
            resp = http_request(
                method,
                url,
                {**headers, **self.headers(method, url, authenticated=authenticated, proof=True)},
                body,
            )
            self.observe(resp.status, resp.headers, resp.body)
        return resp


def provider_class(state: PassAuth, auth_headers: Callable[[Any], dict[str, str]]) -> type:
    """Return a Schemathesis auth provider class bound to ``state``.

    ``get`` returns the current session; ``set`` adds the headers ``auth_headers(case)`` returns
    (``Authorization: DPoP`` and a fresh ``DPoP`` proof).
    """

    class DPoPAuthProvider:
        """Schemathesis auth provider of one G7 pass."""

        def get(self, case: Any, ctx: Any) -> Session:
            """Return the current session of the pass."""
            return state.current()

        def set(self, case: Any, data: Session, ctx: Any) -> None:
            """Put the auth headers of ``case`` onto it."""
            case.headers = {**(case.headers or {}), **auth_headers(case)}

    return DPoPAuthProvider
