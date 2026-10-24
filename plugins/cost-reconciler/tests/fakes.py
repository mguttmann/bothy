# SPDX-License-Identifier: Apache-2.0
"""Test doubles: a fake cost API, a fake server, a fake clock and a fake stop signal.

Both HTTP fakes are `http.server.ThreadingHTTPServer` instances on 127.0.0.1 port 0 that record
every request (method, path, query, headers, body). No test reaches the network.
"""

from __future__ import annotations

import json
import threading
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import UTC, datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlsplit

FIXTURES = Path(__file__).parent / "fixtures"
COST_REPORT_PATH = "/v1/organizations/cost_report"
ESTIMATES_PATH = "/api/v1/costs/estimates"
RECONCILIATIONS_PATH = "/api/v1/costs/reconciliations"


def load_fixture(name: str) -> Any:
    """Return the parsed JSON fixture `name` of tests/fixtures."""
    return json.loads((FIXTURES / name).read_text(encoding="utf-8"))


@dataclass(frozen=True)
class Recorded:
    """One request a fake received."""

    method: str
    path: str
    query: dict[str, list[str]]
    headers: dict[str, str]
    body: bytes

    def text(self) -> str:
        """Every recorded part as one text, for canary searches."""
        return " ".join(
            [
                self.method,
                self.path,
                json.dumps(self.query),
                json.dumps(self.headers),
                self.body.decode("utf-8", errors="replace"),
            ]
        )

    def json(self) -> Any:
        """The body parsed as JSON."""
        return json.loads(self.body)


@dataclass(frozen=True)
class Reply:
    """One answer of a fake."""

    status: int
    body: bytes = b""
    headers: dict[str, str] = field(default_factory=dict)

    @classmethod
    def of_json(cls, status: int, value: Any, content_type: str = "application/json") -> Reply:
        """A JSON answer."""
        return cls(status, json.dumps(value).encode(), {"Content-Type": content_type})


Responder = Callable[[Recorded], Reply]


class FakeHTTP:
    """A recording HTTP server on 127.0.0.1 port 0 whose answers come from a responder."""

    def __init__(self, responder: Responder) -> None:
        self.responder = responder
        self.requests: list[Recorded] = []
        self._lock = threading.Lock()
        outer = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def _handle(self) -> None:
                length = int(self.headers.get("Content-Length") or 0)
                body = self.rfile.read(length) if length else b""
                parts = urlsplit(self.path)
                recorded = Recorded(
                    method=self.command,
                    path=parts.path,
                    query=parse_qs(parts.query, keep_blank_values=True),
                    headers={k.lower(): v for k, v in self.headers.items()},
                    body=body,
                )
                with outer._lock:
                    outer.requests.append(recorded)
                reply = outer.responder(recorded)
                self.send_response(reply.status)
                for name, value in reply.headers.items():
                    self.send_header(name, value)
                self.send_header("Content-Length", str(len(reply.body)))
                self.end_headers()
                self.wfile.write(reply.body)

            # The names http.server dispatches to.
            do_GET = _handle  # noqa: N815
            do_POST = _handle  # noqa: N815

            def log_message(self, format: str, *args: Any) -> None:
                del format, args

        self._server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self._server.daemon_threads = True
        self._thread = threading.Thread(
            target=self._server.serve_forever, args=(0.02,), daemon=True
        )
        self._thread.start()

    @property
    def url(self) -> str:
        """The base address, `http://127.0.0.1:<port>`."""
        host, port = self._server.server_address[:2]
        return f"http://{host!s}:{port}"

    def close(self) -> None:
        """Stop the server and wait for its thread."""
        self._server.shutdown()
        self._server.server_close()
        self._thread.join()


class FakeCostApi(FakeHTTP):
    """The provider's cost report endpoint serving fixture pages.

    `pages` maps the value of the query parameter `page` (None for the first page) to a page.
    `override`, when set, answers every request instead.
    """

    def __init__(self, pages: dict[str | None, Any] | None = None) -> None:
        self.pages: dict[str | None, Any] = (
            pages
            if pages is not None
            else {
                None: load_fixture("cost_report_page1.json"),
                "page_2": load_fixture("cost_report_page2.json"),
            }
        )
        self.override: Reply | None = None
        super().__init__(self._respond)

    def _respond(self, request: Recorded) -> Reply:
        if self.override is not None:
            return self.override
        if request.method != "GET" or request.path != COST_REPORT_PATH:
            return Reply.of_json(404, {"type": "error"})
        page = request.query.get("page", [None])[0]
        if page not in self.pages:
            return Reply.of_json(400, {"type": "error"})
        return Reply.of_json(200, self.pages[page])


class FakeServer(FakeHTTP):
    """The server's two cost operations.

    `estimates` maps the `from` parameter of a request to the items of that range; items are
    filtered by the requested `agent_id` values and paged by `limit` with an index cursor.
    `submit_reply`, when set, answers every submission; otherwise the answer counts every
    entry as reconciled, except the run IDs in `unmatched`.
    """

    def __init__(self) -> None:
        self.estimates: dict[str, list[dict[str, Any]]] = {}
        self.submit_reply: Reply | None = None
        self.estimates_reply: Reply | None = None
        self.unmatched: set[str] = set()
        FakeHTTP.__init__(self, self._respond)

    def _respond(self, request: Recorded) -> Reply:
        if request.method == "GET" and request.path == ESTIMATES_PATH:
            if self.estimates_reply is not None:
                return self.estimates_reply
            return self._estimates(request)
        if request.method == "POST" and request.path == RECONCILIATIONS_PATH:
            if self.submit_reply is not None:
                return self.submit_reply
            entries = request.json()["entries"]
            unmatched = [e["run_id"] for e in entries if e["run_id"] in self.unmatched]
            return Reply.of_json(
                200,
                {"reconciled": len(entries) - len(unmatched), "unmatched_run_ids": unmatched},
            )
        return Reply.of_json(404, {"code": "not_found"}, "application/problem+json")

    def _estimates(self, request: Recorded) -> Reply:
        agents = set(request.query.get("agent_id", []))
        start = request.query["from"][0]
        items = [i for i in self.estimates.get(start, []) if i["agent_id"] in agents]
        items.sort(key=lambda i: str(i["run_id"]))
        limit = int(request.query["limit"][0])
        offset = int(request.query.get("cursor", ["0"])[0])
        page: dict[str, Any] = {"items": items[offset : offset + limit]}
        if offset + limit < len(items):
            page["next_cursor"] = str(offset + limit)
        return Reply.of_json(200, page)

    def submissions(self) -> list[Recorded]:
        """The recorded submissions in arrival order."""
        return [r for r in self.requests if r.path == RECONCILIATIONS_PATH]


class FakeClock:
    """A monotonic clock and a sleep function that advance a counter instead of waiting."""

    def __init__(self, wall: datetime | None = None) -> None:
        self.now = 1000.0
        self.sleeps: list[float] = []
        self.wall = wall or datetime(2026, 10, 21, 5, 30, tzinfo=UTC)

    def monotonic(self) -> float:
        """The current reading."""
        return self.now

    def sleep(self, seconds: float) -> None:
        """Record the sleep and advance the clock."""
        self.sleeps.append(seconds)
        self.now += seconds

    def wall_now(self) -> datetime:
        """The fixed wall time."""
        return self.wall


class FakeStop:
    """A stop signal whose wait advances the fake clock; it sets itself after `passes` waits."""

    def __init__(self, clock: FakeClock, stop_after_waits: int | None = None) -> None:
        self.clock = clock
        self.waits: list[float] = []
        self._set = False
        self._limit = stop_after_waits

    def is_set(self) -> bool:
        """Whether a stop was requested."""
        return self._set

    def set(self) -> None:
        """Request the stop."""
        self._set = True

    def wait(self, timeout: float) -> bool:
        """Record the wait, advance the clock and return whether the stop is set."""
        self.waits.append(timeout)
        self.clock.now += timeout
        if self._limit is not None and len(self.waits) >= self._limit:
            self._set = True
        return self._set


@contextmanager
def fakes() -> Iterator[tuple[FakeCostApi, FakeServer]]:
    """Start a fake cost API and a fake server and stop both afterwards."""
    cost_api = FakeCostApi()
    try:
        server = FakeServer()
        try:
            yield cost_api, server
        finally:
            server.close()
    finally:
        cost_api.close()
