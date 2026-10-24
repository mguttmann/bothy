# SPDX-License-Identifier: Apache-2.0
"""Retry policy of the REST client (docs/spec/11-extensions.md 11.2 row ``retry``).

Only idempotent requests are repeated (docs/DECISIONS.md D-M2-29-4): ``GET``, ``HEAD``, ``PUT``
and ``DELETE`` always; ``POST`` and ``PATCH`` only when they carry ``Idempotency-Key`` or match a
route of `EXEMPT_ROUTES` whose reason is ``client_id`` (the ID is in the body) or
``agent_runtime`` (monotone or compare-and-swap, docs/spec/06-api.md 7 point 3). Repeated are
network errors and timeouts, ``5xx`` except ``503 setup_required``, ``429 rate_limited`` and
``409 request_in_progress``. The wait is the larger of ``Retry-After`` and full jitter
``uniform(0, min(cap, base * 2**n))`` with ``sdk.retry_backoff_base`` and
``sdk.retry_backoff_cap``; at most ``sdk.retry_max_attempts`` attempts. Each attempt gets a
fresh DPoP proof through the ``prepare`` hook, because a proof's ``jti`` is single use.

Examples:
    >>> route_of("POST", "/api/v1/runs/0192f4a0-7b1c-7000-8a00-000000000001/start").reason
    'agent_runtime'
"""

from __future__ import annotations

import email.utils
import json
import random
import re
import secrets
import time
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime

import httpx

from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.settings import SdkSettings

__all__ = [
    "API_PREFIX",
    "EXEMPT_ROUTES",
    "ExemptRoute",
    "RetryTransport",
    "SyncRetryTransport",
    "backoff",
    "is_retryable_request",
    "parse_retry_after",
    "route_of",
]

API_PREFIX = "/api/v1"
"""The path prefix of every operation (``servers`` of ``docs/api/openapi.yaml``)."""


@dataclass(frozen=True)
class ExemptRoute:
    """An operation with ``x-idempotency-exempt``: method, path template and reason."""

    method: str
    path: str
    reason: str

    def matches(self, method: str, path: str) -> bool:
        """Tell whether a request path (below `API_PREFIX`) is this route.

        Args:
            method: The HTTP method.
            path: The path without `API_PREFIX`.

        Returns:
            True for a match.
        """
        return method.upper() == self.method and _template(self.path).fullmatch(path) is not None


def _template(path: str) -> re.Pattern[str]:
    return re.compile(re.sub(r"\\\{[^}]+\\\}", "[^/]+", re.escape(path)))


# Every operation of docs/api/openapi.yaml with `x-idempotency-exempt`; tests/test_client.py holds
# this table equal to the specification.
EXEMPT_ROUTES: tuple[ExemptRoute, ...] = (
    ExemptRoute("POST", "/agents/me/consumers/{consumer}/ack", "agent_runtime"),
    ExemptRoute("POST", "/agents/me/heartbeat", "agent_runtime"),
    ExemptRoute("POST", "/app-manifests/validate", "read_only"),
    ExemptRoute("POST", "/audit/verify", "read_only"),
    ExemptRoute("POST", "/auth/enroll/begin", "auth_flow"),
    ExemptRoute("POST", "/auth/enroll/complete", "auth_flow"),
    ExemptRoute("POST", "/auth/enroll/redeem", "auth_flow"),
    ExemptRoute("POST", "/auth/logout", "auth_flow"),
    ExemptRoute("POST", "/auth/oidc/begin", "auth_flow"),
    ExemptRoute("POST", "/auth/oidc/complete", "auth_flow"),
    ExemptRoute("POST", "/auth/pairing/begin", "auth_flow"),
    ExemptRoute("POST", "/auth/pairing/complete", "auth_flow"),
    ExemptRoute("POST", "/auth/passkey/begin", "auth_flow"),
    ExemptRoute("POST", "/auth/passkey/complete", "auth_flow"),
    ExemptRoute("POST", "/auth/password/login", "auth_flow"),
    ExemptRoute("POST", "/auth/step-up/challenge", "auth_flow"),
    ExemptRoute("POST", "/auth/step-up/device-key", "auth_flow"),
    ExemptRoute("POST", "/auth/step-up/oidc", "auth_flow"),
    ExemptRoute("POST", "/auth/step-up/oidc/complete", "auth_flow"),
    ExemptRoute("POST", "/auth/step-up/passkey", "auth_flow"),
    ExemptRoute("POST", "/auth/step-up/password", "auth_flow"),
    ExemptRoute("POST", "/auth/token/refresh", "auth_flow"),
    ExemptRoute("POST", "/auth/trusted-header/login", "auth_flow"),
    ExemptRoute("POST", "/channels/{channel_id}/commands/{command_id}/autocomplete", "read_only"),
    ExemptRoute("POST", "/client-metrics", "telemetry"),
    ExemptRoute("POST", "/command-autocompletions/{request_id}/answer", "agent_runtime"),
    ExemptRoute("POST", "/components/{component_id}/interactions", "client_id"),
    ExemptRoute("POST", "/csp-reports", "telemetry"),
    ExemptRoute("POST", "/files/{file_id}/tickets", "read_only"),
    ExemptRoute("POST", "/grants/preview", "read_only"),
    ExemptRoute("POST", "/hooks/{webhook_id}", "client_id"),
    ExemptRoute("POST", "/hooks/{webhook_id}/{token}", "client_id"),
    ExemptRoute("POST", "/hosts/pair", "auth_flow"),
    ExemptRoute("POST", "/hosts/{host_id}/terminal/sessions", "auth_flow"),
    ExemptRoute("POST", "/imports/{import_id}/batches", "origin_key"),
    ExemptRoute("POST", "/imports/{import_id}/files", "origin_key"),
    ExemptRoute("POST", "/imports/{import_id}/files/{file_id}/complete", "origin_key"),
    ExemptRoute("POST", "/me/identities/begin", "auth_flow"),
    ExemptRoute("POST", "/me/identities/complete", "auth_flow"),
    ExemptRoute("POST", "/me/passkeys/begin", "auth_flow"),
    ExemptRoute("POST", "/me/passkeys/complete", "auth_flow"),
    ExemptRoute("POST", "/me/sessions/extensions", "auth_flow"),
    ExemptRoute("PATCH", "/runs/{run_id}", "agent_runtime"),
    ExemptRoute("POST", "/runs/{run_id}/await-input", "agent_runtime"),
    ExemptRoute("POST", "/runs/{run_id}/finish", "agent_runtime"),
    ExemptRoute("POST", "/runs/{run_id}/invocation-token", "agent_runtime"),
    ExemptRoute("POST", "/runs/{run_id}/resume", "agent_runtime"),
    ExemptRoute("POST", "/runs/{run_id}/start", "agent_runtime"),
    ExemptRoute("POST", "/terminal/sessions/{session_id}/attach", "auth_flow"),
    ExemptRoute("POST", "/threads/{thread_id}/messages", "client_id"),
)

_RETRY_REASONS = frozenset({"client_id", "agent_runtime"})
_IDEMPOTENT_METHODS = frozenset({"GET", "HEAD", "PUT", "DELETE", "OPTIONS"})


def route_of(method: str, path: str) -> ExemptRoute | None:
    """Return the exempt route of a request, if any.

    Args:
        method: The HTTP method.
        path: The request path with or without `API_PREFIX`.

    Returns:
        The route or ``None``.
    """
    if path.startswith(API_PREFIX + "/"):
        path = path[len(API_PREFIX) :]
    # Literal routes win over templated ones of the same shape (for example `/agents/me/...`).
    matches = [r for r in EXEMPT_ROUTES if r.matches(method, path)]
    matches.sort(key=lambda r: r.path.count("{"))
    return matches[0] if matches else None


def is_retryable_request(request: httpx.Request) -> bool:
    """Tell whether the request may be sent again (D-M2-29-4).

    Args:
        request: The request.

    Returns:
        True for an idempotent request.
    """
    method = request.method.upper()
    if method in _IDEMPOTENT_METHODS:
        return True
    if "Idempotency-Key" in request.headers:
        return True
    route = route_of(method, request.url.path)
    return route is not None and route.reason in _RETRY_REASONS


def parse_retry_after(value: str | None, now: datetime | None = None) -> float | None:
    """Read ``Retry-After`` as seconds (delta seconds or an HTTP date).

    Args:
        value: The header value.
        now: The current time for an HTTP date.

    Returns:
        Seconds, never negative, or ``None`` when absent or unreadable.
    """
    if not value:
        return None
    value = value.strip()
    if value.isdigit():
        return float(value)
    try:
        when = email.utils.parsedate_to_datetime(value)
    except (TypeError, ValueError):
        return None
    if when.tzinfo is None:
        when = when.replace(tzinfo=UTC)
    return max(0.0, (when - (now or datetime.now(UTC))).total_seconds())


def backoff(attempt: int, settings: SdkSettings, rng: random.Random) -> float:
    """Return the full-jitter wait before retry number ``attempt`` (0 for the first retry).

    Args:
        attempt: How many retries happened before.
        settings: Base ``sdk.retry_backoff_base`` and cap ``sdk.retry_backoff_cap``.
        rng: The random source.

    Returns:
        ``uniform(0, min(cap, base * 2**attempt))``.
    """
    ceiling = min(settings.retry_backoff_cap, settings.retry_backoff_base * (2**attempt))
    return rng.uniform(0.0, ceiling)


def _problem_code(body: bytes) -> str | None:
    try:
        data = json.loads(body)
    except ValueError:
        return None
    code = data.get("code") if isinstance(data, dict) else None
    return code if isinstance(code, str) else None


def _retry_status(status: int, body: bytes) -> bool:
    if status == httpx.codes.TOO_MANY_REQUESTS:
        return True
    if status == httpx.codes.SERVICE_UNAVAILABLE:
        return _problem_code(body) != "setup_required"
    if status >= httpx.codes.INTERNAL_SERVER_ERROR:
        return True
    return status == httpx.codes.CONFLICT and _problem_code(body) == "request_in_progress"


class _Policy:
    def __init__(
        self,
        settings: SdkSettings,
        rng: random.Random | None,
        prepare: Callable[[httpx.Request], None] | None,
        now: Callable[[], datetime],
    ) -> None:
        self.settings = settings
        self.rng = rng or secrets.SystemRandom()
        self.prepare = prepare
        self.now = now

    def wait(self, attempt: int, response: httpx.Response | None) -> float:
        jitter = backoff(attempt, self.settings, self.rng)
        if response is None:
            return jitter
        after = parse_retry_after(response.headers.get("Retry-After"), self.now())
        return max(jitter, after or 0.0)


class RetryTransport(httpx.AsyncBaseTransport):
    """An ``httpx`` transport that repeats idempotent requests after the policy of this module.

    Args:
        inner: The transport that sends.
        settings: The retry settings.
        clock: The time source of the waits.
        rng: The jitter source (default `secrets.SystemRandom`).
        prepare: Called before every repeated attempt (the client renews the DPoP proof here).
    """

    def __init__(
        self,
        inner: httpx.AsyncBaseTransport,
        settings: SdkSettings,
        *,
        clock: Clock | None = None,
        rng: random.Random | None = None,
        prepare: Callable[[httpx.Request], None] | None = None,
    ) -> None:
        """Create the transport; see the class docstring."""
        self._inner = inner
        self._clock = clock or SystemClock()
        self._policy = _Policy(settings, rng, prepare, self._clock.now)

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        """Send ``request``, repeating it while the policy allows.

        Args:
            request: The request.

        Returns:
            The last response.

        Raises:
            httpx.TransportError: The last attempt failed on the network.
        """
        retryable = is_retryable_request(request)
        attempts = max(1, self._policy.settings.retry_max_attempts)
        for attempt in range(attempts):
            last = attempt == attempts - 1 or not retryable
            if attempt > 0 and self._policy.prepare is not None:
                self._policy.prepare(request)
            try:
                response = await self._inner.handle_async_request(request)
            except httpx.TransportError:
                if last:
                    raise
                await self._clock.sleep(self._policy.wait(attempt, None))
                continue
            if last or response.status_code < httpx.codes.CONFLICT:
                return response
            body = await response.aread()
            if not _retry_status(response.status_code, body):
                return response
            wait = self._policy.wait(attempt, response)
            await response.aclose()
            await self._clock.sleep(wait)
        raise AssertionError("unreachable")  # pragma: no cover

    async def aclose(self) -> None:
        """Close the inner transport."""
        await self._inner.aclose()


class SyncRetryTransport(httpx.BaseTransport):
    """The synchronous counterpart of `RetryTransport` for `bothy_sdk.sync.SyncClient`.

    Args:
        inner: The transport that sends.
        settings: The retry settings.
        sleep: Blocking sleep (default `time.sleep`).
        rng: The jitter source.
        prepare: Called before every repeated attempt.
    """

    def __init__(
        self,
        inner: httpx.BaseTransport,
        settings: SdkSettings,
        *,
        sleep: Callable[[float], None] = time.sleep,
        rng: random.Random | None = None,
        prepare: Callable[[httpx.Request], None] | None = None,
    ) -> None:
        """Create the transport; see the class docstring."""
        self._inner = inner
        self._sleep = sleep
        self._policy = _Policy(settings, rng, prepare, lambda: datetime.now(UTC))

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        """Send ``request``, repeating it while the policy allows.

        Args:
            request: The request.

        Returns:
            The last response.

        Raises:
            httpx.TransportError: The last attempt failed on the network.
        """
        retryable = is_retryable_request(request)
        attempts = max(1, self._policy.settings.retry_max_attempts)
        for attempt in range(attempts):
            last = attempt == attempts - 1 or not retryable
            if attempt > 0 and self._policy.prepare is not None:
                self._policy.prepare(request)
            try:
                response = self._inner.handle_request(request)
            except httpx.TransportError:
                if last:
                    raise
                self._sleep(self._policy.wait(attempt, None))
                continue
            if last or response.status_code < httpx.codes.CONFLICT:
                return response
            body = response.read()
            if not _retry_status(response.status_code, body):
                return response
            wait = self._policy.wait(attempt, response)
            response.close()
            self._sleep(wait)
        raise AssertionError("unreachable")  # pragma: no cover

    def close(self) -> None:
        """Close the inner transport."""
        self._inner.close()
