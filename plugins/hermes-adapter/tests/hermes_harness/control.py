# SPDX-License-Identifier: Apache-2.0
"""The control channel between the harness and the probe plugin.

:class:`ControlServer` listens on ``127.0.0.1`` port 0. The probe posts every call it
receives from Hermes as ``{"method", "args"}`` (``POST /call``), asks for the next command
(``GET /command``, ``204`` when there is none) and posts each command's result
(``POST /result``). The server keeps calls in arrival order and serves each command once, in
the order the harness pushed them.

Example:
    >>> with ControlServer() as control:
    ...     command = control.push("describe")
    ...     surfaces = control.result(command)
"""

from __future__ import annotations

import itertools
import json
import threading
from collections import deque
from collections.abc import Callable
from typing import Any

from hermes_harness._server import LoopbackServer, QuietHandler
from hermes_harness.limits import STEP_DEADLINE_S

Predicate = Callable[[dict[str, Any]], bool]


class _Handler(QuietHandler):
    @property
    def control(self) -> ControlServer:
        owner: ControlServer = self.server.owner  # type: ignore[attr-defined]
        return owner

    def _json(self) -> Any:
        try:
            return json.loads(self.read_body() or b"null")
        except json.JSONDecodeError:
            return None

    def do_GET(self) -> None:  # noqa: N802 - http.server dispatch name
        """Serve the next command."""
        if self.path != "/command":
            self.reply(404)
            return
        command = self.control.take()
        if command is None:
            self.reply(204)
        else:
            self.reply(200, json.dumps(command).encode(), "application/json")

    def do_POST(self) -> None:  # noqa: N802 - http.server dispatch name
        """Record a call or a command result."""
        data = self._json()
        if not isinstance(data, dict):
            self.reply(400)
            return
        if self.path == "/call" and isinstance(data.get("method"), str):
            self.control.add_call(data["method"], data.get("args") or {})
            self.reply(204)
        elif self.path == "/result" and isinstance(data.get("id"), str):
            self.reply(204 if self.control.set_result(data["id"], data.get("result")) else 404)
        else:
            self.reply(400 if self.path in {"/call", "/result"} else 404)


class ControlServer(LoopbackServer):
    """Control channel endpoint on ``127.0.0.1`` port 0."""

    def __init__(self) -> None:
        """Bind the endpoint."""
        super().__init__(_Handler)
        self._cond = threading.Condition()
        self._calls: list[dict[str, Any]] = []
        self._commands: deque[dict[str, Any]] = deque()
        self._results: dict[str, Any] = {}
        self._pending: dict[str, str] = {}
        self._ids = itertools.count(1)

    def add_call(self, method: str, args: dict[str, Any]) -> None:
        """Append one call of the probe."""
        with self._cond:
            self._calls.append({"seq": len(self._calls) + 1, "method": method, "args": args})
            self._cond.notify_all()

    def calls(self) -> list[dict[str, Any]]:
        """All calls in arrival order."""
        with self._cond:
            return [dict(c) for c in self._calls]

    def mark(self) -> int:
        """The number of calls so far; pass it as ``after`` to wait for later calls only."""
        with self._cond:
            return len(self._calls)

    def push(self, name: str, **args: Any) -> str:
        """Queue a command for the probe.

        Args:
            name: Command name (``inject``, ``interrupt``, ``session``, ``credential_type``,
                ``describe``).
            **args: The command's arguments.

        Returns:
            The command ID, for :meth:`result`.
        """
        with self._cond:
            command_id = f"c{next(self._ids)}"
            self._commands.append({"id": command_id, "name": name, "args": args})
            self._pending[command_id] = name
            return command_id

    def take(self) -> dict[str, Any] | None:
        """Hand out the next command once (used by the request handler)."""
        with self._cond:
            return self._commands.popleft() if self._commands else None

    def set_result(self, command_id: str, result: Any) -> bool:
        """Store the result of a command; false for an unknown ID."""
        with self._cond:
            if command_id not in self._pending:
                return False
            self._results[command_id] = result
            self._cond.notify_all()
            return True

    def result(self, command_id: str, *, deadline_s: float = STEP_DEADLINE_S) -> Any:
        """Wait for a command's result.

        Raises:
            TimeoutError: The probe posted no result before the deadline.
        """
        with self._cond:
            if not self._cond.wait_for(lambda: command_id in self._results, deadline_s):
                name = self._pending.get(command_id, "?")
                raise TimeoutError(f"no result for command {name} within the step deadline")
            return self._results[command_id]

    def wait_for(
        self,
        method: str,
        predicate: Predicate | None = None,
        *,
        after: int = 0,
        deadline_s: float = STEP_DEADLINE_S,
    ) -> dict[str, Any]:
        """Wait for the first call of ``method`` after call number ``after``.

        Args:
            method: The method name.
            predicate: Optional test on the call's arguments.
            after: Only calls with a higher sequence number count (see :meth:`mark`).
            deadline_s: The named deadline of the step.

        Returns:
            The matching call.

        Raises:
            TimeoutError: No matching call arrived before the deadline.
        """

        def found() -> dict[str, Any] | None:
            for call in self._calls[after:]:
                if call["method"] == method and (predicate is None or predicate(call["args"])):
                    return call
            return None

        with self._cond:
            if not self._cond.wait_for(lambda: found() is not None, deadline_s):
                raise TimeoutError(f"no call {method} within the step deadline")
            call = found()
            assert call is not None  # noqa: S101 - guaranteed by wait_for
            return dict(call)
