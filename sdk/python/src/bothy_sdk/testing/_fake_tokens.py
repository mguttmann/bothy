# SPDX-License-Identifier: Apache-2.0
"""Invocation tokens of the FakeServer: issuer, server-side check and forgeries (private).

The issuer follows M2-06 (docs/spec/05-actions.md 5, docs/spec/09-agents.md 4.5): an Ed25519
JWS with ``alg = EdDSA``, ``typ = agent-invocation+jwt`` and the RFC 7638 thumbprint as ``kid``.
The check is the server's ``Verify``: the first failing rule gives ``invalid``; ``expired`` only
when every other rule holds.
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from datetime import datetime
from typing import Any
from uuid import UUID

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from bothy_sdk.auth import Secret
from bothy_sdk.ids import uuid7
from bothy_sdk.testing._fake_base import b64url, b64url_decode, compact_json, strict_object

TYP = "agent-invocation+jwt"
TRIGGER_OBJECT = {
    "message": "message_id",
    "followup": "message_id",
    "interaction": "interaction_id",
    "command": "command_id",
    "autocomplete": "command_id",
}
_REQUIRED_STRINGS = (
    "iss",
    "aud",
    "sub",
    "jti",
    "trigger_kind",
    "thread_id",
    "channel_id",
    "space_id",
    "level",
    "billing_mode",
    "tool_class",
)
_ID_CLAIMS = (
    "sub",
    "jti",
    "run_id",
    "message_id",
    "interaction_id",
    "command_id",
    "thread_id",
    "channel_id",
    "space_id",
)
_ENUMS = {
    "level": {"read", "write", "trigger"},
    "billing_mode": {"personal", "organization"},
    "tool_class": {"chat", "scoped", "host"},
    "trigger_kind": set(TRIGGER_OBJECT),
}
_FORBIDDEN_HEADER = {"crit", "jku", "x5u", "x5c", "zip"}
FORGE_VARIANTS = ("bad_signature", "alg_hs256", "typ_access", "aud_other_agent")


@dataclass(frozen=True)
class Checked:
    """The verdict of a token check and the claims when the token decoded."""

    verdict: str  # valid | expired | invalid
    claims: dict[str, Any] | None = None


def _is_v7(text: object) -> bool:
    if not isinstance(text, str):
        return False
    try:
        parsed = UUID(text)
    except ValueError:
        return False
    return str(parsed) == text and parsed.version == 7


def _is_int(value: object) -> bool:
    return isinstance(value, int) and not isinstance(value, bool)


def shape_ok(claims: dict[str, Any]) -> bool:  # noqa: PLR0911 - one rule per return
    """The claims shape of the server's check (required members, types, IDs, trigger table)."""
    if any(not isinstance(claims.get(n), str) or not claims[n] for n in _REQUIRED_STRINGS):
        return False
    if not all(_is_int(claims.get(n)) for n in ("iat", "exp", "permission_version")):
        return False
    for name in _ID_CLAIMS:
        if name in claims and not _is_v7(claims[name]):
            return False
    if any(claims[n] not in values for n, values in _ENUMS.items()):
        return False
    kind = claims["trigger_kind"]
    present = [n for n in ("message_id", "interaction_id", "command_id") if n in claims]
    if present != [TRIGGER_OBJECT[kind]]:
        return False
    if ("run_id" in claims) == (kind == "autocomplete"):
        return False
    return bool(claims["exp"] > claims["iat"] and claims["permission_version"] >= 0)


class Issuer:
    """The instance's invocation key: issues, checks and forges tokens.

    Args:
        instance_id: The ``iss`` of every token.
    """

    def __init__(self, instance_id: UUID) -> None:
        self.instance_id = instance_id
        self._key = Ed25519PrivateKey.generate()
        raw = self._key.public_key().public_bytes(
            serialization.Encoding.Raw, serialization.PublicFormat.Raw
        )
        self._x = b64url(raw)
        canonical = json.dumps(
            {"crv": "Ed25519", "kty": "OKP", "x": self._x}, separators=(",", ":"), sort_keys=True
        )
        self.kid = b64url(hashlib.sha256(canonical.encode()).digest())

    def jwks(self) -> dict[str, Any]:
        """The JWKS body with the one active key."""
        return {
            "keys": [
                {
                    "kty": "OKP",
                    "crv": "Ed25519",
                    "x": self._x,
                    "kid": self.kid,
                    "alg": "EdDSA",
                    "use": "sig",
                }
            ]
        }

    def _sign(self, header: dict[str, Any], claims: dict[str, Any], key: Any = None) -> str:
        signing_input = b64url(compact_json(header)) + "." + b64url(compact_json(claims))
        signer = key or self._key
        return signing_input + "." + b64url(signer.sign(signing_input.encode("ascii")))

    def issue(self, claims: dict[str, Any]) -> tuple[str, dict[str, Any]]:
        """Sign ``claims`` (without ``iss`` and ``jti``, which are added) as a new token.

        Returns:
            The compact token and its full claims.
        """
        full = {"iss": str(self.instance_id), "jti": str(uuid7()), **claims}
        ordered = {n: full[n] for n in _ORDER if n in full}
        header = {"alg": "EdDSA", "typ": TYP, "kid": self.kid}
        return self._sign(header, ordered), ordered

    def check(  # noqa: PLR0913 - the inputs of the server check
        self,
        token: str,
        *,
        aud: UUID,
        now: datetime,
        leeway: float,
        run_id: UUID | None = None,
        ignore_expiry: bool = False,
    ) -> Checked:
        """The server's ``Verify``: header, key, signature, shape, expected values, expiry.

        Returns:
            ``valid``, ``expired`` (only when every other rule holds) or ``invalid``.
        """
        claims = self._decode(token)
        if claims is None or not shape_ok(claims):
            return Checked("invalid")
        if claims["iss"] != str(self.instance_id) or claims["aud"] != str(aud):
            return Checked("invalid", claims)
        if run_id is not None and claims.get("run_id") != str(run_id):
            return Checked("invalid", claims)
        stamp = now.timestamp()
        if claims["iat"] > stamp + leeway:
            return Checked("invalid", claims)
        if not ignore_expiry and stamp >= claims["exp"] + leeway:
            return Checked("expired", claims)
        return Checked("valid", claims)

    def _decode(self, token: str) -> dict[str, Any] | None:  # noqa: PLR0911 - one rule each
        parts = token.split(".")
        if len(parts) != 3 or not all(parts):
            return None
        try:
            header = strict_object(b64url_decode(parts[0]))
            claims = strict_object(b64url_decode(parts[1]))
            signature = b64url_decode(parts[2])
        except ValueError:
            return None
        if header.get("alg") != "EdDSA" or header.get("typ") != TYP:
            return None
        if _FORBIDDEN_HEADER & set(header) or not header.get("kid") or header["kid"] != self.kid:
            return None
        if len(signature) != 64:
            return None
        try:
            self._key.public_key().verify(signature, f"{parts[0]}.{parts[1]}".encode("ascii"))
        except InvalidSignature:
            return None
        return claims

    def forge(self, claims: dict[str, Any], variant: str, *, other_agent_id: UUID) -> Secret:
        """Build one of the forged tokens of AG-08 from the claims of a delivered token.

        Like the server's tests, header and claims are encoded with sorted members and always
        signed with Ed25519: ``bad_signature`` with a new key under the same ``kid``,
        ``alg_hs256`` with ``alg = HS256``, ``typ_access`` with ``typ = at+jwt`` and
        ``aud_other_agent`` with the other agent's ID, each validly signed otherwise.

        Raises:
            ValueError: An unknown variant.
        """
        header: dict[str, Any] = {"alg": "EdDSA", "kid": self.kid, "typ": TYP}
        body = dict(sorted(claims.items()))
        key = None
        if variant == "bad_signature":
            key = Ed25519PrivateKey.generate()
        elif variant == "alg_hs256":
            header["alg"] = "HS256"
        elif variant == "typ_access":
            header["typ"] = "at+jwt"
        elif variant == "aud_other_agent":
            body["aud"] = str(other_agent_id)
        else:
            raise ValueError(f"unknown forge variant: {variant}")
        return Secret(self._sign(dict(sorted(header.items())), body, key))


_ORDER = (
    "iss",
    "aud",
    "sub",
    "jti",
    "iat",
    "exp",
    "trigger_kind",
    "run_id",
    "message_id",
    "interaction_id",
    "command_id",
    "thread_id",
    "channel_id",
    "space_id",
    "level",
    "billing_mode",
    "tool_class",
    "permission_version",
)
