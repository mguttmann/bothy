# SPDX-License-Identifier: Apache-2.0
"""The signature of a Jira webhook delivery (docs/spec/11-extensions.md 10.2; RK-83).

Jira names the method in the header: ``X-Hub-Signature: <method>=<signature>``. The app reads
the method, accepts exactly the methods of `SUPPORTED_METHODS` and refuses every other one
before any HMAC is computed, instead of assuming SHA-256: Jira announces that the method may
change, and an app that guessed it would accept forged or reject valid deliveries. A new method
is a code change with its test values.

Rules (brief M2-50 Design note 5): exactly one header (none is ``missing``, more is
``malformed``); split at the first ``=``, no ``=`` or an empty method is ``malformed``; the
method compared exactly with the allow list (``unsupported_method``); the signature 64
hexadecimal digits decoded with `bytes.fromhex` (``malformed``); HMAC-SHA256 with the secret over
the raw body compared with the decoded 32 bytes through `hmac.compare_digest` only
(``mismatch``).

Examples:
    >>> body = b"Hello, World!"
    >>> value = "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"
    >>> verify_signature(b"It's a Secret to Everybody", body, [value]) is None
    True
    >>> verify_signature(b"x", body, ["sha1=01dc10d0c83e72ed246219cdd91669667fe2ca59"])
    'unsupported_method'
"""

from __future__ import annotations

import hashlib
import hmac
from collections.abc import Sequence

__all__ = [
    "HEADER",
    "SUPPORTED_METHODS",
    "compute_hmac",
    "sign_body",
    "verify_signature",
]

HEADER = "X-Hub-Signature"
"""The header of Jira's signature, ``<method>=<hex signature>``."""

SUPPORTED_METHODS = frozenset({"sha256"})
"""The methods the app verifies; every other method is refused (RK-83)."""

_DIGEST_HEX = 64
_HEX = frozenset("0123456789abcdefABCDEF")


def compute_hmac(secret: bytes, body: bytes) -> bytes:
    """Return HMAC-SHA256 of ``body`` with ``secret``, the only method of `SUPPORTED_METHODS`.

    Args:
        secret: The webhook secret.
        body: The raw request body.

    Returns:
        The 32-byte digest.
    """
    return hmac.new(secret, body, hashlib.sha256).digest()


def sign_body(secret: bytes, body: bytes) -> str:
    """Return the header value Jira sends for ``body``: ``sha256=<lower-case hex>``.

    Args:
        secret: The webhook secret.
        body: The raw body.

    Returns:
        The value of ``X-Hub-Signature``.
    """
    return "sha256=" + compute_hmac(secret, body).hex()


def verify_signature(secret: bytes, body: bytes, values: Sequence[str]) -> str | None:
    """Check the ``X-Hub-Signature`` values of one request.

    Args:
        secret: The webhook secret, read at this use.
        body: The raw body exactly as received.
        values: Every value of the header in the request (case-insensitive name).

    Returns:
        ``None`` for a valid signature, else the reason: ``missing``, ``malformed``,
        ``unsupported_method`` or ``mismatch``. Never raises for any header value.
    """
    if not values:
        return "missing"
    if len(values) > 1:
        return "malformed"
    method, sep, signature = values[0].partition("=")
    if not sep or not method:
        return "malformed"
    # Decided before any HMAC: there is no fallback to SHA-256 (RK-83).
    if method not in SUPPORTED_METHODS:
        return "unsupported_method"
    if len(signature) != _DIGEST_HEX or not set(signature) <= _HEX:
        return "malformed"
    given = bytes.fromhex(signature)
    expected = compute_hmac(secret, body)
    # Through the module attribute, so that only compare_digest decides (and tests can see it).
    if not hmac.compare_digest(expected, given):
        return "mismatch"
    return None
