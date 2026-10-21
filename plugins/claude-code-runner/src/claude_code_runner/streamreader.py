# SPDX-License-Identifier: Apache-2.0
"""Tail the stream file of a turn line by line (design note 12, 09 18.4).

The reader keeps a partial last line until it is complete, skips lines that are not JSON
objects and messages of a type it does not know (counted per reason in a debug line without
content), and stops after ``result`` or when the process has exited and the file is read to its
end. It polls the file every ``runner.poll_interval`` (stream in a file, design note 11).

Examples:
    >>> sorted(KNOWN_TYPES)
    ['assistant', 'result', 'system', 'tool_progress', 'user']
"""

from __future__ import annotations

import asyncio
import json
import logging
from collections import Counter
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any

__all__ = ["KNOWN_TYPES", "StreamReader"]

_LOG = logging.getLogger("claude_code_runner")

KNOWN_TYPES = frozenset({"system", "assistant", "user", "tool_progress", "result"})
"""Message types of the stream the runner reads; others are skipped."""


class StreamReader:
    """Reads the messages of one stream file.

    Args:
        path: The stream file of the turn.
        poll_interval: ``runner.poll_interval``.
        alive: Tells whether the turn process still runs.
    """

    def __init__(self, path: Path, poll_interval: float, alive: Callable[[], bool]) -> None:
        """Create the reader; nothing is read before `messages`."""
        self.path = path
        self.poll_interval = poll_interval
        self.alive = alive
        self.skipped: Counter[str] = Counter()

    async def messages(self) -> AsyncIterator[dict[str, Any]]:
        """Yield every known message in order until ``result`` or the end of the turn.

        Yields:
            One decoded stream message.
        """
        offset = 0
        partial = b""
        try:
            while True:
                running = self.alive()
                chunk = self._read(offset)
                offset += len(chunk)
                partial += chunk
                *lines, partial = partial.split(b"\n")
                for raw in lines:
                    message = self._decode(raw)
                    if message is None:
                        continue
                    yield message
                    if message["type"] == "result":
                        return
                if not chunk and not running:
                    return
                if not chunk:
                    await asyncio.sleep(self.poll_interval)
        finally:
            if self.skipped:
                _LOG.debug(json.dumps({"event": "stream_skipped", "counts": dict(self.skipped)}))

    def _read(self, offset: int) -> bytes:
        try:
            with self.path.open("rb") as fh:
                fh.seek(offset)
                return fh.read()
        except FileNotFoundError:
            return b""

    def _decode(self, raw: bytes) -> dict[str, Any] | None:
        if not raw.strip():
            return None
        try:
            message = json.loads(raw)
        except ValueError:
            self.skipped["not_json"] += 1
            return None
        if not isinstance(message, dict):
            self.skipped["not_object"] += 1
            return None
        kind = message.get("type")
        if kind not in KNOWN_TYPES:
            self.skipped["unknown_type"] += 1
            return None
        return message
