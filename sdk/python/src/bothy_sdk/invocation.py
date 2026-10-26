# SPDX-License-Identifier: Apache-2.0
"""The adapter's check of the invocation token (docs/spec/09-agents.md 4.5 points 1 to 10).

`InvocationVerifier.check` decides one verdict per token in the precedence of M2-06's reference
verifier (``services/server/internal/runs/adapters/runstest/refverifier.go``):

1. Points 1 and 2 and the token form: three segments of strict base64url, a header object
   without duplicate members, ``alg`` exactly ``EdDSA``, ``typ`` exactly
   ``agent-invocation+jwt``, no ``crit``, a non-empty ``kid``. Failure: ``INVALID``.
2. Point 3: the ``kid`` is in the cached JWKS, after at most one forced reload per
   ``adapter.jwks_refresh_min_interval``. Failure: ``UNKNOWN_KEY``.
3. Point 4: the signature (PyJWT with ``algorithms=["EdDSA"]`` only), the payload shape, then
   ``iss`` and ``aud`` byte-exactly (``aud`` a single string). Failure: ``INVALID``.
4. Point 6: ``run_id``, ``thread_id`` and the triggering object equal the event's payload; a
   triggering object without counterpart in the payload fails closed. Failure: ``INVALID``.
5. Point 9: the allow list of 17.7 when configured; a ``sub`` still missing after the reload
   rules is ``INVALID`` (docs/DECISIONS.md D-M2-29-8).
6. Point 8: ``billing_mode`` equals the actual mode, and ``unknown`` never matches
   (docs/spec/10-backend-policy.md 7). Failure: ``BILLING_MODE_MISMATCH``.
7. Point 10: the actual tool class is not above ``tool_class`` (``chat`` < ``scoped`` <
   ``host``). Failure: ``TOOL_CLASS_MISMATCH``.
8. Point 5: ``iat`` after now plus ``adapter.invocation_token_leeway``, or ``exp`` not after
   ``iat``, is ``INVALID``; now at or after ``exp`` plus that leeway is ``EXPIRED``.
9. Point 7: a ``jti`` seen before and not yet expired is ``REPLAY``. A passing token's ``jti``
   is remembered until ``exp`` plus the leeway.

Header, ``iss``, ``aud`` and the times are compared by this module itself, not by PyJWT, so
that the precedence above holds; PyJWT only verifies the signature. The JWKS is cached for the
``max-age`` its fetcher reports (absent: fetched again on the next use). The token itself never
appears in a log, a ``repr`` or an exception text (SEC-LOG-2).

Examples:
    >>> expectation = Expectation(thread_id=None, run_id=None, trigger_claim="command_id",
    ...                           trigger_id=None)
    >>> expectation.trigger_id is None  # no counterpart in the payload: the check fails closed
    True
"""

from __future__ import annotations

import asyncio
import base64
import binascii
import json
import logging
import re
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import Enum
from typing import Any, Protocol, runtime_checkable
from uuid import UUID

import jwt
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from bothy_sdk.auth import Secret
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.generated.models import (
    AgentFollowupPayload,
    AgentInvocationPayload,
    AgentTriggerers,
    CommandAutocompleteRequestedPayload,
    CommandInvokedPayload,
    InteractionCreatedPayload,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.settings import SdkSettings

__all__ = [
    "TOKEN_EVENT_TYPES",
    "Actual",
    "Expectation",
    "InvocationClaims",
    "InvocationVerifier",
    "JwksFetcher",
    "ReplayStore",
    "TokenCheck",
    "TokenVerdict",
    "TriggererLoader",
    "compare_actual",
    "expectation_for",
    "stored_claims",
]

_log = logging.getLogger("bothy_sdk")

_ALG = "EdDSA"
_TYP = "agent-invocation+jwt"
_B64URL = re.compile(r"^[A-Za-z0-9_-]+$")
_ED25519_KEY_BYTES = 32
_INT64_MIN = -(2**63)
_INT64_MAX = 2**63 - 1
# Order of the tool classes, docs/spec/07-permissions.md 2.11.
_TOOL_CLASS_RANK = {"chat": 0, "scoped": 1, "host": 2}
_REQUIRED_TEXT = (
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
_OPTIONAL_TEXT = ("run_id", "message_id", "interaction_id", "command_id")
_REQUIRED_INT = ("iat", "exp", "permission_version")
_TRIGGER_CLAIMS = frozenset({"message_id", "interaction_id", "command_id"})


class TokenVerdict(Enum):
    """The result of one check, in the vocabulary of 09 4.5."""

    VALID = "valid"
    """Every point holds."""
    EXPIRED = "expired"
    """Only point 5 fails because ``exp`` passed: renew (4.5.1); acceptance ignores it."""
    INVALID = "invalid"
    """Points 1, 2, 4, 6 or 9 fail, or ``iat`` is in the future: ``invalid_invocation_token``."""
    UNKNOWN_KEY = "unknown_key"
    """The ``kid`` is missing after the allowed reload: neither accept nor acknowledge."""
    BILLING_MODE_MISMATCH = "billing_mode_mismatch"
    """Point 8: the actual billing mode differs or is unknown."""
    TOOL_CLASS_MISMATCH = "tool_class_mismatch"
    """Point 10: the actual tool class is above the claim."""
    REPLAY = "replay"
    """Point 7: the ``jti`` was seen before and has not expired."""


TOKEN_EVENT_TYPES: frozenset[str] = frozenset(
    {
        "agent.invocation",
        "agent.followup",
        "interaction.created",
        "command.invoked",
        "command.autocomplete_requested",
    }
)
"""The event types that carry an ``invocation_token`` (09 4.4)."""


@dataclass(frozen=True)
class InvocationClaims:
    """What a verified token says; never the token itself.

    Attributes:
        iss: The issuing instance.
        aud: The agent the token is for.
        sub: The triggering principal.
        jti: The token ID.
        iat: Issued at (seconds since the epoch).
        exp: Expiry (seconds since the epoch).
        trigger_kind: ``message``, ``followup``, ``interaction``, ``command`` or ``autocomplete``.
        thread_id: The thread.
        channel_id: The channel.
        space_id: The space of the channel.
        level: The effective level of ``sub`` on the channel.
        billing_mode: The configured billing mode.
        tool_class: The configured tool class.
        permission_version: The permission version at delivery time.
        run_id: The run, when the token has one.
        message_id: The triggering message, when it is one.
        interaction_id: The triggering interaction, when it is one.
        command_id: The triggering command, when it is one.
    """

    iss: str
    aud: str
    sub: UUID
    jti: str
    iat: int
    exp: int
    trigger_kind: str
    thread_id: UUID
    channel_id: UUID
    space_id: UUID
    level: str
    billing_mode: str
    tool_class: str
    permission_version: int
    run_id: UUID | None = None
    message_id: UUID | None = None
    interaction_id: UUID | None = None
    command_id: UUID | None = None


@dataclass(frozen=True)
class Expectation:
    """What the event's payload says (point 6); ``None`` run or thread is not compared.

    Attributes:
        thread_id: The thread of the payload, if it names one.
        run_id: The run of the payload, if it names one.
        trigger_claim: The claim that names the triggering object: ``message_id``,
            ``interaction_id`` or ``command_id`` (empty for a trigger kind this SDK does not
            know).
        trigger_id: The triggering object of the payload; ``None`` when the payload has no
            counterpart, which makes the check ``INVALID`` (fail closed).
    """

    thread_id: UUID | None
    run_id: UUID | None
    trigger_claim: str
    trigger_id: UUID | None


@dataclass(frozen=True)
class Actual:
    """The runtime's actual state for this run (points 8 and 10).

    Attributes:
        billing_mode: A ``BillingMode`` value, or ``unknown`` (never matches).
        tool_class: The tool class actually loaded: ``chat``, ``scoped`` or ``host``.
    """

    billing_mode: str
    tool_class: str


@dataclass(frozen=True)
class TokenCheck:
    """The verdict of one check and, when the payload decoded, the claims.

    Attributes:
        verdict: The verdict.
        claims: The claims; ``None`` when the header, signature or payload was refused or the
            key is unknown.
    """

    verdict: TokenVerdict
    claims: InvocationClaims | None


JwksFetcher = Callable[[], Awaitable[tuple[Mapping[str, Any], float | None]]]
"""Fetches the JWKS: returns its JSON body and the ``max-age`` seconds of ``Cache-Control``
(``None`` when absent)."""

TriggererLoader = Callable[[], Awaitable[AgentTriggerers]]
"""Loads the allow list of 17.7 (``getAgentTriggerers``)."""


@runtime_checkable
class ReplayStore(Protocol):
    """The ``jti`` store of point 7."""

    async def jti_seen(self, jti: str, now: datetime) -> bool:
        """Return whether ``jti`` is remembered beyond ``now``.

        Args:
            jti: The token ID.
            now: The current wall time.
        """
        ...

    async def remember_jti(self, jti: str, until: datetime) -> None:
        """Remember ``jti`` until ``until``.

        Args:
            jti: The token ID.
            until: ``exp`` plus ``adapter.invocation_token_leeway``.
        """
        ...


def compare_actual(claims: InvocationClaims, actual: Actual) -> TokenVerdict:
    """Compare the claims with the runtime's actual state (points 8 and 10 only).

    The adapter asks again at start time, after ``prepare``, with this function.

    Args:
        claims: The verified claims.
        actual: The actual billing mode and tool class.

    Returns:
        ``VALID``, ``BILLING_MODE_MISMATCH`` (also for ``unknown`` or an empty mode) or
        ``TOOL_CLASS_MISMATCH`` (also for a tool class this SDK does not know).
    """
    if actual.billing_mode in ("", "unknown") or actual.billing_mode != claims.billing_mode:
        return TokenVerdict.BILLING_MODE_MISMATCH
    have = _TOOL_CLASS_RANK.get(actual.tool_class)
    allowed = _TOOL_CLASS_RANK.get(claims.tool_class)
    if have is None or allowed is None or have > allowed:
        return TokenVerdict.TOOL_CLASS_MISMATCH
    return TokenVerdict.VALID


def _opt(value: UUID | Unset) -> UUID | None:
    return None if isinstance(value, Unset) else value


def expectation_for(envelope_type: str, event: object) -> Expectation:
    """Return what point 6 compares for an event (Design note 8 of M2-29).

    Rows: ``agent.invocation`` (run, thread, and by ``trigger_kind`` ``message_id``,
    ``interaction_id`` or ``command_id``), ``agent.followup`` (run, thread, ``message_id``),
    ``interaction.created`` (``interaction.id``, its ``run_id`` and ``thread_id``),
    ``command.invoked`` (``command_id``, ``run_id`` and ``thread_id`` when present),
    ``command.autocomplete_requested`` (``command_id``, its thread when present, no run).
    ``agent.invocation`` with ``trigger_kind = command`` compares the payload's ``command_id``
    (M2-16); a payload without one has no counterpart, so its token is ``INVALID``.

    Args:
        envelope_type: The envelope ``type``.
        event: The decoded event (a generated event model with ``payload``).

    Returns:
        The expectation.

    Raises:
        ValueError: The type carries no invocation token.
        TypeError: The event's payload is not the generated payload of that type.
    """
    if envelope_type not in TOKEN_EVENT_TYPES:
        raise ValueError(f"event type {envelope_type} carries no invocation token")
    payload = getattr(event, "payload", None)
    if envelope_type == "agent.invocation" and isinstance(payload, AgentInvocationPayload):
        kind = str(payload.trigger_kind.value)
        claim, trigger = {
            "message": ("message_id", _opt(payload.message_id)),
            "interaction": ("interaction_id", _opt(payload.interaction_id)),
            # M2-16 put command_id into the payload; without it the check fails closed.
            "command": ("command_id", _opt(payload.command_id)),
        }.get(kind, ("", None))
        return Expectation(payload.thread_id, payload.run_id, claim, trigger)
    if envelope_type == "agent.followup" and isinstance(payload, AgentFollowupPayload):
        return Expectation(payload.thread_id, payload.run_id, "message_id", payload.message_id)
    if envelope_type == "interaction.created" and isinstance(payload, InteractionCreatedPayload):
        i = payload.interaction
        return Expectation(_opt(i.thread_id), _opt(i.run_id), "interaction_id", i.id)
    if envelope_type == "command.invoked" and isinstance(payload, CommandInvokedPayload):
        return Expectation(
            _opt(payload.thread_id), _opt(payload.run_id), "command_id", payload.command_id
        )
    if envelope_type == "command.autocomplete_requested" and isinstance(
        payload, CommandAutocompleteRequestedPayload
    ):
        return Expectation(_opt(payload.thread_id), None, "command_id", payload.command_id)
    raise TypeError(f"the payload is not the generated payload of {envelope_type}")


class _RefusedError(Exception):
    """A verdict reached before the end of the check; ``reason`` names the point only."""

    def __init__(
        self, verdict: TokenVerdict, reason: str, claims: InvocationClaims | None = None
    ) -> None:
        super().__init__(reason)
        self.verdict = verdict
        self.reason = reason
        self.claims = claims


def _decode_segment(segment: str) -> bytes:
    # Strict base64url without padding: the alphabet only, no impossible length, and zero
    # trailing bits (the encoding of the decoded bytes must give the segment back).
    if not _B64URL.fullmatch(segment) or len(segment) % 4 == 1:
        raise ValueError("not base64url")
    try:
        data = base64.urlsafe_b64decode(segment + "=" * (-len(segment) % 4))
    except binascii.Error as exc:
        raise ValueError("not base64url") from exc
    if base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii") != segment:
        raise ValueError("not canonical base64url")
    return data


def _no_duplicates(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for name, value in pairs:
        if name in out:
            raise ValueError("duplicate member")
        out[name] = value
    return out


def _refuse_constant(name: str) -> Any:
    raise ValueError(f"not JSON: {name}")


def _object(data: bytes) -> dict[str, Any]:
    # One JSON object, members unique (a parser keeping the first and one keeping the last
    # would read different headers), no trailing data, no NaN or Infinity.
    loaded = json.loads(
        data.decode("utf-8"),
        object_pairs_hook=_no_duplicates,
        parse_constant=_refuse_constant,
    )
    if not isinstance(loaded, dict):
        raise ValueError("not an object")
    return loaded


def _check_header(head: bytes) -> str:
    try:
        members = _object(head)
    except ValueError as exc:  # json.JSONDecodeError and UnicodeDecodeError are ValueErrors
        raise _RefusedError(TokenVerdict.INVALID, "header") from exc
    if members.get("alg") != _ALG:
        raise _RefusedError(TokenVerdict.INVALID, "alg")
    if members.get("typ") != _TYP:
        raise _RefusedError(TokenVerdict.INVALID, "typ")
    if "crit" in members:
        raise _RefusedError(TokenVerdict.INVALID, "crit")
    kid = members.get("kid")
    if not isinstance(kid, str) or not kid:
        raise _RefusedError(TokenVerdict.INVALID, "kid")
    return kid


def _is_int(value: object) -> bool:
    return (
        isinstance(value, int) and not isinstance(value, bool) and _INT64_MIN <= value <= _INT64_MAX
    )


def _decode_claims(body: bytes) -> tuple[InvocationClaims, dict[str, Any]]:
    try:
        members = _object(body)
    except ValueError as exc:
        raise _RefusedError(TokenVerdict.INVALID, "payload") from exc
    for name in _REQUIRED_TEXT:
        value = members.get(name)
        if not isinstance(value, str) or not value:
            raise _RefusedError(TokenVerdict.INVALID, f"claim {name}")
    for name in _OPTIONAL_TEXT:
        if name in members and (not isinstance(members[name], str) or not members[name]):
            raise _RefusedError(TokenVerdict.INVALID, f"claim {name}")
    for name in _REQUIRED_INT:
        if not _is_int(members.get(name)):
            raise _RefusedError(TokenVerdict.INVALID, f"claim {name}")
    if members["tool_class"] not in _TOOL_CLASS_RANK:
        raise _RefusedError(TokenVerdict.INVALID, "claim tool_class")

    def uuid_of(name: str) -> UUID | None:
        value = members.get(name)
        if value is None:
            return None
        try:
            return UUID(value)
        except ValueError as exc:
            raise _RefusedError(TokenVerdict.INVALID, f"claim {name}") from exc

    claims = InvocationClaims(
        iss=members["iss"],
        aud=members["aud"],
        sub=_must(uuid_of("sub")),
        jti=members["jti"],
        iat=members["iat"],
        exp=members["exp"],
        trigger_kind=members["trigger_kind"],
        thread_id=_must(uuid_of("thread_id")),
        channel_id=_must(uuid_of("channel_id")),
        space_id=_must(uuid_of("space_id")),
        level=members["level"],
        billing_mode=members["billing_mode"],
        tool_class=members["tool_class"],
        permission_version=members["permission_version"],
        run_id=uuid_of("run_id"),
        message_id=uuid_of("message_id"),
        interaction_id=uuid_of("interaction_id"),
        command_id=uuid_of("command_id"),
    )
    return claims, members


def stored_claims(token: Secret) -> InvocationClaims:
    """Read the claims of a token this adapter verified before, without checking it again.

    Only for tokens the adapter accepted into its inbox or received from
    ``renewInvocationToken`` over its authenticated connection; never for a delivered token
    that has not passed `InvocationVerifier.check`.

    Args:
        token: The token.

    Returns:
        Its claims.

    Raises:
        ValueError: The token is not a JWS with a readable payload.
    """
    parts = token.reveal().split(".")
    if len(parts) != 3:  # header, payload, signature
        raise ValueError("not a compact JWS")
    try:
        claims, _ = _decode_claims(_decode_segment(parts[1]))
    except _RefusedError as exc:
        raise ValueError("unreadable claims") from exc
    return claims


def _must(value: UUID | None) -> UUID:
    if value is None:  # unreachable: required members were checked before
        raise _RefusedError(TokenVerdict.INVALID, "claim")
    return value


def _usable_key(jwk: object) -> tuple[str, Ed25519PublicKey] | None:
    # Usable: kty OKP, crv Ed25519, a kid, a 32-byte x in strict base64url, and alg and use,
    # when present, EdDSA and sig. Anything else is skipped, not an error of the whole set.
    if not isinstance(jwk, Mapping):
        return None
    kid, x = jwk.get("kid"), jwk.get("x")
    if (
        jwk.get("kty") != "OKP"
        or jwk.get("crv") != "Ed25519"
        or not isinstance(kid, str)
        or not kid
        or not isinstance(x, str)
        or jwk.get("alg") not in (None, "", _ALG)
        or jwk.get("use") not in (None, "", "sig")
    ):
        return None
    try:
        raw = _decode_segment(x)
    except ValueError:
        return None
    if len(raw) != _ED25519_KEY_BYTES:
        return None
    return kid, Ed25519PublicKey.from_public_bytes(raw)


def _until(exp: int, leeway: float) -> datetime:
    try:
        return datetime.fromtimestamp(exp + leeway, UTC)
    except (OverflowError, OSError, ValueError):
        return datetime.max.replace(tzinfo=UTC)


class InvocationVerifier:
    """The check of 09 4.5 for one agent; safe for concurrent use (checks run one at a time).

    Args:
        instance_id: The configured instance (``iss``).
        agent_id: The own agent ID (``aud``).
        jwks: Fetches the JWKS (``getJwks``) with its ``max-age``.
        replay: The ``jti`` store (point 7), usually the inbox store.
        settings: ``adapter.invocation_token_leeway`` and ``adapter.jwks_refresh_min_interval``.
        clock: The time of every check, of the JWKS cache and of the reload throttle.
        allowlist: Loads the allow list of 17.7; ``None`` skips point 9.

    Attributes:
        fetches: How often the JWKS was fetched, forced reloads included (read-only).
        forced_reloads: How many fetches an unknown ``kid`` caused (read-only).
    """

    def __init__(  # noqa: PLR0913 - the binding signature of Design note 8
        self,
        *,
        instance_id: UUID,
        agent_id: UUID,
        jwks: JwksFetcher,
        replay: ReplayStore,
        settings: SdkSettings | None = None,
        clock: Clock | None = None,
        allowlist: TriggererLoader | None = None,
    ) -> None:
        """Create the verifier; see the class docstring."""
        self._iss = str(instance_id)
        self._aud = str(agent_id)
        self._jwks = jwks
        self._replay = replay
        self._settings = settings or SdkSettings()
        self._clock: Clock = clock or SystemClock()
        self._allowlist = allowlist
        self._lock = asyncio.Lock()
        self._keys: dict[str, Ed25519PublicKey] = {}
        self._loaded = False
        self._expires = 0.0
        self._last_forced: float | None = None
        self._fetches = 0
        self._forced = 0
        self._triggerers: frozenset[UUID] | None = None
        self._triggerers_version = 0
        self._triggerers_stale = False
        self._jws = jwt.PyJWS(algorithms=[_ALG])

    @property
    def fetches(self) -> int:
        """How often the JWKS was fetched, forced reloads included."""
        return self._fetches

    @property
    def forced_reloads(self) -> int:
        """How many fetches an unknown ``kid`` caused."""
        return self._forced

    def access_changed(self) -> None:
        """Reload the allow list before the next check (on ``access.changed``)."""
        self._triggerers_stale = True

    async def refresh(self) -> None:
        """Fetch the JWKS now, as at adapter start; it counts as a fetch, not a forced reload.

        Raises:
            Exception: Whatever the fetcher raises, or ``ValueError`` for a body that is not a
                key set.
        """
        async with self._lock:
            await self._fetch()

    async def check(
        self,
        token: Secret,
        expect: Expectation,
        actual: Actual | Callable[[InvocationClaims], Actual],
    ) -> TokenCheck:
        """Check a token against the event's payload and the runtime's actual state.

        Args:
            token: The delivered ``invocation_token``.
            expect: What the event's payload says (`expectation_for`).
            actual: The actual billing mode and tool class, or a function that returns them
                for the claims once signature, ``iss``, ``aud``, the payload and the allow list
                passed (the adapter asks its handler with the verified claims there, so the
                precedence of the points stays that of the reference).

        Returns:
            The verdict and, unless the token form, the signature or the payload was refused or
            the key is unknown, the claims.

        Raises:
            Exception: A failure of the JWKS fetcher or of the allow list loader propagates; the
                event is then neither accepted nor refused.
        """
        try:
            claims = await self._check(token.reveal(), expect, actual)
        except _RefusedError as refused:
            _log.debug("invocation token refused: %s", refused.reason)
            return TokenCheck(refused.verdict, refused.claims)
        return TokenCheck(TokenVerdict.VALID, claims)

    async def _check(
        self,
        text: str,
        expect: Expectation,
        actual: Actual | Callable[[InvocationClaims], Actual],
    ) -> InvocationClaims:
        parts = text.split(".")
        if len(parts) != 3:
            raise _RefusedError(TokenVerdict.INVALID, "not three segments")
        try:
            head, body = _decode_segment(parts[0]), _decode_segment(parts[1])
            _decode_segment(parts[2])
        except ValueError as exc:
            raise _RefusedError(TokenVerdict.INVALID, "segments") from exc
        kid = _check_header(head)
        async with self._lock:
            key = await self._key(kid)
            if key is None:
                raise _RefusedError(TokenVerdict.UNKNOWN_KEY, "kid")
            try:
                self._jws.decode_complete(text, key=key, algorithms=[_ALG])
            except jwt.PyJWTError as exc:
                raise _RefusedError(TokenVerdict.INVALID, "signature") from exc
            claims, members = _decode_claims(body)
            if claims.iss != self._iss or claims.aud != self._aud:
                raise _RefusedError(TokenVerdict.INVALID, "iss or aud", claims)
            _check_expectation(members, expect, claims)
            await self._check_allowlist(claims)
            resolved = actual if isinstance(actual, Actual) else actual(claims)
            verdict = compare_actual(claims, resolved)
            if verdict is not TokenVerdict.VALID:
                raise _RefusedError(verdict, verdict.value, claims)
            return await self._check_time_and_replay(claims)

    async def _check_time_and_replay(self, claims: InvocationClaims) -> InvocationClaims:
        leeway = self._settings.invocation_token_leeway
        now = self._clock.now()
        ts = now.timestamp()
        if claims.iat > ts + leeway or claims.exp <= claims.iat:
            raise _RefusedError(
                TokenVerdict.INVALID, "iat in the future or exp not after iat", claims
            )
        if ts >= claims.exp + leeway:
            raise _RefusedError(TokenVerdict.EXPIRED, "expired", claims)
        if await self._replay.jti_seen(claims.jti, now):
            raise _RefusedError(TokenVerdict.REPLAY, "replay", claims)
        await self._replay.remember_jti(claims.jti, _until(claims.exp, leeway))
        return claims

    async def _check_allowlist(self, claims: InvocationClaims) -> None:
        if self._allowlist is None:
            return
        if (
            self._triggerers is None
            or self._triggerers_stale
            or claims.permission_version > self._triggerers_version
        ):
            loaded = await self._allowlist()
            self._triggerers = frozenset(loaded.principal_ids)
            self._triggerers_version = loaded.permission_version
            self._triggerers_stale = False
        if claims.sub not in self._triggerers:
            raise _RefusedError(TokenVerdict.INVALID, "sub not on the allow list", claims)

    async def _key(self, kid: str) -> Ed25519PublicKey | None:
        now = self._clock.monotonic()
        if not self._loaded or now >= self._expires:
            await self._fetch()
        key = self._keys.get(kid)
        if key is not None:
            return key
        interval = self._settings.jwks_refresh_min_interval
        if self._last_forced is not None and now - self._last_forced < interval:
            return None
        self._last_forced = now
        self._forced += 1
        await self._fetch()
        return self._keys.get(kid)

    async def _fetch(self) -> None:
        self._fetches += 1
        body, max_age = await self._jwks()
        keys = body.get("keys") if isinstance(body, Mapping) else None
        if not isinstance(keys, list):
            raise ValueError("the JWKS body is not a key set")
        usable = dict(k for k in map(_usable_key, keys) if k is not None)
        self._keys, self._loaded = usable, True
        self._expires = self._clock.monotonic() + max(0.0, max_age or 0.0)


def _check_expectation(
    members: Mapping[str, Any], expect: Expectation, claims: InvocationClaims
) -> None:
    # Byte-exact comparison of the claim text with the canonical form of the payload's ID, as
    # the reference does; a missing counterpart in the payload fails closed.
    for want, name in ((expect.run_id, "run_id"), (expect.thread_id, "thread_id")):
        if want is not None and members.get(name) != str(want):
            raise _RefusedError(TokenVerdict.INVALID, name, claims)
    if expect.trigger_id is None or expect.trigger_claim not in _TRIGGER_CLAIMS:
        raise _RefusedError(TokenVerdict.INVALID, "no triggering object in the payload", claims)
    if members.get(expect.trigger_claim) != str(expect.trigger_id):
        raise _RefusedError(TokenVerdict.INVALID, "triggering object", claims)
