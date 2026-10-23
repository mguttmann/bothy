# SPDX-License-Identifier: Apache-2.0
"""Synchronous hull of the REST client for scripts (docs/spec/11-extensions.md 11.3).

`SyncClient` offers what `bothy_sdk.client.Client` offers, with the generated ``sync_detailed``
functions: the same credentials and DPoP proofs, timeout ``sdk.http_timeout``, retry policy,
automatic ``Idempotency-Key``, ``User-Agent``, deprecation warning and SR9 binding. Adapters use
the asynchronous client; this hull is for scripts and tools.

Examples:
    >>> from bothy_sdk.auth import Credentials, Secret
    >>> with SyncClient("https://example.com/api/v1", Credentials(Secret("t"))) as c:
    ...     c.base_url
    'https://example.com/api/v1'
"""

from __future__ import annotations

import contextlib
import contextvars
import json
import logging
import random
import time
import warnings
from collections.abc import Callable, Iterator
from types import TracebackType
from typing import Any, TypeVar
from uuid import UUID

import httpx

from bothy_sdk.auth import Credentials, DpopAuth, Secret, require_safe_url
from bothy_sdk.client import (
    IDEMPOTENCY_KEY_HEADER,
    INVOCATION_TOKEN_HEADER,
    expect,
    path_template,
    user_agent_header,
)
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
from bothy_sdk.retry import SyncRetryTransport, parse_retry_after, route_of
from bothy_sdk.settings import SdkSettings

__all__ = ["SyncClient"]

T = TypeVar("T")
M = TypeVar("M")

_LOG = logging.getLogger("bothy_sdk")


class SyncClient:
    """The synchronous REST client.

    Args:
        base_url: The API base; ``http`` only for a loopback host.
        credentials: The API token and optional DPoP key.
        settings: Timeout and retries.
        user_agent: ``<name>/<version>`` before the SDK's own part.
        transport: An ``httpx`` transport (tests pass ``httpx.MockTransport``).
        rng: The jitter source of retries.
        sleep: The blocking sleep of retries.

    Raises:
        ValueError: ``base_url`` uses clear text off the loopback host.
    """

    def __init__(  # noqa: PLR0913 - the asynchronous client's inputs plus a blocking sleep
        self,
        base_url: str,
        credentials: Credentials,
        *,
        settings: SdkSettings | None = None,
        user_agent: str | None = None,
        transport: httpx.BaseTransport | None = None,
        rng: random.Random | None = None,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        """Create the client; see the class docstring."""
        require_safe_url(base_url)
        self.base_url = base_url.rstrip("/")
        self.settings = settings or SdkSettings()
        self.auth = DpopAuth(credentials)
        self._binding: contextvars.ContextVar[tuple[UUID, Secret] | None] = contextvars.ContextVar(
            f"bothy_sdk_sync_binding_{id(self)}", default=None
        )
        self._deprecated: set[tuple[str, str]] = set()
        self._http = httpx.Client(
            base_url=self.base_url,
            auth=self.auth,
            timeout=self.settings.http_timeout,
            transport=SyncRetryTransport(
                transport or httpx.HTTPTransport(),
                self.settings,
                sleep=sleep,
                rng=rng,
                prepare=self.auth.apply,
            ),
            headers={"User-Agent": user_agent_header(user_agent)},
            event_hooks={"request": [self._on_request], "response": [self._on_response]},
        )
        self._core = _CoreClient(base_url=self.base_url).set_httpx_client(self._http)

    def __enter__(self) -> SyncClient:
        """Return the client."""
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        """Close the client."""
        self.close()

    def close(self) -> None:
        """Close the HTTP connections."""
        self._http.close()

    @contextlib.contextmanager
    def bind_run(self, run_id: UUID, token: Secret) -> Iterator[None]:
        """Bind every request inside the block to a run (07 SR9, header ``Invocation-Token``).

        Args:
            run_id: The run.
            token: The run's invocation token.

        Yields:
            Nothing.
        """
        reset = self._binding.set((run_id, token))
        try:
            yield
        finally:
            self._binding.reset(reset)

    def _on_request(self, request: httpx.Request) -> None:
        binding = self._binding.get()
        if binding is not None:
            request.headers[INVOCATION_TOKEN_HEADER] = binding[1].reveal()
        method = request.method.upper()
        if (
            method in ("POST", "PATCH")
            and IDEMPOTENCY_KEY_HEADER not in request.headers
            and route_of(method, request.url.path) is None
        ):
            request.headers[IDEMPOTENCY_KEY_HEADER] = str(uuid7())

    def _on_response(self, response: httpx.Response) -> None:
        if response.status_code >= httpx.codes.BAD_REQUEST:
            body = response.read()
            retry_after = parse_retry_after(response.headers.get("Retry-After"))
            error = unparsable_problem(response.status_code, body, retry_after)
            if error is not None:
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

    def call(self, operation: Callable[..., Response[T]], /, **kwargs: Any) -> T:
        """Run a generated ``sync_detailed`` operation and return its parsed ``2xx`` model.

        Args:
            operation: A generated function, for example ``get_run.sync_detailed``.
            **kwargs: Its arguments without ``client``.

        Returns:
            The parsed model of the ``2xx`` answer.

        Raises:
            ProblemError: The answer was not ``2xx``.
        """
        response = operation(client=self._core, **kwargs)
        status = int(response.status_code)
        if 200 <= status < 300:
            return response.parsed  # type: ignore[return-value]
        try:
            body = json.loads(response.content) if response.content else None
        except ValueError:
            body = None
        error: ProblemError = problem_error(
            status, body, retry_after=parse_retry_after(response.headers.get("Retry-After"))
        )
        _LOG.info(
            "call status=%d code=%s correlation_id=%s", status, error.code, error.correlation_id
        )
        raise error

    def call_as(
        self, model: type[M], operation: Callable[..., Response[Any]], /, **kwargs: Any
    ) -> M:
        """Like `call`, narrowed to ``model``.

        Args:
            model: The expected generated model.
            operation: A generated ``sync_detailed`` function.
            **kwargs: Its arguments.

        Returns:
            The parsed answer.

        Raises:
            ProblemError: The answer was not ``2xx``.
            TypeError: The answer is not of ``model``.
        """
        return expect(self.call(operation, **kwargs), model)

    def discover(self) -> Discovery:
        """Read the instance discovery.

        Returns:
            The discovery document.
        """
        return self.call_as(Discovery, get_discovery.sync_detailed)
