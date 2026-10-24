# SPDX-License-Identifier: Apache-2.0
"""JSON lines on stderr with a closed list of events and fields.

Every line has `ts` (RFC 3339 UTC with milliseconds), `level`, `event` and only fields of the
fixed list `FIELDS`: bucket start, workspace ID, currency, amounts in micro units, counts, run
IDs, status, problem code and the like. A header, a token, a key or a body has no field. As a
second line of defence a logging filter replaces the current secret values in every record with
`secretref.REDACTED`.

Example:
    >>> import io
    >>> log = EventLog(io.StringIO())
    >>> log.emit("pass_started")
"""

from __future__ import annotations

import json
import logging
from collections.abc import Callable, Iterable
from datetime import UTC, datetime
from typing import TextIO

from cost_reconciler.secretref import REDACTED

EVENTS = frozenset(
    {
        "pass_started",
        "pass_finished",
        "pass_failed",
        "not_assigned",
        "currency_mismatch",
        "submitted",
        "unmatched_runs",
        "config_error",
    }
)
"""The events of the app."""

FIELDS = frozenset(
    {
        "window_start",
        "window_end",
        "bucket_start",
        "workspace_id",
        "currency",
        "amount_micros",
        "entry_count",
        "reconciled",
        "run_ids",
        "bucket_count",
        "submission_count",
        "stage",
        "error",
        "status",
        "code",
        "reason",
        "errors",
        "key",
        "rule",
    }
)
"""The fields a line may hold besides `ts`, `level` and `event`."""

LEVELS = {"info": logging.INFO, "warning": logging.WARNING, "error": logging.ERROR}


def _redact(value: object, secrets: tuple[str, ...]) -> object:
    if isinstance(value, str):
        for secret in secrets:
            value = value.replace(secret, REDACTED)
        return value
    if isinstance(value, list | tuple):
        return [_redact(v, secrets) for v in value]
    if isinstance(value, dict):
        return {_redact(k, secrets): _redact(v, secrets) for k, v in value.items()}
    return value


class RedactingFilter(logging.Filter):
    """Replaces the current secret values in the message and the fields of every record."""

    def __init__(self) -> None:
        """Create the filter without secrets."""
        super().__init__()
        self.secrets: tuple[str, ...] = ()

    def filter(self, record: logging.LogRecord) -> bool:
        """Redact the record in place and keep it.

        Args:
            record: The record; its `fields` attribute holds the line's fields.

        Returns:
            Always True.
        """
        if self.secrets:
            record.msg = _redact(record.msg, self.secrets)
            fields = getattr(record, "fields", None)
            if isinstance(fields, dict):
                record.fields = _redact(fields, self.secrets)
        return True


class _JsonFormatter(logging.Formatter):
    def __init__(self, now: Callable[[], datetime]) -> None:
        super().__init__()
        self._now = now

    def format(self, record: logging.LogRecord) -> str:
        ts = self._now().astimezone(UTC)
        line: dict[str, object] = {
            "ts": ts.strftime("%Y-%m-%dT%H:%M:%S.") + f"{ts.microsecond // 1000:03d}Z",
            "level": record.levelname.lower(),
            "event": record.getMessage(),
        }
        line.update(getattr(record, "fields", {}))
        return json.dumps(line, ensure_ascii=False, separators=(",", ":"))


def _utc_now() -> datetime:
    return datetime.now(UTC)


class EventLog:
    """Writes the app's events as JSON lines to one stream."""

    def __init__(self, stream: TextIO, now: Callable[[], datetime] = _utc_now) -> None:
        """Create the log.

        Args:
            stream: The target, stderr in production.
            now: The wall clock for `ts`.
        """
        # An unregistered logger: no propagation to the root logger, no global state.
        self._logger = logging.Logger("cost_reconciler", logging.INFO)
        self._filter = RedactingFilter()
        handler = logging.StreamHandler(stream)
        handler.setFormatter(_JsonFormatter(now))
        handler.addFilter(self._filter)
        self._logger.addHandler(handler)

    def set_secrets(self, values: Iterable[str]) -> None:
        """Set the secret values the filter replaces; empty values are ignored.

        Args:
            values: The current values of the token and the admin key.
        """
        # Longest first, so a secret that contains another is replaced as a whole.
        self._filter.secrets = tuple(sorted({v for v in values if v}, key=len, reverse=True))

    def emit(self, event: str, level: str = "info", **fields: object) -> None:
        """Write one line.

        Args:
            event: One of `EVENTS`.
            level: `info`, `warning` or `error`.
            **fields: Fields of `FIELDS`.

        Raises:
            ValueError: An unknown event, level or field (a programming error).
        """
        if event not in EVENTS:
            raise ValueError(f"unknown event {event!r}")
        if level not in LEVELS:
            raise ValueError(f"unknown level {level!r}")
        unknown = set(fields) - FIELDS
        if unknown:
            raise ValueError(f"unknown field {sorted(unknown)!r}")
        self._logger.log(LEVELS[level], event, extra={"fields": fields})
