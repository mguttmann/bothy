# SPDX-License-Identifier: Apache-2.0
"""The Fake-Jira API of the tests (brief M2-50 Design note 18).

An aiohttp server on ``127.0.0.1`` port 0 with the two endpoints of the app's Jira client:
``POST /rest/api/3/issue`` and ``POST /rest/api/3/issue/<id>/comment``. It checks the Basic
credentials (``401`` without the test credentials), ``Accept``, ``Content-Type`` and
``User-Agent``; answers ``400`` for an unknown project, an empty summary, an issue type other
than the configured one, and a description or comment body that is not an ADF document of
``type`` ``doc`` and ``version`` 1; ``404`` for a comment on an unknown issue; otherwise ``201``
with increasing IDs from ``10001`` and keys ``<PROJECT>-<n>``. It records method, path, JSON
body and header names in memory. Faults per call: ``500``, ``400``, a ``302`` to a trap
listener (which must never receive a request), no answer until the fake stops (later than the
client's time limit), and a dropped connection.

`FakeJira.deliver` renders a fixture with overrides, signs it with HMAC-SHA256 in hex like Jira
and posts it to the app with ``X-Hub-Signature``; options omit, alter or truncate the signature,
use another method, add a second header or set a raw header value. With `FakeJira.auto_echo`,
every issue created through the fake is followed by its ``jira:issue_created`` delivery, and
every comment by its ``comment_created`` delivery and a ``jira:issue_updated`` delivery without
``changelog``.
"""

from __future__ import annotations

import asyncio
import base64
import copy
import hashlib
import hmac
import json
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import httpx
from aiohttp import web

FIXTURES = Path(__file__).resolve().parents[1] / "fixtures" / "jira"
_KEEP: Any = object()
# The bound of one delivery to the app; never reached on a healthy run.
DELIVERY_PATIENCE = 120.0


def load_fixture(name: str) -> dict[str, Any]:
    """A fixture of ``tests/fixtures/jira`` by its event name (``jira:issue_created``...)."""
    file = name.replace("jira:", "jira_") + ".json"
    loaded: dict[str, Any] = json.loads((FIXTURES / file).read_text(encoding="utf-8"))
    return loaded


def render_fixture(  # noqa: PLR0912 - every override of Design note 18
    name: str,
    *,
    issue_id: str | None = None,
    key: str | None = None,
    project: str | None = None,
    summary: str | None = None,
    text: Any = _KEEP,
    comment_id: str | None = None,
    actor: str | None = None,
    timestamp: int | None = None,
    changelog: Any = _KEEP,
) -> dict[str, Any]:
    """A fixture with overrides of IDs, key, project, texts, actor and ``timestamp``."""
    data = copy.deepcopy(load_fixture(name))
    issue = data["issue"]
    if issue_id is not None:
        issue["id"] = issue_id
    if key is not None:
        issue["key"] = key
        if project is None:
            project = key.split("-", 1)[0]
    if project is not None:
        issue["fields"]["project"]["key"] = project
    if summary is not None:
        issue["fields"]["summary"] = summary
    if timestamp is not None:
        data["timestamp"] = timestamp
    if "comment" in data:
        if comment_id is not None:
            data["comment"]["id"] = comment_id
        if actor is not None:
            data["comment"]["author"]["displayName"] = actor
        if text is not _KEEP:
            data["comment"]["body"] = text
    else:
        if actor is not None:
            data["user"]["displayName"] = actor
        if text is not _KEEP:
            issue["fields"]["description"] = text
    if changelog is not _KEEP:
        if changelog is None:
            data.pop("changelog", None)
        else:
            data["changelog"] = {"id": "10101", "items": changelog}
    return data


@dataclass
class Recorded:
    """One request the fake received."""

    method: str
    path: str
    body: Any
    header_names: list[str]
    headers: dict[str, str] = field(repr=False, default_factory=dict)


class FakeJira:
    """The Fake-Jira API (create it with `FakeJira.start`)."""

    def __init__(self, credentials: str, secret: str, projects: dict[str, str]) -> None:
        self.credentials = credentials
        self.secret = secret
        self.projects = dict(projects)
        self.requests: list[Recorded] = []
        self.trap_requests: list[Recorded] = []
        self.faults: list[str] = []
        self.known_issues: dict[str, str] = {"10012": "SUP-12", "10013": "SUP-13"}
        self.auto_echo: str | None = None
        self.echo_before_answer = False
        self.url = ""
        self.trap_url = ""
        self._next_id = 10001
        self._numbers: dict[str, int] = {}
        self._release = asyncio.Event()
        self._echoes: list[asyncio.Task[int]] = []
        self._runners: list[web.AppRunner] = []
        self._http = httpx.AsyncClient(timeout=DELIVERY_PATIENCE)
        self.echo_statuses: list[int] = []

    @classmethod
    async def start(
        cls,
        *,
        credentials: str,
        secret: str,
        projects: dict[str, str] | None = None,
    ) -> FakeJira:
        """Start the fake and its trap listener on ``127.0.0.1`` port 0."""
        fake = cls(credentials, secret, projects or {"SUP": "Task", "WEB": "Bug"})
        app = web.Application()
        app.router.add_route("*", "/{tail:.*}", fake._handle)
        fake.url = await fake._listen(app)
        trap = web.Application()
        trap.router.add_route("*", "/{tail:.*}", fake._trap)
        fake.trap_url = await fake._listen(trap)
        return fake

    async def _listen(self, app: web.Application) -> str:
        runner = web.AppRunner(app, access_log=None, handle_signals=False)
        await runner.setup()
        site = web.TCPSite(runner, "127.0.0.1", 0)
        await site.start()
        self._runners.append(runner)
        host, port = runner.addresses[0][:2]
        return f"http://{host}:{port}"

    async def stop(self) -> None:
        """Release held answers, wait for echoes and stop both listeners."""
        self._release.set()
        try:
            await self.echoes_done()
        finally:
            await self._http.aclose()
            for runner in self._runners:
                await runner.cleanup()

    def fail(self, *kinds: str) -> None:
        """Queue faults for the next calls: ``500``, ``400``, ``302``, ``hang``, ``drop``."""
        self.faults.extend(kinds)

    def issues(self) -> list[Recorded]:
        """The recorded issue creations."""
        return [r for r in self.requests if r.path == "/rest/api/3/issue"]

    def comments(self) -> list[Recorded]:
        """The recorded comment creations."""
        return [r for r in self.requests if r.path.endswith("/comment")]

    def pending_echoes(self) -> bool:
        """Whether an echo delivery was started and not yet awaited."""
        return bool(self._echoes)

    async def echoes_done(self) -> None:
        """Wait until every echo delivery was answered."""
        while self._echoes:
            tasks, self._echoes = self._echoes, []
            self.echo_statuses.extend(await asyncio.gather(*tasks))

    # -- the REST endpoints -----------------------------------------------------------------

    async def _trap(self, request: web.Request) -> web.StreamResponse:
        self.trap_requests.append(Recorded(request.method, request.path, None, []))
        return web.Response(status=599)

    def _authorized(self, request: web.Request) -> bool:
        expected = "Basic " + base64.b64encode(self.credentials.encode()).decode()
        return hmac.compare_digest(request.headers.get("Authorization", ""), expected)

    async def _handle(self, request: web.Request) -> web.StreamResponse:
        raw = await request.read()
        try:
            body = json.loads(raw) if raw else None
        except ValueError:
            body = None
        self.requests.append(
            Recorded(
                request.method,
                request.path,
                body,
                sorted(h.lower() for h in request.headers),
                dict(request.headers),
            )
        )
        if self.faults:
            fault = self.faults.pop(0)
            answer = await self._fault(request, fault)
            if answer is not None:
                return answer
        if request.method != "POST":
            return web.Response(status=405)
        if not self._authorized(request):
            return web.Response(status=401)
        if (
            request.headers.get("Accept") != "application/json"
            or request.headers.get("Content-Type") != "application/json"
            or not request.headers.get("User-Agent", "").startswith("jira-app/")
        ):
            return web.Response(status=400)
        if request.path == "/rest/api/3/issue":
            return await self._create_issue(body)
        match = re.fullmatch(r"/rest/api/3/issue/([0-9]+)/comment", request.path)
        if match is not None:
            return await self._create_comment(match.group(1), body)
        return web.Response(status=404)

    async def _fault(self, request: web.Request, fault: str) -> web.StreamResponse | None:
        if fault == "500":
            return web.json_response({"errorMessages": ["internal"]}, status=500)
        if fault == "400":
            return web.json_response({"errorMessages": ["refused"]}, status=400)
        if fault == "302":
            return web.Response(status=302, headers={"Location": self.trap_url + request.path})
        if fault == "hang":
            await self._release.wait()
            return web.Response(status=201)
        if fault == "drop":
            assert request.transport is not None
            request.transport.close()
            return web.Response(status=599)
        return None

    def _is_adf(self, value: Any) -> bool:
        return isinstance(value, dict) and value.get("type") == "doc" and value.get("version") == 1

    async def _create_issue(self, body: Any) -> web.StreamResponse:
        fields = body.get("fields") if isinstance(body, dict) else None
        if not isinstance(fields, dict):
            return web.Response(status=400)
        project = (fields.get("project") or {}).get("key")
        issue_type = (fields.get("issuetype") or {}).get("name")
        summary = fields.get("summary")
        if (
            project not in self.projects
            or issue_type != self.projects[project]
            or not isinstance(summary, str)
            or not summary.strip()
            or not self._is_adf(fields.get("description"))
        ):
            return web.Response(status=400)
        issue_id = str(self._next_id)
        self._next_id += 1
        self._numbers[project] = self._numbers.get(project, 0) + 1
        key = f"{project}-{self._numbers[project]}"
        self.known_issues[issue_id] = key
        answer = {"id": issue_id, "key": key, "self": f"{self.url}/rest/api/3/issue/{issue_id}"}
        if self.auto_echo is not None:
            echo = render_fixture(
                "jira:issue_created", issue_id=issue_id, key=key, summary=summary, actor="Sam"
            )
            await self._echo(echo)
        return web.json_response(answer, status=201)

    async def _create_comment(self, issue_id: str, body: Any) -> web.StreamResponse:
        if issue_id not in self.known_issues:
            return web.Response(status=404)
        if not isinstance(body, dict) or not self._is_adf(body.get("body")):
            return web.Response(status=400)
        comment_id = str(self._next_id)
        self._next_id += 1
        if self.auto_echo is not None:
            key = self.known_issues[issue_id]
            await self._echo(
                render_fixture(
                    "comment_created",
                    issue_id=issue_id,
                    key=key,
                    comment_id=comment_id,
                    text=body["body"],
                    actor="Sam",
                )
            )
            await self._echo(
                render_fixture("jira:issue_updated", issue_id=issue_id, key=key, changelog=None)
            )
        return web.json_response({"id": comment_id}, status=201)

    async def _echo(self, payload: dict[str, Any]) -> None:
        target = self.auto_echo
        assert target is not None
        delivery = self.deliver(target, payload)
        if self.echo_before_answer:
            self.echo_statuses.append(await delivery)
        else:
            self._echoes.append(asyncio.ensure_future(delivery))

    # -- deliveries to the app -------------------------------------------------------------

    async def deliver(
        self,
        url: str,
        payload: dict[str, Any] | bytes,
        *,
        secret: str | None = None,
        method: str = "sha256",
        signature: str | None = None,
        omit: bool = False,
        alter: bool = False,
        truncate: bool = False,
        second_header: bool = False,
        content_type: str = "application/json",
    ) -> int:
        """Sign ``payload`` like Jira and post it to ``url``; return the status."""
        body = payload if isinstance(payload, bytes) else json.dumps(payload).encode("utf-8")
        key = (secret if secret is not None else self.secret).encode("utf-8")
        digest = getattr(hashlib, method.lower(), hashlib.sha256)
        value = f"{method}={hmac.new(key, body, digest).hexdigest()}"
        if alter:
            last = value[-1]
            value = value[:-1] + ("0" if last != "0" else "1")
        if truncate:
            value = value[:-8]
        if signature is not None:
            value = signature
        headers: list[tuple[str, str]] = [("Content-Type", content_type)]
        if not omit:
            headers.append(("X-Hub-Signature", value))
            if second_header:
                headers.append(("X-Hub-Signature", value))
        response = await self._http.post(url, content=body, headers=httpx.Headers(headers))
        return response.status_code
