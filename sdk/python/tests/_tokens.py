# SPDX-License-Identifier: Apache-2.0
"""Test-only issuer of invocation tokens: Ed25519 keys by kid, a JWKS and signed tokens.

It plays the server side of the M2-06 reference tests (refverifier_test.go): it signs whatever
header and payload a case needs, so a test can build every malformed variant. Nothing in
``src`` signs invocation tokens.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
from collections.abc import Callable, Mapping
from datetime import datetime
from typing import Any
from uuid import UUID, uuid4

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from bothy_sdk.auth import Secret
from bothy_sdk.ids import uuid7

TYP = "agent-invocation+jwt"


def b64(data: bytes) -> str:
    """Base64url without padding."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def unb64(text: str) -> bytes:
    """Decode base64url without padding."""
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


def public_bytes(key: Ed25519PrivateKey) -> bytes:
    """The raw 32 public key bytes of ``key``."""
    return key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)


def sign_ed(header: str, payload: bytes, key: Ed25519PrivateKey) -> str:
    """A compact JWS of ``header`` (as given) and ``payload`` with an Ed25519 signature."""
    signing_input = b64(header.encode()) + "." + b64(payload)
    return signing_input + "." + b64(key.sign(signing_input.encode()))


def sign_hs256_with_public_key(header: str, payload: bytes, key: Ed25519PrivateKey) -> str:
    """Algorithm confusion: HS256 keyed with the bytes of the published public key."""
    signing_input = b64(header.encode()) + "." + b64(payload)
    mac = hmac.new(public_bytes(key), signing_input.encode(), hashlib.sha256).digest()
    return signing_input + "." + b64(mac)


def header(kid: str, alg: str | None = "EdDSA", typ: str | None = TYP) -> str:
    """A header JSON with ``kid``; ``None`` leaves ``alg`` or ``typ`` out."""
    h: dict[str, Any] = {"kid": kid}
    if alg is not None:
        h["alg"] = alg
    if typ is not None:
        h["typ"] = typ
    return json.dumps(h, separators=(",", ":"))


def payload_of(token: str) -> dict[str, Any]:
    """The decoded payload of a compact token."""
    loaded: dict[str, Any] = json.loads(unb64(token.split(".")[1]))
    return loaded


class Issuer:
    """The server side: keys by kid, the active kid, a JWKS and grants turned into tokens.

    Args:
        now: Returns the current wall time (the clock of the test).
        instance_id: ``iss``.
        agent_id: ``aud``.
        ttl: Seconds from ``iat`` to ``exp`` (``agent.invocation_token_ttl`` of the tests).
    """

    def __init__(
        self, now: Callable[[], datetime], instance_id: UUID, agent_id: UUID, ttl: int = 60
    ) -> None:
        """Create the issuer with one active key."""
        self._now = now
        self.instance_id = instance_id
        self.agent_id = agent_id
        self.ttl = ttl
        self.keys: dict[str, Ed25519PrivateKey] = {}
        self.published: set[str] = set()
        self.active = self.add_key()
        self.grant: dict[str, Any] = {
            "sub": str(uuid4()),
            "trigger_kind": "message",
            "run_id": str(uuid7()),
            "message_id": str(uuid7()),
            "thread_id": str(uuid7()),
            "channel_id": str(uuid7()),
            "space_id": str(uuid7()),
            "level": "trigger",
            "billing_mode": "organization",
            "tool_class": "scoped",
            "permission_version": 4,
        }

    def add_key(self, *, active: bool = True, published: bool = True) -> str:
        """Create a key; ``active`` makes it the signing key; returns its kid."""
        key = Ed25519PrivateKey.generate()
        kid = hashlib.sha256(public_bytes(key)).hexdigest()
        self.keys[kid] = key
        if published:
            self.published.add(kid)
        if active:
            self.active = kid
        return kid

    def jwks(self) -> dict[str, Any]:
        """The JWKS body of the published keys."""
        return {
            "keys": [
                {
                    "kty": "OKP",
                    "crv": "Ed25519",
                    "kid": kid,
                    "alg": "EdDSA",
                    "use": "sig",
                    "x": b64(public_bytes(self.keys[kid])),
                }
                for kid in sorted(self.published)
            ]
        }

    def claims(self, **changes: Any) -> dict[str, Any]:
        """The claims of a fresh token for the grant with ``changes`` applied (None removes)."""
        iat = int(self._now().timestamp())
        c: dict[str, Any] = {
            "iss": str(self.instance_id),
            "aud": str(self.agent_id),
            "jti": str(uuid7()),
            "iat": iat,
            "exp": iat + self.ttl,
            **self.grant,
        }
        for name, value in changes.items():
            if value is None:
                c.pop(name, None)
            else:
                c[name] = value
        return c

    def raw(self, claims: Mapping[str, Any] | None = None, *, kid: str | None = None) -> str:
        """A token text signed with the key ``kid`` (default the active one)."""
        kid = kid or self.active
        body = json.dumps(dict(claims or self.claims()), separators=(",", ":")).encode()
        return sign_ed(header(kid), body, self.keys[kid])

    def token(self, **changes: Any) -> Secret:
        """A valid token of the grant, with ``changes`` applied to the claims."""
        return Secret(self.raw(self.claims(**changes)))

    def resign(self, token: str, change: Callable[[dict[str, Any]], None]) -> str:
        """``token`` with its payload changed by ``change``, signed again with the active key."""
        claims = payload_of(token)
        change(claims)
        return self.raw(claims)


class Jwks:
    """An in-test JWKS fetcher that counts its calls.

    Args:
        issuer: The issuer whose published keys it serves.
        max_age: The max-age of ``Cache-Control``; ``None`` when absent.
    """

    def __init__(self, issuer: Issuer, max_age: float | None) -> None:
        """Serve ``issuer``'s JWKS."""
        self.issuer = issuer
        self.max_age = max_age
        self.calls = 0
        self.extra: list[dict[str, Any]] = []

    async def __call__(self) -> tuple[Mapping[str, Any], float | None]:
        """Return the JWKS body and the max-age."""
        self.calls += 1
        body = self.issuer.jwks()
        body["keys"] = [*self.extra, *body["keys"]]
        return body, self.max_age
