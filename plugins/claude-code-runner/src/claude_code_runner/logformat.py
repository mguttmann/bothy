# SPDX-License-Identifier: Apache-2.0
"""The log of the command line: one JSON object per line on stderr.

Every line has ``ts`` (RFC 3339 UTC with milliseconds), ``level`` and ``logger``. The runner's
own messages are JSON objects with an ``event``; their fields are kept as they are. A message
of a library that is not JSON becomes the field ``message``. The request lines of ``httpx`` and
``httpcore`` (one per REST call, with the full URL) and the frame dumps of ``websockets`` (they
can hold the API token) are not logged; their warnings are.

Example:
    >>> import io
    >>> configure(io.StringIO())
"""

from __future__ import annotations

import json
import logging
from datetime import UTC, datetime
from typing import TextIO

__all__ = ["QUIET_LOGGERS", "JsonLineFormatter", "configure"]

QUIET_LOGGERS = ("httpx", "httpcore", "websockets")
"""Libraries that log only from level WARNING on."""


class JsonLineFormatter(logging.Formatter):
    """Format a record as one JSON object with ``ts``, ``level`` and ``logger``."""

    def format(self, record: logging.LogRecord) -> str:  # noqa: D102
        stamp = datetime.fromtimestamp(record.created, UTC).isoformat(timespec="milliseconds")
        line: dict[str, object] = {
            "ts": stamp.replace("+00:00", "Z"),
            "level": record.levelname.lower(),
            "logger": record.name,
        }
        message = record.getMessage()
        try:
            parsed = json.loads(message)
        except ValueError:
            parsed = None
        if isinstance(parsed, dict):
            line.update(parsed)
        else:
            line["message"] = message
        if record.exc_info:
            line["traceback"] = self.formatException(record.exc_info)
        return json.dumps(line)


def configure(stream: TextIO) -> None:
    """Send every logger of the process to ``stream`` as JSON lines from level INFO on.

    Args:
        stream: The target, ``sys.stderr`` in the command line.
    """
    handler = logging.StreamHandler(stream)
    handler.setFormatter(JsonLineFormatter())
    root = logging.getLogger()
    root.handlers[:] = [handler]
    root.setLevel(logging.INFO)
    for name in QUIET_LOGGERS:
        logging.getLogger(name).setLevel(logging.WARNING)
