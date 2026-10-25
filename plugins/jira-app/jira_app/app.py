# SPDX-License-Identifier: Apache-2.0
"""Start and stop of the app (brief M2-50 Design notes 3, 4, 7, 9 and 14).

`start_app` checks the secret files and the state directory, opens the store (and its lock),
creates the SDK client with the app's token (read once), asks ``getMe`` and ``getInstance``,
loads the keys of ``getWebhookKeys``, starts the worker and the HTTP listener, and returns the
listening address with ``stop()``. A failure of any of these aborts the start. The HTTP
transports, the clock, the logger, the chat port and the store are injectable for tests.

After a ``401 unauthenticated`` the endpoints answer ``503`` until ``getMe`` succeeds again,
tried once per ``sdk.retry_backoff_cap``. ``stop()`` stops accepting (later requests get
``503``), lets the running step finish, closes the store and releases the lock.

Examples:
    >>> ROUTES["jira"]
    '/jira/webhook'
"""

from __future__ import annotations

import asyncio
import contextlib
from dataclasses import dataclass, field
from typing import Any

import httpx
from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.client import Client, user_agent_header
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.generated.client import Client as CoreClient
from bothy_sdk.generated.models.instance_limits import InstanceLimits
from bothy_sdk.settings import SdkSettings

from jira_app import APP_VERSION
from jira_app.chat.port import ChatPort
from jira_app.chat.receive import ReceiveContext, events_route, interactions_route, jira_route
from jira_app.chat.sdk_adapter import SdkChatPort
from jira_app.config import (
    AppConfig,
    check_start_files,
    read_jira_credentials,
    read_secret_file,
)
from jira_app.jira.client import JiraClient
from jira_app.log import AppLog
from jira_app.store.base import Store
from jira_app.store.sqlite_store import SqliteStore
from jira_app.web import HttpServer, start_http
from jira_app.worker.jobs import JobDeps, JobRunner
from jira_app.worker.worker import Worker

__all__ = ["ROUTES", "AppDeps", "RunningApp", "start_app"]

ROUTES = {
    "jira": "/jira/webhook",
    "interactions": "/chat/interactions",
    "events": "/chat/events",
}
"""The paths of the app: Jira deliveries, ``delivery.interactions_url``, ``delivery.events_url``."""


@dataclass(frozen=True)
class AppDeps:
    """Injectable dependencies of `start_app`; everything defaults to production.

    Attributes:
        clock: The clock (default the system clock).
        log: The logger (default JSON lines on stdout).
        settings: The SDK settings (``sdk.http_timeout``, ``sdk.retry_*``,
            ``webhook.receiver_tolerance``); tests pass small values.
        chat: The port to the server (default the SDK adapter over the app's token).
        store: The store (default `SqliteStore` in ``jira_app.state_dir``).
        server_transport: The ``httpx`` transport of the SDK clients (tests).
        jira_transport: The ``httpx`` transport of the Jira client (tests).
    """

    clock: Clock | None = None
    log: AppLog | None = None
    settings: SdkSettings | None = None
    chat: ChatPort | None = None
    store: Store | None = None
    server_transport: httpx.AsyncBaseTransport | None = None
    jira_transport: httpx.AsyncBaseTransport | None = None


@dataclass
class RunningApp:
    """A running app.

    Attributes:
        host: The bound host of the listener.
        port: The bound port of the listener.
        url: ``http://<host>:<port>``.
    """

    host: str
    port: int
    url: str
    _worker: Worker
    _stop: Any = field(repr=False)
    _stopped: asyncio.Task[None] | None = field(default=None, repr=False)

    async def idle(self) -> None:
        """Return once the queue is empty and the worker waits (tests)."""
        await self._worker.idle()

    async def stop(self) -> None:
        """Stop accepting, let the running step finish, close the store, release the lock."""
        if self._stopped is None:
            self._stopped = asyncio.ensure_future(self._stop())
        await asyncio.shield(self._stopped)


@dataclass
class _State:
    """What the running app knows: the limits and whether it accepts requests."""

    limits: InstanceLimits
    authenticated: bool = True
    stopping: bool = False


def _sdk_port(config: AppConfig, deps: AppDeps, settings: SdkSettings, clock: Clock) -> SdkChatPort:
    base = config.server_url + "/api/v1"
    dpop = FileDpopKey.load(config.dpop_key_file) if config.dpop_key_file else None
    client = Client(
        base,
        Credentials(Secret.from_file(config.token_file), dpop),
        settings=settings,
        clock=clock,
        user_agent=f"jira-app/{APP_VERSION}",
        transport=deps.server_transport,
    )
    http = httpx.AsyncClient(
        base_url=base,
        timeout=settings.http_timeout,
        transport=deps.server_transport,
        headers={"User-Agent": user_agent_header(f"jira-app/{APP_VERSION}")},
    )
    public = CoreClient(base_url=base).set_async_httpx_client(http)
    return SdkChatPort(client, public)


async def start_app(config: AppConfig, deps: AppDeps | None = None) -> RunningApp:  # noqa: PLR0915
    """Start the app.

    Args:
        config: The loaded configuration.
        deps: Injectable dependencies.

    Returns:
        The running app.

    Raises:
        ConfigError: A secret file or the state directory fails its check.
        StoreLockedError: Another process runs on the state directory.
        ProblemError: ``getMe``, ``getInstance`` or ``getWebhookKeys`` were refused.
        OSError: The address cannot be bound.
    """
    deps = deps or AppDeps()
    clock = deps.clock or SystemClock()
    log = deps.log or AppLog.to_stream()
    log.attach_sdk()
    settings = deps.settings or SdkSettings()
    check_start_files(config)
    store: Store = deps.store or SqliteStore(
        config.state_dir, now=lambda: clock.now().timestamp(), window=config.dedupe_window
    )
    await store.open()
    sdk_port: SdkChatPort | None = None
    jira: JiraClient | None = None
    worker: Worker | None = None
    try:
        if deps.chat is None:
            sdk_port = _sdk_port(config, deps, settings, clock)
        chat: ChatPort = deps.chat or sdk_port  # type: ignore[assignment]
        me = await chat.me()
        own_id = str(me.id)
        limits = await chat.limits()
        await chat.webhook_keys()
        state = _State(limits)
        recheck: asyncio.Task[None] | None = None

        async def recheck_loop() -> None:
            while not state.authenticated and not state.stopping:
                await clock.sleep(settings.retry_backoff_cap)
                with contextlib.suppress(Exception):
                    await chat.me()
                    state.authenticated = True
                    log.info("app.authenticated", reason="get_me")

        def on_unauthenticated() -> None:
            nonlocal recheck
            if not state.authenticated:
                return
            state.authenticated = False
            recheck = asyncio.create_task(recheck_loop())

        async def refresh_limits() -> None:
            state.limits = await chat.limits()

        jira = JiraClient(
            config.jira_base_url,
            lambda: read_jira_credentials(config.jira_credentials_file),
            settings.http_timeout,
            transport=deps.jira_transport,
        )
        runner = JobRunner(
            JobDeps(
                config=config,
                store=store,
                chat=chat,
                jira=jira,
                log=log,
                now=lambda: clock.now().timestamp(),
                limits=lambda: state.limits,
                refresh_limits=refresh_limits,
                on_unauthenticated=on_unauthenticated,
                settings=settings,
            )
        )
        worker = Worker(store, runner.run, clock, settings.retry_backoff_cap, log)
        running = worker
        context = ReceiveContext(
            config=config,
            store=store,
            chat=chat,
            webhook_secret=lambda: read_secret_file(
                config.webhook_secret_file, "webhook_secret_file"
            ).encode("utf-8"),
            own_principal_id=lambda: own_id,
            wake=running.wake,
            clock=clock,
            log=log,
        )
        http: HttpServer = await start_http(
            config.listen_host,
            config.listen_port,
            routes={
                ROUTES["jira"]: jira_route(context),
                ROUTES["interactions"]: interactions_route(context),
                ROUTES["events"]: events_route(context),
            },
            max_body_bytes=config.max_body_bytes,
            ready=lambda: state.authenticated and not state.stopping,
            log=log,
        )
    except BaseException:
        if worker is not None:
            await worker.stop()
        if jira is not None:
            await jira.aclose()
        if sdk_port is not None:
            await sdk_port.aclose()
        await store.close()
        if deps.log is None:
            log.close()
        raise
    running.start()
    log.info("app.started", status=0)
    jira_client = jira

    async def stop() -> None:
        state.stopping = True
        if recheck is not None:
            recheck.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await recheck
        await running.stop()
        await http.close()
        await jira_client.aclose()
        if sdk_port is not None:
            await sdk_port.aclose()
        await store.close()
        log.info("app.stopped", status=0)
        if deps.log is None:
            log.close()

    host = f"[{http.host}]" if ":" in http.host else http.host
    # The app's own listener speaks plain HTTP behind the TLS proxy of the instance (D-M2-50-3).
    url = f"http://{host}:{http.port}"  # NOSONAR
    return RunningApp(http.host, http.port, url, running, stop)
