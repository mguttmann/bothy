# SPDX-License-Identifier: Apache-2.0
"""JSON log lines with an attribute allow list (brief M2-50 Design note 16).

Every line is one JSON object with ``time``, ``level``, ``msg`` (a fixed code such as
``jira.delivery.accepted``) and only the attributes of `LOG_ATTRIBUTES`. Values of other names,
and values that are not strings, integers or booleans, are dropped, so a summary, description,
comment, form value, display name, e-mail address, Jira account ID, issue or project key, URL,
header value, token, secret, signature or file content can never reach a line
(SEC-LOG-2, SEC-LOG-4, SEC-SEC-3). The SDK's logger ``bothy_sdk`` goes through the same
formatter as the code ``sdk.client`` without its text.

Examples:
    >>> import io
    >>> out = io.StringIO()
    >>> log = AppLog.to_stream(out)
    >>> log.info("jira.delivery.accepted", status=202, summary="never logged")
    >>> '"summary"' in out.getvalue(), '"status": 202' in out.getvalue()
    (False, True)
    >>> log.close()
"""

from __future__ import annotations

import itertools
import json
import logging
import sys
from datetime import UTC, datetime
from typing import IO, Any

__all__ = ["LOG_ATTRIBUTES", "SDK_LOGGER", "AppLog", "JsonLineFormatter"]

LOG_ATTRIBUTES: tuple[str, ...] = (
    "event",
    "delivery_id",
    "webhook_id",
    "job_id",
    "job_kind",
    "step",
    "operation",
    "status",
    "outcome",
    "reason",
    "code",
    "issue_id",
    "thread_id",
    "channel_id",
)
"""The only attributes a line may carry besides ``time``, ``level`` and ``msg``."""

SDK_LOGGER = "bothy_sdk"
"""The logger of the SDK; its records are written as ``sdk.client`` without their text."""

_ALLOWED = frozenset(LOG_ATTRIBUTES)
_LEVELS = {
    logging.DEBUG: "debug",
    logging.INFO: "info",
    logging.WARNING: "warn",
    logging.ERROR: "error",
    logging.CRITICAL: "error",
}
_FIELDS = "app_fields"
_SEQUENCE = itertools.count(1)


def allowed_fields(fields: dict[str, Any]) -> dict[str, str | int | bool]:
    """Return the members of ``fields`` that the allow list permits.

    Args:
        fields: Attributes of one line.

    Returns:
        Only allowed names whose values are strings, integers or booleans.
    """
    out: dict[str, str | int | bool] = {}
    for key, value in fields.items():
        if key in _ALLOWED and isinstance(value, str | int | bool):
            out[key] = value
    return out


class JsonLineFormatter(logging.Formatter):
    """Formats a record as one JSON line of Design note 16."""

    def format(self, record: logging.LogRecord) -> str:
        """Return the JSON line of ``record`` without a trailing line break.

        Args:
            record: A record of the app's logger or of ``bothy_sdk``.

        Returns:
            The line.
        """
        fields = getattr(record, _FIELDS, None)
        own = isinstance(fields, dict)
        attributes: dict[str, Any] = fields if isinstance(fields, dict) else {}
        line: dict[str, Any] = {
            "time": datetime.fromtimestamp(record.created, UTC).isoformat(),
            "level": _LEVELS.get(record.levelno, "info"),
            # A record of the SDK keeps only its level: its text may name paths or codes the
            # allow list does not cover.
            "msg": str(record.msg) if own else "sdk.client",
        }
        if own:
            line.update(allowed_fields(attributes))
        return json.dumps(line, ensure_ascii=False)


class AppLog:
    """The app's logger: one call per line with a fixed code and allowed attributes.

    Each instance owns a logger of its own (``jira_app.<n>``), so several apps in one test
    process never share lines; the SDK's records reach every attached instance.

    Args:
        handler: The handler that receives the formatted lines.
        level: The lowest level that is written.
    """

    def __init__(self, handler: logging.Handler, level: int = logging.INFO) -> None:
        """Create the logger; see the class docstring."""
        handler.setFormatter(JsonLineFormatter())
        self._handler = handler
        self._logger = logging.getLogger(f"jira_app.{next(_SEQUENCE)}")
        self._logger.propagate = False
        self._logger.setLevel(level)
        self._logger.addHandler(handler)
        self._sdk_level = level

    @classmethod
    def to_stream(cls, stream: IO[str] | None = None, level: int = logging.INFO) -> AppLog:
        """Return a logger that writes lines to ``stream`` (default stdout).

        Args:
            stream: The text stream.
            level: The lowest level.

        Returns:
            The logger.
        """
        return cls(logging.StreamHandler(stream or sys.stdout), level)

    def attach_sdk(self) -> None:
        """Send the records of ``bothy_sdk`` through this logger's handler."""
        sdk = logging.getLogger(SDK_LOGGER)
        if self._handler not in sdk.handlers:
            sdk.addHandler(self._handler)
        if sdk.level == logging.NOTSET or sdk.level > self._sdk_level:
            sdk.setLevel(self._sdk_level)
        sdk.propagate = False

    def close(self) -> None:
        """Detach the handler from both loggers and flush it."""
        logging.getLogger(SDK_LOGGER).removeHandler(self._handler)
        self._logger.removeHandler(self._handler)
        self._handler.flush()

    def _emit(self, level: int, msg: str, fields: dict[str, Any]) -> None:
        self._logger.log(level, msg, extra={_FIELDS: fields})

    def debug(self, msg: str, /, **fields: Any) -> None:
        """Write a Debug line.

        Args:
            msg: The fixed code of the line.
            **fields: Attributes; only allowed names are written.
        """
        self._emit(logging.DEBUG, msg, fields)

    def info(self, msg: str, /, **fields: Any) -> None:
        """Write an Info line.

        Args:
            msg: The fixed code of the line.
            **fields: Attributes; only allowed names are written.
        """
        self._emit(logging.INFO, msg, fields)

    def warn(self, msg: str, /, **fields: Any) -> None:
        """Write a Warn line.

        Args:
            msg: The fixed code of the line.
            **fields: Attributes; only allowed names are written.
        """
        self._emit(logging.WARNING, msg, fields)

    def error(self, msg: str, /, **fields: Any) -> None:
        """Write an Error line.

        Args:
            msg: The fixed code of the line.
            **fields: Attributes; only allowed names are written.
        """
        self._emit(logging.ERROR, msg, fields)
