# SPDX-License-Identifier: Apache-2.0
"""A scenario world for a runner that hosts built-in agents (docs/spec/26-builtin-agents.md 3).

`HostingServer` is the FakeServer of the runner's own agent (Lotse there) with the two
operations of A1-03 the runner calls: ``listHostedAgents`` and ``issueHostedAgentToken``, and
the event ``agent.hosting_changed``. Each hosted agent is the agent under test of a FakeServer
of its own (`HostedBackend`), on the same clock and instance: the FakeServer models one agent
principal, so a second principal gets a second server. A token the runner fetches becomes the
only API token that backend accepts, bound to the runner's DPoP key, and fetching the next one
closes the connections of the old one with ``4012``, as the server's revocation path does.
The runner reaches a backend through the test seam ``hosted_url`` of `Runner`; the real server
test (``tests/realserver``) runs the same path against one server.

A backend (`HostedFake`) also serves ``getAgentDefinition`` for its agent (A1-02): the current
definition with its version, ``404`` without one, and `HostedFake.set_definition` appends the
durable event ``agent.definition_changed`` (26 4.1 item 2, A1-05).
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import secrets
from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any
from uuid import UUID

from aiohttp import web
from bothy_sdk.auth import FileDpopKey
from bothy_sdk.ids import uuid7
from bothy_sdk.testing._fake_base import FakeProblem
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.doubles import FakeClock
from bothy_sdk.testing.fakeserver import FakeServer
from scenarios import conftest as scenarios
from scenarios.conftest import World

from claude_code_runner.app import Runner
from claude_code_runner.config import RunnerConfig

_API = "/api/v1"
TOKEN_TTL = 3600.0
"""``agents.hosted_token_ttl`` of the fake, in seconds of the shared clock."""


class HostedFake(FakeServer):
    """The FakeServer of one hosted agent, with ``getAgentDefinition`` of its own agent."""

    def __init__(self, **kwargs: Any) -> None:
        """Create the server without a definition."""
        super().__init__(**kwargs)
        self.definition: Any = None
        self.definition_version = 0
        self.definition_reads = 0

    async def start(self) -> None:
        """Open the listener with the definition route ahead of the FakeServer's own."""
        app = web.Application(client_max_size=4 << 20)
        app.router.add_get(f"{_API}/gateway", self.gateway.handle)
        app.router.add_get(f"{_API}/agents/{{agent_id}}/definition", self._definition)
        app.router.add_route("*", "/{tail:.*}", self._http)
        self._runner = web.AppRunner(app, access_log=None, handle_signals=False)
        await self._runner.setup()
        site = web.TCPSite(self._runner, "127.0.0.1", 0)
        await site.start()
        host, port = self._runner.addresses[0][:2]
        self.origin = f"http://{host}:{port}"

    def set_definition(self, definition: Any, *, announce: bool = True) -> int:
        """Store a new version of the definition and announce ``agent.definition_changed``.

        Args:
            definition: The ``definition`` member as the server would return it.
            announce: Append the durable event for the agent.

        Returns:
            The new version.
        """
        self.definition = definition
        self.definition_version += 1
        if announce:
            world = self.world
            envelope = {
                "id": str(uuid7()),
                "type": "agent.definition_changed",
                "schema_version": 1,
                "occurred_at": world.now_iso(),
                "scope": {},
                "resource": {"kind": "agent", "id": str(self.agent_id)},
                "permission_version": world.permission_version,
                "payload": {"definition_version": self.definition_version},
                "actor": {"id": str(world.robin.id), "kind": "user"},
                "audience": "self",
            }
            world.append("agent.definition_changed", envelope)
        return self.definition_version

    async def _definition(self, request: web.Request) -> web.Response:
        path = request.rel_url.raw_path
        try:
            self._auth.authenticate(
                method=request.method,
                htu=self.origin + path,
                authorization=request.headers.getall("Authorization", []),
                proofs=request.headers.getall("DPoP", []),
            )
            if UUID(request.match_info["agent_id"]) != self.agent_id:
                raise FakeProblem("not_found")
            if self.definition is None:
                raise FakeProblem("not_found")
        except (FakeProblem, ValueError) as problem:
            found = problem if isinstance(problem, FakeProblem) else FakeProblem("not_found")
            return self._problem(found, path, "getAgentDefinition")
        self.definition_reads += 1
        body = {
            "agent_id": str(self.agent_id),
            "version": self.definition_version,
            "origin_kind": "manual",
            "created_at": self.world.now_iso(),
            "created_by": str(self.world.robin.id),
            "definition": self.definition,
        }
        return self._respond(200, body, {"ETag": f'"{self.definition_version}"'}, "")


@dataclasses.dataclass
class HostedBackend:
    """One hosted agent: its record on the runner's server and the FakeServer of its own."""

    fake: HostedFake
    tool_class: str
    tokens: list[str] = dataclasses.field(default_factory=list)

    @property
    def agent_id(self) -> UUID:
        """The hosted agent's principal ID."""
        return self.fake.agent_id

    def record(self, host: UUID) -> dict[str, Any]:
        """The ``Agent`` of ``listHostedAgents`` (26 3.2 item 1)."""
        return {
            "id": str(self.agent_id),
            "handle": f"helper-{str(self.agent_id)[-4:]}",
            "display_name": "Writing Helper",
            "status": "active",
            "adapter_kind": "claude_agent",
            "tool_class": self.tool_class,
            "description": "Helps with drafts",
            "billing_mode": "organization",
            "presence": {"state": "unknown"},
            "operator_config": {},
            "hosted_by": str(host),
            "version": 1,
        }


class HostingServer(FakeServer):
    """The runner's FakeServer with the hosting operations of A1-03 (see the module docstring).

    Args:
        jkt: The thumbprint of the runner's DPoP key; every token is bound to it.
        **kwargs: The options of `FakeServer`.
    """

    def __init__(self, *, jkt: str, **kwargs: Any) -> None:
        """Create the server; the runner's own token is bound to ``jkt``."""
        super().__init__(dpop=True, **kwargs)
        self._auth.jkt = jkt
        self.jkt = jkt
        self.backends: dict[UUID, HostedBackend] = {}
        self.listed: set[UUID] = set()
        self.list_calls = 0
        self.issue_calls: list[UUID] = []
        self.issue_problem: dict[UUID, str] = {}
        self.ttl = TOKEN_TTL

    async def start(self) -> None:
        """Open the listener with the two hosting routes ahead of the FakeServer's own."""
        app = web.Application(client_max_size=4 << 20)
        app.router.add_get(f"{_API}/gateway", self.gateway.handle)
        app.router.add_get(f"{_API}/agents/me/hosted", self._list)
        app.router.add_post(f"{_API}/agents/me/hosted/{{agent_id}}/token", self._issue)
        app.router.add_route("*", "/{tail:.*}", self._http)
        self._runner = web.AppRunner(app, access_log=None, handle_signals=False)
        await self._runner.setup()
        site = web.TCPSite(self._runner, "127.0.0.1", 0)
        await site.start()
        host, port = self._runner.addresses[0][:2]
        self.origin = f"http://{host}:{port}"

    async def add_backend(self, tool_class: str = "scoped") -> HostedBackend:
        """Start the FakeServer of a new hosted agent (not hosted yet, see `host`)."""
        fake = HostedFake(dpop=True, costs=True, clock=self.clock, tool_class=tool_class)
        await fake.start()
        fake.world.instance_id = self.instance_id
        fake.world.issuer.instance_id = self.instance_id
        fake._auth.api_token = "unissued-" + secrets.token_urlsafe(16)
        fake._auth.jkt = self.jkt
        backend = HostedBackend(fake, tool_class)
        self.backends[fake.agent_id] = backend
        return backend

    def host(self, backend: HostedBackend, *, announce: bool = True) -> None:
        """List the agent as hosted by the runner and announce ``agent.hosting_changed``."""
        self.listed.add(backend.agent_id)
        if announce:
            self.announce(backend.agent_id)

    def unhost(self, backend: HostedBackend, *, announce: bool = True) -> None:
        """Remove the agent from the list (its host changed), revoking its tokens."""
        self.listed.discard(backend.agent_id)
        backend.fake._auth.api_token = "revoked-" + secrets.token_urlsafe(16)
        if announce:
            self.announce(backend.agent_id)

    def announce(self, agent_id: UUID) -> None:
        """Append ``agent.hosting_changed`` for the runner (decisions.d/A1-03.md D-A1-03-12)."""
        world = self.world
        envelope = {
            "id": str(uuid7()),
            "type": "agent.hosting_changed",
            "schema_version": 1,
            "occurred_at": world.now_iso(),
            "scope": {},
            "resource": {"kind": "agent", "id": str(self.agent_id)},
            "permission_version": world.permission_version,
            "payload": {"agent_id": str(agent_id)},
            "actor": {"id": str(world.robin.id), "kind": "user"},
            "audience": "self",
        }
        world.append("agent.hosting_changed", envelope)

    def set_tool_class(self, backend: HostedBackend, tool_class: str) -> None:
        """Change the hosted agent's class (``agent.tool_class.set``) on both servers."""
        backend.tool_class = tool_class
        backend.fake.world.tool_class = tool_class

    async def revoke(self, backend: HostedBackend) -> None:
        """Revoke the hosted agent's token and close its connections with ``4012``."""
        backend.fake._auth.api_token = "revoked-" + secrets.token_urlsafe(16)
        await backend.fake.close_connections(4012)

    def _check(self, request: web.Request) -> None:
        self._auth.authenticate(
            method=request.method,
            htu=self.origin + request.rel_url.raw_path,
            authorization=request.headers.getall("Authorization", []),
            proofs=request.headers.getall("DPoP", []),
        )

    async def _list(self, request: web.Request) -> web.Response:
        try:
            self._check(request)
        except FakeProblem as problem:
            return self._problem(problem, request.rel_url.raw_path, "listHostedAgents")
        self.list_calls += 1
        items = [self.backends[a].record(self.agent_id) for a in sorted(self.listed, key=str)]
        return self._respond(200, {"items": items}, {}, "listHostedAgents")

    async def _issue(self, request: web.Request) -> web.Response:
        path = request.rel_url.raw_path
        try:
            self._check(request)
            agent_id = UUID(request.match_info["agent_id"])
            self.issue_calls.append(agent_id)
            code = self.issue_problem.get(agent_id)
            if code is not None:
                raise FakeProblem(code)
            if agent_id not in self.listed:
                raise FakeProblem("not_found")
        except (FakeProblem, ValueError) as problem:
            found = problem if isinstance(problem, FakeProblem) else FakeProblem("not_found")
            return self._problem(found, path, "issueHostedAgentToken")
        backend = self.backends[agent_id]
        secret = "bothy_agt_" + secrets.token_urlsafe(32)
        backend.fake._auth.api_token = secret
        if backend.tokens:
            # The issue revokes the older tokens of the agent from this runner (26 3.3 item 1),
            # which closes their connections with 4012.
            await backend.fake.close_connections(4012)
        backend.tokens.append(secret)
        now = self.clock.now()
        body = {
            "token": {
                "id": str(uuid7()),
                "principal_id": str(agent_id),
                "name": "hosted by lotse",
                "token_prefix": secret[:16],
                "scopes": [],
                "dpop_jkt": self.jkt,
                "expires_at": _iso(now + timedelta(seconds=self.ttl)),
                "created_at": _iso(now),
            },
            "secret": secret,
        }
        return self._respond(201, body, {}, "issueHostedAgentToken")

    async def stop(self) -> None:
        """Stop every backend, then this server."""
        for backend in self.backends.values():
            await backend.fake.stop()
        await super().stop()


def _iso(when: datetime) -> str:
    return when.astimezone(UTC).isoformat().replace("+00:00", "Z")


@dataclasses.dataclass
class HostingWorld(World):
    """The scenario world with a `HostingServer` and the runner's DPoP key."""

    dpop_file: Path = Path()

    @property
    def agents_root(self) -> Path:
        """``runner.hosted_agents_root`` of the world (created by the runner's start check)."""
        return self.root / "agents"

    @property
    def server(self) -> HostingServer:
        """The runner's server."""
        assert isinstance(self.fake, HostingServer)
        return self.fake

    def config(self, **runner: Any) -> RunnerConfig:
        """`World.config` with the DPoP key and ``runner.hosted_agents`` on (unless given)."""
        values: dict[str, Any] = {
            "dpop_key_ref": f"file:{self.dpop_file}",
            "hosted_agents": True,
            "hosted_max_tool_class": "scoped",
            "hosted_agents_root": str(self.agents_root),
        }
        values.update(runner)
        return super().config(**values)

    def hosted_url(self, agent_id: UUID) -> str:
        """The origin of a hosted agent's server (the test seam of `Runner`)."""
        return self.server.backends[agent_id].fake.origin

    async def start(
        self, config: RunnerConfig | None = None, wrap: Any = None, **env: str
    ) -> Runner:
        """Start the runner with the seam and wait for its first heartbeat."""
        del wrap
        runner = Runner(
            config or self.config(),
            self.environ(**env),
            clock=self.fake.clock,
            hosted_url=self.hosted_url,
        )
        await runner.build()
        task = asyncio.ensure_future(runner.run())
        self.runners.append((runner, task))
        await self.until(
            lambda: any(h.get("details", {}).get("adapter") for h in self.fake.heartbeats)
        )
        return runner

    def all_files(self) -> list[tuple[Path, bytes]]:
        """Every file under the world's folder (state, projects, home), with its content."""
        return scenarios.live_files(self.root.rglob("*"))

    def issued_secrets(self) -> list[str]:
        """Every hosted token the server issued."""
        return [t for b in self.server.backends.values() for t in b.tokens]


@contextlib.asynccontextmanager
async def hosting_world(
    root: Path, timings: dict[str, float] | None = None
) -> AsyncIterator[HostingWorld]:
    """Build a `HostingWorld` in ``root`` (like the scenario world, with the runner's key)."""
    root.mkdir(mode=0o700)
    projects = root / "projects"
    (projects / "website" / ".git").mkdir(parents=True)
    home = root / "home"
    (home / ".claude").mkdir(parents=True)
    bin_dir = root / "bin"
    capture = root / "capture.jsonl"
    claude = scenarios.helper.install(bin_dir, {"fixture": "happy"}, capture)
    key_file = root / "api_key"
    key_file.write_text(scenarios.API_KEY + "\n")
    key_file.chmod(0o600)
    dpop_file = root / "dpop.pem"
    key = FileDpopKey.generate(dpop_file)
    server = HostingServer(jkt=key.jkt, costs=True, timings=timings, clock=FakeClock())
    await server.start()
    token_file = root / "agent_token"
    token_file.write_text(server.credentials.api_token.reveal())
    token_file.chmod(0o600)
    w = HostingWorld(
        fake=server,
        root=root,
        projects=projects,
        state=root / "state",
        home=home,
        bin_dir=bin_dir,
        capture=capture,
        key_file=key_file,
        token_file=token_file,
        forum_a=server.add_forum("Projekt-A", "website", prompt="Answer in short sentences."),
        forum_b=server.add_forum("Projekt-B", "docs"),
        werkzeuge=server.add_forum("Werkzeuge", None),
        claude=claude,
        dpop_file=dpop_file,
    )
    pump = asyncio.ensure_future(scenarios._pump(server))
    try:
        yield w
    finally:
        for runner, task in list(w.runners):
            await runner.stop()
            with contextlib.suppress(Exception):
                await asyncio.wait_for(task, PATIENCE)
        pump.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await pump
        await server.stop()
