# SPDX-License-Identifier: Apache-2.0
"""A local stand-in for the Messages API, for the free checks against the real CLI.

The pinned CLI runs against this server on ``127.0.0.1`` (``ANTHROPIC_BASE_URL`` on loopback, a
dummy key, its own ``HOME`` and ``CLAUDE_CONFIG_DIR``): no login, no model call, no cost. The
named pipe test and the recordings of ``record.py`` use it.

Without a script every model request gets one short text answer (``ok <n>``). A script decides
the answer of each request from its body: content blocks (``text`` and ``tool_use``), or an
`ApiError` the server sends with its status. The script runs under a lock, because a workflow
asks for several agents at once.
"""

from __future__ import annotations

import json
import os
import threading
import uuid
from collections.abc import Callable, Iterator, Mapping, Sequence
from contextlib import contextmanager
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

__all__ = ["ApiError", "StandIn", "cli_env", "private_ancestors", "stand_in", "text", "user_line"]

DUMMY_KEY = "sk-ant-stand-in-not-a-key"
"""The key the CLI sends to the stand-in; no API knows it."""


@dataclass(frozen=True)
class ApiError:
    """An error answer of the stand-in, in the form of the API's error body.

    Attributes:
        status: The HTTP status.
        kind: ``error.type`` of the body.
        message: ``error.message`` of the body.
        details: ``error.details`` of the body, if any (the API names a typed error kind there
            as ``error_code``).
    """

    status: int
    kind: str
    message: str
    details: Mapping[str, Any] | None = field(default=None)

    def body(self) -> dict[str, Any]:
        """Return the JSON body."""
        error: dict[str, Any] = {"type": self.kind, "message": self.message}
        if self.details is not None:
            error["details"] = dict(self.details)
        return {"type": "error", "error": error, "request_id": "req_stand_in"}


Answer = Sequence[Mapping[str, Any]] | ApiError
Script = Callable[[Mapping[str, Any]], Answer]


def text(value: str) -> dict[str, Any]:
    """Return a ``text`` content block."""
    return {"type": "text", "text": value}


class StandIn(BaseHTTPRequestHandler):
    """The handler: one answer per model request, as a stream or as one JSON document."""

    script: Script | None
    usage: Mapping[str, int]
    answers: list[str]
    paths: list[str]
    lock: threading.Lock

    def log_message(self, *_: Any) -> None:
        """Quiet."""

    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length") or 0)
        body = json.loads(self.rfile.read(length) or b"{}")
        path = self.path.split("?", 1)[0]
        self.paths.append(path)
        if path.endswith("/count_tokens"):
            self._json({"input_tokens": 10})
            return
        if not path.endswith("/v1/messages"):
            self._json({"type": "error"}, status=404)
            return
        with self.lock:
            answer: Answer
            if self.script is None:
                answer = [text(f"ok {len(self.answers) + 1}")]
            else:
                answer = self.script(body)
            if isinstance(answer, ApiError):
                self.answers.append(f"error {answer.status}")
            else:
                self.answers.append(
                    " ".join(str(b.get("text") or b.get("name") or "") for b in answer)
                )
        if isinstance(answer, ApiError):
            self._json(answer.body(), status=answer.status)
            return
        self._message(body, [dict(block) for block in answer])

    def _message(self, body: Mapping[str, Any], content: list[dict[str, Any]]) -> None:
        tool_use = any(block["type"] == "tool_use" for block in content)
        stop_reason = "tool_use" if tool_use else "end_turn"
        message = {
            "id": f"msg_{uuid.uuid4().hex[:12]}",
            "type": "message",
            "role": "assistant",
            "model": body.get("model", "model-small"),
            "content": content,
            "stop_reason": stop_reason,
            "stop_sequence": None,
            "usage": dict(self.usage),
        }
        if not body.get("stream"):
            self._json(message)
            return
        start = {**message, "content": [], "stop_reason": None}
        events: list[tuple[str, dict[str, Any]]] = [
            ("message_start", {"type": "message_start", "message": start})
        ]
        for index, block in enumerate(content):
            if block["type"] == "tool_use":
                opened = {**block, "input": {}}
                delta = {"type": "input_json_delta", "partial_json": json.dumps(block["input"])}
            else:
                opened = {"type": "text", "text": ""}
                delta = {"type": "text_delta", "text": block["text"]}
            events += [
                (
                    "content_block_start",
                    {"type": "content_block_start", "index": index, "content_block": opened},
                ),
                (
                    "content_block_delta",
                    {"type": "content_block_delta", "index": index, "delta": delta},
                ),
                ("content_block_stop", {"type": "content_block_stop", "index": index}),
            ]
        events += [
            (
                "message_delta",
                {
                    "type": "message_delta",
                    "delta": {"stop_reason": stop_reason, "stop_sequence": None},
                    "usage": {"output_tokens": self.usage.get("output_tokens", 2)},
                },
            ),
            ("message_stop", {"type": "message_stop"}),
        ]
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        for name, data in events:
            self.wfile.write(f"event: {name}\ndata: {json.dumps(data)}\n\n".encode())
        self.wfile.flush()

    def do_GET(self) -> None:
        self.paths.append(self.path.split("?", 1)[0])
        self._json({}, status=404)

    def do_HEAD(self) -> None:
        self.paths.append(self.path.split("?", 1)[0])
        self.send_response(200)
        self.end_headers()

    def _json(self, value: Any, status: int = 200) -> None:
        data = json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


@contextmanager
def stand_in(
    script: Script | None = None, usage: Mapping[str, int] | None = None
) -> Iterator[tuple[str, type[StandIn]]]:
    """Serve the stand-in on a free loopback port.

    Args:
        script: Decides the answer of each model request; ``None`` answers ``ok <n>``.
        usage: The ``usage`` of every answer (default 10 input and 2 output tokens).

    Yields:
        The base URL and the handler class with ``answers`` and ``paths``.
    """
    members = {
        "script": staticmethod(script) if script is not None else None,
        "usage": dict(usage or {"input_tokens": 10, "output_tokens": 2}),
        "answers": [],
        "paths": [],
        "lock": threading.Lock(),
    }
    handler = type("Handler", (StandIn,), members)
    server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_address[1]}", handler
    finally:
        server.shutdown()
        server.server_close()


def cli_env(root: Path, url: str) -> dict[str, str]:
    """Return the environment of a CLI process that talks to the stand-in only.

    Args:
        root: A folder of the caller; ``home`` and ``config`` are created in it.
        url: The base URL of `stand_in`.

    Returns:
        The environment: an empty ``HOME``, its own ``CLAUDE_CONFIG_DIR``, the dummy key and
        the switches that keep the CLI from any other traffic.
    """
    (root / "config").mkdir(exist_ok=True)
    (root / "home").mkdir(exist_ok=True)
    return {
        "HOME": str(root / "home"),
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "CLAUDE_CONFIG_DIR": str(root / "config"),
        "ANTHROPIC_BASE_URL": url,
        "ANTHROPIC_API_KEY": DUMMY_KEY,
        "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
        "DISABLE_TELEMETRY": "1",
        "DISABLE_AUTOUPDATER": "1",
    }


def user_line(value: str) -> tuple[str, bytes]:
    """Return the ``uuid`` and the stdin line of one user message."""
    line_uuid = str(uuid.uuid4())
    doc = {
        "type": "user",
        "uuid": line_uuid,
        "message": {"role": "user", "content": [text(value)]},
    }
    return line_uuid, json.dumps(doc).encode() + b"\n"


def private_ancestors(folder: Path) -> list[Path]:
    """Return what the CLI would read above a working folder: instruction files and settings.

    The CLI loads ``CLAUDE.md`` and ``.claude`` of every folder above its working directory,
    whatever ``HOME`` and ``CLAUDE_CONFIG_DIR`` say. A recording must not start below such a
    folder, or a developer's own instructions reach the prompt and the recording.

    Args:
        folder: The working folder of a CLI process.

    Returns:
        The paths found above it (empty for a folder that is safe to use).
    """
    found = []
    for parent in folder.resolve().parents:
        found += [p for p in (parent / "CLAUDE.md", parent / ".claude") if p.exists()]
    return found
