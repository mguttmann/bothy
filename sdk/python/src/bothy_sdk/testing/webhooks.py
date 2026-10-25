# SPDX-License-Identifier: Apache-2.0
"""Signing of test deliveries the way the server signs them (docs/api/webhooks.md 2.1 point 4).

For receivers under test: `sign_webhook` writes the ``webhook-signature`` value of a delivery
and `webhook_public_key` the published form of a seed's key (docs/DECISIONS.md D-M2-26-10 and
D-M2-26-11). Needs only ``cryptography``, a dependency of the SDK itself.

Examples:
    >>> header = sign_webhook(
    ...     webhook_id="evt-1", timestamp=1760346000, body="{}", seeds=[bytes(32)]
    ... )
    >>> header.startswith("v1a,")
    True
"""

from __future__ import annotations

import base64
import hashlib
import hmac
from collections.abc import Sequence

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from bothy_sdk.webhooks import parse_webhook_secret

__all__ = ["sign_webhook", "webhook_public_key"]

_SEED_BYTES = 32


def _private_key(seed: bytes) -> Ed25519PrivateKey:
    if len(seed) != _SEED_BYTES:
        raise ValueError("an Ed25519 seed has 32 bytes")
    return Ed25519PrivateKey.from_private_bytes(bytes(seed))


def sign_webhook(
    *,
    webhook_id: str,
    timestamp: int,
    body: bytes | str,
    seeds: Sequence[bytes] = (),
    secrets: Sequence[bytes | str] = (),
) -> str:
    """Return the ``webhook-signature`` value the server would send.

    The ``v1a`` entries come first in seed order, then the ``v1`` entries in the given order,
    separated by one space, each ``<scheme>,<standard base64>`` over
    ``{webhook_id}.{timestamp}.{body}``.

    Args:
        webhook_id: The ``webhook-id``, equal to the event ID.
        timestamp: The ``webhook-timestamp``, Unix seconds.
        body: The body exactly as sent; a string counts as its UTF-8 bytes.
        seeds: 32-byte Ed25519 private seeds, one ``v1a`` entry each, in signer order.
        secrets: ``v1`` secrets in ``whsec_`` form or as raw key bytes, newest generation
            first.

    Returns:
        The header value; empty without seeds and secrets.

    Raises:
        ValueError: A seed has another length.
        WebhookConfigError: A secret is malformed.

    Examples:
        >>> sign_webhook(webhook_id="evt-1", timestamp=1760346000, body=b"{}")
        ''
    """
    raw = body.encode() if isinstance(body, str) else bytes(body)
    content = f"{webhook_id}.{timestamp}.".encode() + raw
    entries: list[str] = []
    for seed in seeds:
        signature = _private_key(seed).sign(content)
        entries.append("v1a," + base64.b64encode(signature).decode("ascii"))
    for secret in secrets:
        key = parse_webhook_secret(secret) if isinstance(secret, str) else bytes(secret)
        digest = hmac.new(key, content, hashlib.sha256).digest()
        entries.append("v1," + base64.b64encode(digest).decode("ascii"))
    return " ".join(entries)


def webhook_public_key(seed: bytes) -> str:
    """Return the published form of the public key of a seed, as ``getWebhookKeys`` lists it.

    Args:
        seed: A 32-byte Ed25519 private seed.

    Returns:
        ``whpk_`` plus the standard base64 of the key's DER SubjectPublicKeyInfo.

    Raises:
        ValueError: The seed has another length.

    Examples:
        >>> webhook_public_key(bytes(32)).startswith("whpk_MCowBQYDK2VwAyEA")
        True
    """
    der = (
        _private_key(seed)
        .public_key()
        .public_bytes(Encoding.DER, PublicFormat.SubjectPublicKeyInfo)
    )
    return "whpk_" + base64.b64encode(der).decode("ascii")
