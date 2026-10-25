# SPDX-License-Identifier: Apache-2.0
"""The HTTP surface of the app (brief M2-50 Design note 4).

One aiohttp application with the routes ``POST /jira/webhook``, ``POST /chat/interactions`` and
``POST /chat/events``; any other path answers ``404``, another method ``405``. Per request, in
this order: while the app is stopping or not authenticated at the server every route answers
``503``; ``Content-Type`` must be ``application/json`` (parameters allowed), else ``415``; the
body is read as bytes up to ``jira_app.max_body_bytes`` (``client_max_size``), a larger body
answers ``413``; then the route's handler decides (signature, JSON, duplicates, job). Bodies
are empty except for the JSON a handler returns. The app speaks plain HTTP behind the
TLS-terminating proxy of the instance; aiohttp keeps its default header limits.

Examples:
    >>> Reply(202).status
    202
"""

from __future__ import annotations

import json
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass
from typing import Any

from aiohttp import web
from multidict import CIMultiDictProxy

from jira_app.log import AppLog

__all__ = ["HttpServer", "RawRequest", "Reply", "RouteHandler", "start_http"]


@dataclass(frozen=True)
class RawRequest:
    """A request after the checks of the surface.

    Attributes:
        headers: The request headers (case-insensitive).
        body: The body exactly as received.
    """

    headers: CIMultiDictProxy[str]
    body: bytes


@dataclass(frozen=True)
class Reply:
    """The answer of a route.

    Attributes:
        status: The status.
        json: A JSON body; ``None`` for an empty body.
    """

    status: int
    json: Any = None


RouteHandler = Callable[[RawRequest], Awaitable[Reply]]
"""A route: decides the answer of a checked request."""


def _is_json(value: str | None) -> bool:
    if value is None:
        return False
    return value.split(";", 1)[0].strip().lower() == "application/json"


def _response(reply: Reply) -> web.Response:
    if reply.json is None:
        return web.Response(status=reply.status)
    return web.Response(
        status=reply.status,
        body=json.dumps(reply.json).encode("utf-8"),
        content_type="application/json",
    )


@dataclass
class HttpServer:
    """A running listener.

    Attributes:
        host: The bound host.
        port: The bound port.
        runner: The aiohttp runner.
    """

    host: str
    port: int
    runner: web.AppRunner

    async def close(self) -> None:
        """Stop accepting and close the listener."""
        await self.runner.cleanup()


def build_app(
    routes: Mapping[str, RouteHandler],
    max_body_bytes: int,
    ready: Callable[[], bool],
    log: AppLog,
) -> web.Application:
    """Build the aiohttp application.

    Args:
        routes: The handler per path; every route takes ``POST`` only.
        max_body_bytes: ``jira_app.max_body_bytes``.
        ready: ``False`` answers ``503`` (stopping, or not authenticated at the server).
        log: The app's logger.

    Returns:
        The application.
    """

    async def dispatch(request: web.Request) -> web.StreamResponse:
        route = routes.get(request.path)
        if route is None:
            return web.Response(status=404)
        if request.method != "POST":
            return web.Response(status=405, headers={"Allow": "POST"})
        if not ready():
            return web.Response(status=503)
        if not _is_json(request.headers.get("Content-Type")):
            return web.Response(status=415)
        try:
            body = await request.read()
        except web.HTTPRequestEntityTooLarge:
            log.warn("http.body_too_large", status=413, reason="body_too_large")
            return web.Response(status=413)
        try:
            reply = await route(RawRequest(request.headers, body))
        except Exception:
            log.error("http.failed", status=503, reason="internal")
            reply = Reply(503)
        return _response(reply)

    app = web.Application(client_max_size=max_body_bytes)
    app.router.add_route("*", "/{tail:.*}", dispatch)
    return app


async def start_http(  # noqa: PLR0913 - the listener's inputs are explicit
    host: str,
    port: int,
    *,
    routes: Mapping[str, RouteHandler],
    max_body_bytes: int,
    ready: Callable[[], bool],
    log: AppLog,
) -> HttpServer:
    """Start the listener.

    Args:
        host: The host of ``jira_app.listen``.
        port: The port (0 lets the system choose).
        routes: The handler per path.
        max_body_bytes: ``jira_app.max_body_bytes``.
        ready: ``False`` answers ``503``.
        log: The app's logger.

    Returns:
        The running server with its bound address.

    Raises:
        OSError: The address cannot be bound.
    """
    runner = web.AppRunner(
        build_app(routes, max_body_bytes, ready, log), access_log=None, handle_signals=False
    )
    await runner.setup()
    site = web.TCPSite(runner, host or None, port)
    try:
        await site.start()
    except OSError:
        await runner.cleanup()
        raise
    address = runner.addresses[0]
    return HttpServer(str(address[0]), int(address[1]), runner)
