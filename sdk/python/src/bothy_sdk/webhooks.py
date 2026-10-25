# SPDX-License-Identifier: Apache-2.0
"""Standard Webhooks verification for apps (11 11.2 row ``webhooks``, docs/api/webhooks.md 3).

`verify_webhook` checks a delivery of an outgoing webhook or a request to ``interactions_url``
(docs/spec/11-extensions.md 7 and 9): ``v1a`` entries with the Ed25519 keys of
``getWebhookKeys``, ``v1`` entries with HMAC-SHA256 and a ``whsec_`` secret, both over the raw
body and inside ``webhook.receiver_tolerance``. `WebhookKeyCache` holds the trusted keys. The
behaviour is the one of the TypeScript module ``webhooks`` (M2-30), with the snake_case names of
its functions (docs/DECISIONS.md D-M2-30-9).

Storing the seen ``webhook-id`` values is the app's duty: remember `VerifiedWebhook.id` for at
least ``webhook.retry_max_age`` (docs/api/webhooks.md 3 point 4).

Examples:
    >>> from datetime import UTC, datetime
    >>> from bothy_sdk.testing import FakeClock, sign_webhook
    >>> secret = "whsec_" + "RVhBTVBMRXNlY3JldA=="
    >>> body = b'{"type":"future.example","seq":3}'
    >>> header = sign_webhook(webhook_id="evt-1", timestamp=1760346000, body=body, secrets=[secret])
    >>> verified = verify_webhook(
    ...     body,
    ...     {"webhook-id": "evt-1", "webhook-timestamp": "1760346000", "webhook-signature": header},
    ...     secrets=[secret],
    ...     clock=FakeClock(datetime.fromtimestamp(1760346000, tz=UTC)),
    ... )
    >>> (verified.scheme, verified.envelope.type)
    ('v1', 'future.example')
"""

from __future__ import annotations

import asyncio
import base64
import binascii
import contextlib
import hashlib
import hmac
import json
import re
from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from types import MappingProxyType
from typing import Any, Literal

from cryptography.exceptions import UnsupportedAlgorithm
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
from cryptography.hazmat.primitives.serialization import load_der_public_key

from bothy_sdk.client import Client
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.errors import problem_error
from bothy_sdk.generated.api.discovery import get_webhook_keys
from bothy_sdk.generated.client import Client as CoreClient
from bothy_sdk.generated.models.webhook_keys import WebhookKeys
from bothy_sdk.generated.types import Response
from bothy_sdk.retry import parse_retry_after
from bothy_sdk.settings import SdkSettings
from bothy_sdk.tolerance import KnownEvent, UnknownEvent, decode_event

__all__ = [
    "VerifiedWebhook",
    "WebhookConfigError",
    "WebhookKeyCache",
    "WebhookVerificationError",
    "parse_webhook_public_key",
    "parse_webhook_secret",
    "verify_webhook",
]

VerificationErrorCode = Literal[
    "missing_header",
    "invalid_timestamp",
    "timestamp_out_of_tolerance",
    "no_trusted_key",
    "signature_mismatch",
]
"""Why a delivery was refused, one value per step of `verify_webhook`."""

_PUBLIC_KEY_PREFIX = "whpk_"
_SECRET_PREFIX = "whsec_"  # noqa: S105 - the prefix of the format, no secret  # nosec B105
# Standard base64 with padding and nothing else; checked with fullmatch, so no trailing newline.
_STANDARD_BASE64 = re.compile(r"(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?")
# ASCII digits only (``\d`` would match other scripts); at most 15 keeps parity with the
# TypeScript verifier, whose numbers are exact to that length. Unix seconds need 10.
_UNIX_SECONDS = re.compile(r"[0-9]{1,15}")
_MAX_AGE = re.compile(r"[0-9]+")
_HEADER_ID = "webhook-id"
_HEADER_TIMESTAMP = "webhook-timestamp"
_HEADER_SIGNATURE = "webhook-signature"
_HMAC_BYTES = 32
# Every state the server lists is trusted: `next` signs soon, `retiring` still signs until
# `not_after`. A state of a newer server is not trusted, since its meaning is unknown.
_TRUSTED_STATES = frozenset({"next", "active", "retiring"})


class WebhookVerificationError(Exception):
    """A delivery failed verification; answer it with ``401`` and do not process it.

    The message names the failed step only; it never contains the body, a signature, a key or
    a secret (SEC-LOG-2), so a caller may log it as it is.

    Attributes:
        code: The failed step: ``missing_header`` (``webhook-id``, ``webhook-timestamp`` or
            ``webhook-signature`` absent), ``invalid_timestamp`` (no integer of Unix seconds),
            ``timestamp_out_of_tolerance`` (farther than the tolerance from the own clock, in
            either direction), ``no_trusted_key`` (neither a key nor a secret configured) or
            ``signature_mismatch`` (no entry valid for a trusted key or secret).

    Examples:
        >>> err = WebhookVerificationError("missing_header", "a webhook needs its headers")
        >>> (err.code, str(err))
        ('missing_header', 'a webhook needs its headers')
    """

    def __init__(self, code: VerificationErrorCode, message: str) -> None:
        """Create the error.

        Args:
            code: The failed step.
            message: A text naming the step, never a value of the request.
        """
        super().__init__(message)
        self.code: VerificationErrorCode = code


class WebhookConfigError(ValueError):
    """A configured public key or secret is malformed.

    Another prefix, bad base64, another key type or an empty value: a fault of the receiver's
    configuration, not of a delivery. The message never repeats the value.

    Examples:
        >>> try:
        ...     parse_webhook_secret("not-a-secret")
        ... except WebhookConfigError as err:
        ...     print(err)
        a webhook secret starts with whsec_
    """


@dataclass(frozen=True)
class VerifiedWebhook:
    """A delivery that passed `verify_webhook`.

    Attributes:
        id: The ``webhook-id``, equal to the event ID; the key of the app's duplicate check.
        timestamp: The ``webhook-timestamp``, Unix seconds of this attempt.
        scheme: The scheme of the first valid entry of ``webhook-signature``.
        envelope: The body decoded by `bothy_sdk.tolerance.decode_event`. A body that is no
            JSON object (invalid UTF-8 or JSON included) is an `UnknownEvent` with type ``""``,
            no ``seq`` and an empty ``raw``; an event of a known type that does not decode is
            an `UnknownEvent` with that type and the received object, never an exception.
        sync_epoch: The delivery member ``sync_epoch`` when it is a string; a change means the
            instance was restored (docs/api/webhooks.md 3 point 6).

    Examples:
        >>> v = VerifiedWebhook("evt-1", 1760346000, "v1a", UnknownEvent("", None, {}), None)
        >>> (v.id, v.scheme)
        ('evt-1', 'v1a')
    """

    id: str
    timestamp: int
    scheme: Literal["v1a", "v1"]
    envelope: KnownEvent | UnknownEvent = field(hash=False)
    sync_epoch: str | None


def _decode_standard_base64(text: str) -> bytes | None:
    """Decode standard base64 with padding strictly; ``None`` when the text is no such form."""
    if _STANDARD_BASE64.fullmatch(text) is None:
        return None
    try:
        return base64.b64decode(text, validate=True)
    except (binascii.Error, ValueError):  # pragma: no cover - the pattern admits valid forms only
        return None


def parse_webhook_public_key(value: str) -> Ed25519PublicKey:
    """Read a public key of ``getWebhookKeys``.

    The form is ``whpk_`` plus the standard base64 (with padding) of the DER-encoded
    SubjectPublicKeyInfo of an Ed25519 key (RFC 8410, docs/DECISIONS.md D-M2-26-11).

    Args:
        value: The ``public_key`` of a ``WebhookKey``.

    Returns:
        The Ed25519 public key.

    Raises:
        WebhookConfigError: Another prefix, bad base64, bad DER or another key type; the
            message never repeats the value.

    Examples:
        >>> from bothy_sdk.testing import webhook_public_key
        >>> key = parse_webhook_public_key(webhook_public_key(bytes(32)))
        >>> type(key).__name__.startswith("Ed25519")
        True
    """
    if not value.startswith(_PUBLIC_KEY_PREFIX):
        raise WebhookConfigError("a webhook public key starts with whpk_")
    der = _decode_standard_base64(value[len(_PUBLIC_KEY_PREFIX) :])
    if not der:
        raise WebhookConfigError("a webhook public key is whpk_ and standard base64")
    try:
        key = load_der_public_key(der)
    except (ValueError, UnsupportedAlgorithm):
        raise WebhookConfigError("a webhook public key is a DER SubjectPublicKeyInfo") from None
    if not isinstance(key, Ed25519PublicKey):
        raise WebhookConfigError("a webhook public key is an Ed25519 key")
    return key


def parse_webhook_secret(value: str) -> bytes:
    """Read a ``v1`` secret: ``whsec_`` plus the standard base64 of the HMAC key.

    Args:
        value: The secret as the server shows it once.

    Returns:
        The HMAC-SHA256 key bytes.

    Raises:
        WebhookConfigError: The value has another prefix, is no standard base64 or decodes to
            no bytes; the message never repeats the value.

    Examples:
        >>> parse_webhook_secret("whsec_" + "RVhBTVBMRQ==")
        b'EXAMPLE'
    """
    if not value.startswith(_SECRET_PREFIX):
        raise WebhookConfigError("a webhook secret starts with whsec_")
    key = _decode_standard_base64(value[len(_SECRET_PREFIX) :])
    if not key:
        raise WebhookConfigError("a webhook secret is whsec_ and non-empty standard base64")
    return key


def _header(headers: Mapping[str, str], name: str) -> str | None:
    # Several entries differing only in case are joined with ", ", as the Fetch API joins
    # repeated headers (the TypeScript verifier does the same for a plain record).
    values = [value for key, value in headers.items() if key.lower() == name]
    return ", ".join(values) if values else None


def _trusted_keys(keys: Sequence[Ed25519PublicKey | str]) -> list[Ed25519PublicKey]:
    return [parse_webhook_public_key(k) if isinstance(k, str) else k for k in keys]


def _trusted_secrets(secrets: Sequence[bytes | str]) -> list[bytes]:
    out: list[bytes] = []
    for secret in secrets:
        if isinstance(secret, str):
            out.append(parse_webhook_secret(secret))
        elif not secret:
            raise WebhookConfigError("a webhook secret is non-empty")
        else:
            out.append(bytes(secret))
    return out


def _hmac_matches(secrets: Sequence[bytes], content: bytes, signature: bytes) -> bool:
    if len(signature) != _HMAC_BYTES:
        return False
    match = False
    for secret in secrets:
        expected = hmac.new(secret, content, hashlib.sha256).digest()
        # No early exit, so the time does not tell which secret matched.
        if hmac.compare_digest(expected, signature):
            match = True
    return match


def _ed25519_matches(keys: Sequence[Ed25519PublicKey], content: bytes, signature: bytes) -> bool:
    for key in keys:
        # InvalidSignature, a malformed signature or a key object of another type: no match.
        with contextlib.suppress(Exception):
            key.verify(signature, content)
            return True
    return False


def _without_null_members(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    # As the REST tolerance layer (docs/spec/06-api.md 2 "Kein null"): a member whose value is
    # null is dropped (after the last of duplicate names won); null items of an array stay.
    return {k: v for k, v in dict(pairs).items() if v is not None}


def _reject_constant(name: str) -> Any:
    # NaN and Infinity are no JSON (RFC 8259); the standard library would accept them.
    raise ValueError(name)


def _decode_body(body: bytes) -> tuple[KnownEvent | UnknownEvent, str | None]:
    try:
        # utf-8-sig skips one leading byte order mark as the TypeScript TextDecoder does.
        raw: object = json.loads(
            body.decode("utf-8-sig"),
            object_pairs_hook=_without_null_members,
            parse_constant=_reject_constant,
        )
    except (UnicodeDecodeError, ValueError, RecursionError):
        # Authentic but no JSON: skipped by the caller like any event it does not know (04 7.1).
        return UnknownEvent(type="", seq=None, raw=MappingProxyType({})), None
    if not isinstance(raw, dict):
        return UnknownEvent(type="", seq=None, raw=MappingProxyType({})), None
    epoch = raw.get("sync_epoch")
    sync_epoch = epoch if isinstance(epoch, str) else None
    try:
        return decode_event(raw), sync_epoch
    except (KeyError, ValueError, TypeError, AttributeError):
        # A known type whose members do not fit its model: the caller sees it as unknown.
        kind = raw.get("type")
        seq = raw.get("seq")
        unknown = UnknownEvent(
            type=kind if isinstance(kind, str) else "",
            seq=seq if isinstance(seq, int) and not isinstance(seq, bool) else None,
            raw=MappingProxyType(raw),
        )
        return unknown, sync_epoch


def verify_webhook(  # noqa: PLR0913 - the keyword-only trust list of Design note 8
    body: bytes,
    headers: Mapping[str, str],
    *,
    keys: Sequence[Ed25519PublicKey | str] = (),
    secrets: Sequence[bytes | str] = (),
    tolerance: float | None = None,
    clock: Clock | None = None,
) -> VerifiedWebhook:
    """Verify one delivery of an outgoing webhook or a request to ``interactions_url``.

    Steps, each with its error code: the configured keys and secrets parse
    (`WebhookConfigError`); the three headers are present (``missing_header``);
    ``webhook-timestamp`` is an integer of Unix seconds (``invalid_timestamp``); its distance
    to the clock is at most ``tolerance`` in both directions (``timestamp_out_of_tolerance``);
    at least one key or secret is configured (``no_trusted_key``); one entry of
    ``webhook-signature`` is valid over ``{webhook-id}.{webhook-timestamp}.{body}``
    (``signature_mismatch``). Entries are split at spaces and at their first comma; an unknown
    scheme or bad base64 is ignored. ``v1a`` entries are checked against every key with
    Ed25519, ``v1`` entries against every secret with HMAC-SHA256 and `hmac.compare_digest`.

    Args:
        body: The raw request body exactly as received; never a body parsed and serialized
            again.
        headers: The request headers, looked up case-insensitively.
        keys: Trusted ``v1a`` public keys, parsed or in their ``whpk_`` form; normally the
            result of `WebhookKeyCache.keys`. Never taken from the request.
        secrets: Trusted ``v1`` secrets in their ``whsec_`` form or as raw key bytes.
        tolerance: Largest distance of ``webhook-timestamp`` to the clock in seconds; default
            ``SdkSettings().webhook_receiver_tolerance`` (``webhook.receiver_tolerance``).
        clock: The own clock; default `bothy_sdk.clock.SystemClock`.

    Returns:
        The verified delivery with its decoded envelope.

    Raises:
        WebhookVerificationError: The delivery fails a step; the message never contains the
            body, a signature or a secret.
        WebhookConfigError: A configured key or secret is malformed.

    Examples:
        >>> try:
        ...     verify_webhook(b"{}", {}, keys=[])
        ... except WebhookVerificationError as err:
        ...     print(err.code)
        missing_header
    """
    trusted_keys = _trusted_keys(keys)
    trusted_secrets = _trusted_secrets(secrets)

    webhook_id = _header(headers, _HEADER_ID)
    timestamp_text = _header(headers, _HEADER_TIMESTAMP)
    signatures = _header(headers, _HEADER_SIGNATURE)
    if webhook_id is None or timestamp_text is None or signatures is None:
        raise WebhookVerificationError(
            "missing_header",
            "a webhook needs the headers webhook-id, webhook-timestamp and webhook-signature",
        )
    if _UNIX_SECONDS.fullmatch(timestamp_text) is None:
        raise WebhookVerificationError(
            "invalid_timestamp", "webhook-timestamp is no integer of Unix seconds"
        )
    timestamp = int(timestamp_text)
    now = (clock or SystemClock()).now().timestamp()
    # webhook.receiver_tolerance
    limit = SdkSettings().webhook_receiver_tolerance if tolerance is None else tolerance
    if abs(now - timestamp) > limit:
        raise WebhookVerificationError(
            "timestamp_out_of_tolerance",
            "webhook-timestamp lies outside webhook.receiver_tolerance",
        )
    if not trusted_keys and not trusted_secrets:
        raise WebhookVerificationError(
            "no_trusted_key", "no webhook public key and no webhook secret is configured"
        )

    # The canonical decimal form of the timestamp, as the signer writes it.
    content = f"{webhook_id}.{timestamp}.".encode() + bytes(body)
    scheme: Literal["v1a", "v1"] | None = None
    for entry in signatures.split(" "):
        name, comma, value = entry.partition(",")
        if not comma or name not in ("v1a", "v1"):
            continue
        signature = _decode_standard_base64(value)
        if signature is None:
            continue
        if name == "v1a":
            valid = _ed25519_matches(trusted_keys, content, signature)
        else:
            valid = _hmac_matches(trusted_secrets, content, signature)
        if valid:
            scheme = "v1a" if name == "v1a" else "v1"
            break
    if scheme is None:
        raise WebhookVerificationError(
            "signature_mismatch",
            "no entry of webhook-signature is valid for a trusted key or secret",
        )
    envelope, sync_epoch = _decode_body(bytes(body))
    return VerifiedWebhook(
        id=webhook_id,
        timestamp=timestamp,
        scheme=scheme,
        envelope=envelope,
        sync_epoch=sync_epoch,
    )


def _max_age(cache_control: str | None) -> float | None:
    for directive in (cache_control or "").split(","):
        parts = directive.strip().split("=")
        if parts[0].lower() != "max-age" or len(parts) < 2:
            continue
        seconds = parts[1].strip()
        if len(seconds) >= 2 and seconds[0] == seconds[-1] == '"':
            seconds = seconds[1:-1]
        return float(seconds) if _MAX_AGE.fullmatch(seconds) else None
    return None


class WebhookKeyCache:
    """Holds the ``v1a`` public keys of ``getWebhookKeys`` as a trust list.

    The list is fetched on first use and again only after the ``max-age`` of its response's
    ``Cache-Control`` (without one: ``agent.jwks_max_age``; never sooner than
    ``adapter.jwks_refresh_min_interval``), measured with the clock's monotonic time. A new
    key appears in it beforehand as ``next``, so a failed verification is never a reason to
    fetch: a forged request cannot make the app call the server.

    ``getWebhookKeys`` is public (``security: []``) and an app token has no scope for it (the
    server answers an app token ``403``), so an app fetches the list without a token, as the
    TypeScript SDK's example does: pass a generated core client
    (`bothy_sdk.generated.client.Client`) without credentials. The hand-layer
    `bothy_sdk.client.Client` is accepted too, for a caller whose token may read the list.

    Args:
        client: A generated core client without credentials, or the hand-layer client.
        clock: Time source of the cache age; default `bothy_sdk.clock.SystemClock`.
        settings: ``webhook_keys_max_age`` without a ``max-age`` and
            ``jwks_refresh_min_interval`` as the shortest age; default `SdkSettings()`.

    Examples:
        >>> from bothy_sdk.generated.client import Client as CoreClient
        >>> cache = WebhookKeyCache(CoreClient(base_url="https://chat.example.com/api/v1"))
        >>> # keys = await cache.keys()
        >>> # verified = verify_webhook(raw_body, request_headers, keys=keys)
    """

    def __init__(
        self,
        client: Client | CoreClient,
        *,
        clock: Clock | None = None,
        settings: SdkSettings | None = None,
    ) -> None:
        """Create the cache; nothing is fetched before `keys`."""
        self._client = client
        self._clock = clock or SystemClock()
        self._settings = settings or SdkSettings()
        self._cached: list[Ed25519PublicKey] | None = None
        self._expires = 0.0
        self._pending: asyncio.Task[list[Ed25519PublicKey]] | None = None

    async def keys(self) -> list[Ed25519PublicKey]:
        """Return the trusted keys, fetching them when the list is missing or too old.

        Concurrent callers share one fetch.

        Returns:
            Every readable listed key in the states ``next``, ``active`` and ``retiring``.

        Raises:
            ProblemError: ``getWebhookKeys`` answered with a problem; the next call tries
                again.
            RuntimeError: The answer could not be read; the next call tries again.
        """
        if self._cached is not None and self._clock.monotonic() < self._expires:
            return list(self._cached)
        if self._pending is None:
            self._pending = asyncio.ensure_future(self._shared_fetch())
        # A cancelled caller does not cancel the fetch the other callers wait for.
        return list(await asyncio.shield(self._pending))

    async def _shared_fetch(self) -> list[Ed25519PublicKey]:
        try:
            return await self._fetch()
        finally:
            self._pending = None

    async def _get(self) -> Response[Any]:
        if isinstance(self._client, Client):
            return await self._client.call_detailed(get_webhook_keys.asyncio_detailed)
        response = await get_webhook_keys.asyncio_detailed(client=self._client)
        status = int(response.status_code)
        if not 200 <= status < 300:
            # The generated parser has read the body as a problem object already.
            retry_after = parse_retry_after(response.headers.get("Retry-After"))
            raise problem_error(status, json.loads(response.content), retry_after=retry_after)
        return response

    async def _fetch(self) -> list[Ed25519PublicKey]:
        try:
            response = await self._get()
        except (KeyError, TypeError, ValueError) as exc:
            # The generated parser failed on the body; never repeat it (it may hold anything).
            reason = type(exc).__name__
            raise RuntimeError(f"getWebhookKeys answer is unreadable: {reason}") from None
        listed = response.parsed
        if not isinstance(listed, WebhookKeys):
            raise RuntimeError("getWebhookKeys answer is unreadable")
        keys: list[Ed25519PublicKey] = []
        for key in listed.keys:
            if key.state.value not in _TRUSTED_STATES:
                continue
            # A key this SDK cannot read signs nothing it could verify; the others stay usable.
            with contextlib.suppress(WebhookConfigError, AttributeError):
                keys.append(parse_webhook_public_key(key.public_key))
        # agent.jwks_max_age without a max-age; adapter.jwks_refresh_min_interval as the floor.
        age = _max_age(response.headers.get("cache-control"))
        if age is None:
            age = self._settings.webhook_keys_max_age
        self._cached = keys
        self._expires = self._clock.monotonic() + max(age, self._settings.jwks_refresh_min_interval)
        return keys
