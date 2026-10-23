# SPDX-License-Identifier: Apache-2.0
"""Asynchronous REST client of the hand layer on top of the generated core.

`Client` builds one ``httpx.AsyncClient`` with the credentials (`bothy_sdk.auth.DpopAuth`), the
timeout ``sdk.http_timeout``, the retry transport (`bothy_sdk.retry`), the ``User-Agent`` of
docs/spec/06-api.md 2 ("Client identifier"), the acting-for binding of 07 SR9 and the
deprecation warning of 11 12, and hands it to the generated client class. Every REST call of the
hand layer goes through `Client.call` with a generated ``asyncio_detailed`` function, so the hand
layer defines no own request or model type (ADR 0026).

SR9: inside `Client.bind_run` every request of this client carries the run's newest invocation
token in the header ``Invocation-Token`` (``components.parameters.InvocationToken``); outside it,
none does.

Logs (logger ``bothy_sdk``) carry the operation, status, problem code, ``correlation_id`` and the
bound run; never header values, bodies, tokens, proofs or contents (SEC-LOG-2).

Examples:
    >>> import asyncio
    >>> from bothy_sdk.auth import Credentials, Secret
    >>> from bothy_sdk.generated.api.discovery import get_discovery
    >>> async def demo() -> None:
    ...     async with Client("https://example.com/api/v1", Credentials(Secret("t"))) as c:
    ...         assert callable(get_discovery.asyncio_detailed)
    >>> asyncio.run(demo())
"""

from __future__ import annotations

import contextlib
import contextvars
import json
import logging
import platform
import random
import re
import sys
import warnings
from collections.abc import Awaitable, Callable, Iterator
from types import TracebackType
from typing import Any, TypeVar
from urllib.parse import urlsplit, urlunsplit
from uuid import UUID

import httpx

from bothy_sdk.auth import Credentials, DpopAuth, Secret, require_safe_url
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.errors import (
    ApiDeprecationWarning,
    ProblemError,
    problem_error,
    unparsable_problem,
)
from bothy_sdk.generated.api.discovery import get_discovery
from bothy_sdk.generated.client import Client as _CoreClient
from bothy_sdk.generated.models.discovery import Discovery
from bothy_sdk.generated.types import Response
from bothy_sdk.ids import uuid7
from bothy_sdk.retry import API_PREFIX, RetryTransport, parse_retry_after, route_of
from bothy_sdk.settings import SdkSettings

__all__ = [
    "IDEMPOTENCY_KEY_HEADER",
    "INVOCATION_TOKEN_HEADER",
    "Client",
    "expect",
    "path_template",
    "user_agent_header",
]

T = TypeVar("T")
M = TypeVar("M")

INVOCATION_TOKEN_HEADER = "Invocation-Token"  # noqa: S105 - a header name, not a secret  # nosec B105
"""Header of 07 SR9 (``components.parameters.InvocationToken``); a test holds it equal."""

IDEMPOTENCY_KEY_HEADER = "Idempotency-Key"
"""Header of docs/spec/06-api.md 7."""

_LOG = logging.getLogger("bothy_sdk")
_ID_SEGMENT = re.compile(
    r"^(?:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|\d+)$"
)
_VERSION = "0.0.0"


def path_template(path: str) -> str:
    """Return a request path with IDs replaced by ``{id}``, for logs and deprecation warnings.

    Args:
        path: The request path.

    Returns:
        The path with every UUID or number segment as ``{id}``.

    Examples:
        >>> path_template("/api/v1/runs/0192f4a0-7b1c-7000-8a00-000000000001/start")
        '/api/v1/runs/{id}/start'
    """
    return "/".join("{id}" if _ID_SEGMENT.match(s) else s for s in path.split("/"))


def user_agent_header(adapter: str | None = None) -> str:
    """Return the ``User-Agent`` of docs/spec/06-api.md 2.

    Args:
        adapter: ``<adapter name>/<version>`` of the caller, if any.

    Returns:
        ``<adapter> bothy-sdk/<version> (python <major.minor>; <platform>)``.
    """
    own = (
        f"bothy-sdk/{_VERSION} (python {sys.version_info.major}.{sys.version_info.minor}; "
        f"{platform.system().lower() or 'unknown'})"
    )
    return f"{adapter} {own}" if adapter else own


def expect(value: object, model: type[M]) -> M:
    """Narrow the parsed result of `Client.call` to the model of a ``2xx`` answer.

    Args:
        value: The result.
        model: The expected generated model.

    Returns:
        ``value`` typed as ``model``.

    Raises:
        TypeError: The value is of another type (a server answer outside the contract).
    """
    if not isinstance(value, model):
        raise TypeError(f"expected {model.__name__}, got {type(value).__name__}")
    return value


class _Binding:
    """The run a request context is bound to, with its newest token (shared by reference)."""

    __slots__ = ("run_id", "token")

    def __init__(self, run_id: UUID, token: Secret) -> None:
        self.run_id = run_id
        self.token = token


class Client:
    """The asynchronous REST client of an agent adapter.

    Args:
        base_url: The API base, for example ``https://example.com/api/v1``; ``http`` only for a
            loopback host.
        credentials: The agent's API token and optional DPoP key.
        settings: The SDK settings (timeout, retries).
        clock: The time source of retries and proofs.
        user_agent: ``<adapter name>/<version>`` placed before the SDK's own part.
        transport: An ``httpx`` transport (tests pass ``httpx.MockTransport``).
        rng: The jitter source of retries.

    Raises:
        ValueError: ``base_url`` uses clear text off the loopback host.
    """

    def __init__(  # noqa: PLR0913 - the binding signature of Design note 2 plus the jitter source
        self,
        base_url: str,
        credentials: Credentials,
        *,
        settings: SdkSettings | None = None,
        clock: Clock | None = None,
        user_agent: str | None = None,
        transport: httpx.AsyncBaseTransport | None = None,
        rng: random.Random | None = None,
    ) -> None:
        """Create the client; see the class docstring."""
        require_safe_url(base_url)
        self.base_url = base_url.rstrip("/")
        self.settings = settings or SdkSettings()
        self.clock = clock or SystemClock()
        self.auth = DpopAuth(credentials, clock=self.clock)
        self._binding: contextvars.ContextVar[_Binding | None] = contextvars.ContextVar(
            f"bothy_sdk_run_binding_{id(self)}", default=None
        )
        self._idempotency: contextvars.ContextVar[str | None] = contextvars.ContextVar(
            f"bothy_sdk_idempotency_{id(self)}", default=None
        )
        self._bindings: dict[UUID, _Binding] = {}
        self._deprecated: set[tuple[str, str]] = set()
        self._retry = RetryTransport(
            transport or httpx.AsyncHTTPTransport(),
            self.settings,
            clock=self.clock,
            rng=rng,
            prepare=self.auth.apply,
        )
        self._http = httpx.AsyncClient(
            base_url=self.base_url,
            auth=self.auth,
            timeout=self.settings.http_timeout,
            transport=self._retry,
            headers={"User-Agent": user_agent_header(user_agent)},
            event_hooks={"request": [self._on_request], "response": [self._on_response]},
        )
        self._core = _CoreClient(base_url=self.base_url).set_async_httpx_client(self._http)

    @property
    def http(self) -> httpx.AsyncClient:
        """The configured ``httpx`` client (shared with the generated core)."""
        return self._http

    async def __aenter__(self) -> Client:
        """Return the client."""
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        """Close the client."""
        await self.aclose()

    async def aclose(self) -> None:
        """Close the HTTP connections."""
        await self._http.aclose()

    @contextlib.contextmanager
    def bind_run(self, run_id: UUID, token: Secret) -> Iterator[None]:
        """Bind every request of this client in the current context to a run (07 SR9).

        Inside the block each request carries ``Invocation-Token`` with the run's newest token;
        `set_run_token` replaces it for every context bound to the run.

        Args:
            run_id: The run.
            token: The run's newest invocation token.

        Yields:
            Nothing; the binding ends with the block.
        """
        binding = self._bindings.get(run_id)
        if binding is None:
            binding = self._bindings[run_id] = _Binding(run_id, token)
        else:
            binding.token = token
        reset = self._binding.set(binding)
        try:
            yield
        finally:
            self._binding.reset(reset)

    @contextlib.contextmanager
    def unbound(self) -> Iterator[None]:
        """Send the requests inside the block without any run binding (the agent's own writes).

        Yields:
            Nothing.
        """
        reset = self._binding.set(None)
        try:
            yield
        finally:
            self._binding.reset(reset)

    def set_run_token(self, run_id: UUID, token: Secret) -> None:
        """Make ``token`` the token every request bound to ``run_id`` carries from now on.

        Args:
            run_id: The run.
            token: The newer token (a renewal grant or a resume token).
        """
        binding = self._bindings.get(run_id)
        if binding is not None:
            binding.token = token

    def release_run(self, run_id: UUID) -> None:
        """Forget the binding state of an ended run.

        Args:
            run_id: The run.
        """
        self._bindings.pop(run_id, None)

    @property
    def bound_run(self) -> UUID | None:
        """The run the current context is bound to, if any."""
        binding = self._binding.get()
        return binding.run_id if binding is not None else None

    @contextlib.contextmanager
    def idempotency_key(self, key: str) -> Iterator[None]:
        """Use ``key`` as ``Idempotency-Key`` for the writes inside the block.

        The outbox stores the key before the first send and repeats it on every later attempt.

        Args:
            key: The key (a UUIDv7 text).

        Yields:
            Nothing.
        """
        reset = self._idempotency.set(key)
        try:
            yield
        finally:
            self._idempotency.reset(reset)

    async def _on_request(self, request: httpx.Request) -> None:
        binding = self._binding.get()
        if binding is not None:
            request.headers[INVOCATION_TOKEN_HEADER] = binding.token.reveal()
        method = request.method.upper()
        if (
            method in ("POST", "PATCH")
            and IDEMPOTENCY_KEY_HEADER not in request.headers
            and route_of(method, request.url.path) is None
        ):
            # Once per logical call: the retry transport repeats this very request object.
            request.headers[IDEMPOTENCY_KEY_HEADER] = self._idempotency.get() or str(uuid7())

    async def _on_response(self, response: httpx.Response) -> None:
        if response.status_code >= httpx.codes.BAD_REQUEST:
            # Runs before the generated parser: a body it cannot decode becomes a typed error.
            body = await response.aread()
            retry_after = parse_retry_after(response.headers.get("Retry-After"))
            error = unparsable_problem(response.status_code, body, retry_after)
            if error is not None:
                _LOG.info("call status=%d with an undecodable problem body", response.status_code)
                raise error
        if "Deprecation" not in response.headers and "Sunset" not in response.headers:
            return
        key = (response.request.method.upper(), path_template(response.request.url.path))
        if key in self._deprecated:
            return
        self._deprecated.add(key)
        _LOG.warning("deprecated operation %s %s", key[0], key[1])
        warnings.warn(
            f"the server marked {key[0]} {key[1]} as deprecated",
            ApiDeprecationWarning,
            stacklevel=2,
        )

    async def call(self, operation: Callable[..., Awaitable[Response[T]]], /, **kwargs: Any) -> T:
        """Run a generated ``asyncio_detailed`` operation and return its parsed ``2xx`` model.

        Args:
            operation: A generated function, for example ``start_run.asyncio_detailed``.
            **kwargs: Its arguments without ``client``.

        Returns:
            The parsed model of the ``2xx`` answer (``None`` for an answer without body). Use
            `expect` to narrow the union the generated signature declares.

        Raises:
            ProblemError: The answer was not ``2xx``; the subclass follows code and status.
            httpx.TransportError: The network failed after the retries.
        """
        response = await self.call_detailed(operation, **kwargs)
        return response.parsed  # type: ignore[return-value]

    async def call_as(
        self, model: type[M], operation: Callable[..., Awaitable[Response[Any]]], /, **kwargs: Any
    ) -> M:
        """Like `call`, narrowed to the generated model of the ``2xx`` answer.

        Args:
            model: The expected generated model, for example ``Run``.
            operation: A generated ``asyncio_detailed`` function.
            **kwargs: Its arguments without ``client``.

        Returns:
            The parsed answer.

        Raises:
            ProblemError: The answer was not ``2xx``.
            TypeError: The ``2xx`` answer is not of ``model``.
        """
        response = await self.call_detailed(operation, **kwargs)
        return expect(response.parsed, model)

    async def call_detailed(
        self, operation: Callable[..., Awaitable[Response[T]]], /, **kwargs: Any
    ) -> Response[T]:
        """Like `call`, but return the generated ``Response`` with the raw body and headers.

        The polling source reads the delivery fields of each event from the raw body.

        Args:
            operation: A generated ``asyncio_detailed`` function.
            **kwargs: Its arguments without ``client``.

        Returns:
            The generated response of a ``2xx`` answer.

        Raises:
            ProblemError: The answer was not ``2xx``.
        """
        name = getattr(operation, "__module__", "operation").rsplit(".", 1)[-1]
        response = await operation(client=self._core, **kwargs)
        status = int(response.status_code)
        if 200 <= status < 300:
            _LOG.debug("call %s status=%d run=%s", name, status, self.bound_run)
            return response
        error = self._problem(status, response)
        _LOG.info(
            "call %s status=%d code=%s correlation_id=%s run=%s",
            name,
            status,
            error.code,
            error.correlation_id,
            self.bound_run,
        )
        raise error

    @staticmethod
    def _problem(status: int, response: Response[Any]) -> ProblemError:
        try:
            body = json.loads(response.content) if response.content else None
        except ValueError:
            body = None
        retry_after = parse_retry_after(response.headers.get("Retry-After"))
        return problem_error(status, body, retry_after=retry_after)

    async def discover(self) -> Discovery:
        """Read the instance discovery (``getDiscovery``).

        Returns:
            The discovery document.

        Raises:
            ProblemError: The server refused.
        """
        result = await self.call(get_discovery.asyncio_detailed)
        return expect(result, Discovery)

    def origin_url(self, path: str) -> str:
        """Return `url` of ``path`` only when it is on the origin of the API base.

        Every request of the shared HTTP client carries the API token, so a URL the server names
        (for example ``jwks_uri`` of the discovery) must not lead to another origin or to clear
        text.

        Args:
            path: As for `url`.

        Returns:
            The absolute URL.

        Raises:
            ValueError: The URL has another scheme, host or port than the API base.
        """
        url = self.url(path)
        mine, theirs = urlsplit(self.base_url), urlsplit(url)
        if (theirs.scheme, theirs.netloc) != (mine.scheme, mine.netloc):
            raise ValueError("URL is not on the origin of the API base")
        return url

    def url(self, path: str) -> str:
        """Return the absolute URL of an API path or of a URL relative to the origin.

        Args:
            path: An absolute URL, a path starting with ``/api/v1`` (relative to the origin, as
                the discovery names URLs) or a path below the API base.

        Returns:
            The absolute URL.
        """
        if "://" in path:
            return path
        parts = urlsplit(self.base_url)
        if path.startswith(API_PREFIX + "/"):
            return urlunsplit((parts.scheme, parts.netloc, path, "", ""))
        return self.base_url + (path if path.startswith("/") else "/" + path)
