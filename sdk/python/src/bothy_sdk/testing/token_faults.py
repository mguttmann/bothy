# SPDX-License-Identifier: Apache-2.0
"""Forged variants of an invocation token for the Fake-Agent (AG-08 of docs/spec/09-agents.md).

Each variant is built from a real token of the run and must be refused by every verifier, the
SDK's and the server's: `bad_signature` changes one byte of the signature, `alg_hs256` and
`alg_none` re-encode the header with another algorithm (the signature stays, so it no longer
matches the signing input), and `typ_access` gives the header the ``typ`` of an access token.
The original token is never changed: every function returns a new `Secret`.

Examples:
    >>> sorted(VARIANTS)
    ['alg_hs256', 'alg_none', 'bad_signature', 'typ_access']
"""

from __future__ import annotations

import base64
import json
from collections.abc import Callable

from bothy_sdk.auth import Secret

__all__ = ["VARIANTS", "forge"]


def _b64(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def _unb64(text: str) -> bytes:
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


def _parts(token: Secret) -> list[str]:
    parts = token.reveal().split(".")
    if len(parts) != 3:  # header, payload, signature
        raise ValueError("not a compact JWS")
    return parts


def _with_header(token: Secret, change: Callable[[dict[str, object]], None]) -> Secret:
    head, body, sig = _parts(token)
    header = json.loads(_unb64(head))
    if not isinstance(header, dict):
        raise ValueError("the header is not an object")
    change(header)
    new_head = _b64(json.dumps(header, separators=(",", ":")).encode())
    return Secret(f"{new_head}.{body}.{sig}")


def _bad_signature(token: Secret) -> Secret:
    head, body, sig = _parts(token)
    raw = bytearray(_unb64(sig))
    if not raw:
        raise ValueError("empty signature")
    raw[0] ^= 0x01
    return Secret(f"{head}.{body}.{_b64(bytes(raw))}")


def _alg(value: str) -> Callable[[Secret], Secret]:
    def forge_alg(token: Secret) -> Secret:
        return _with_header(token, lambda h: h.__setitem__("alg", value))

    return forge_alg


def _typ_access(token: Secret) -> Secret:
    return _with_header(token, lambda h: h.__setitem__("typ", "at+jwt"))


VARIANTS: dict[str, Callable[[Secret], Secret]] = {
    "bad_signature": _bad_signature,
    "alg_hs256": _alg("HS256"),
    "alg_none": _alg("none"),
    "typ_access": _typ_access,
}
"""The forged variants by name (``forged:<name>`` in a scenario)."""


def forge(variant: str, token: Secret) -> Secret:
    """Return the forged variant of ``token``.

    Args:
        variant: A key of `VARIANTS`.
        token: A real invocation token; it is not changed.

    Returns:
        A new token that every verifier must refuse.

    Raises:
        KeyError: The variant is unknown.
        ValueError: The token is not a compact JWS.
    """
    return VARIANTS[variant](token)
