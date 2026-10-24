# SPDX-License-Identifier: Apache-2.0
"""Client of the server's two cost operations, through the public REST API only.

- `listCostEstimates` (`GET /api/v1/costs/estimates`, action `cost.reconcile.read`): the
  estimates per run of the given agents in a range, followed to the last page.
- `submitCostReconciliation` (`POST /api/v1/costs/reconciliations`, action
  `cost.reconcile.submit`): the reconciled amounts of one bucket, with a fresh UUIDv7 as
  `Idempotency-Key`.

Both authenticate with `Authorization: Bearer <app token>`; DPoP is not used. No redirect is
followed and no request is retried inside a pass: the next pass submits again and the server's
no-op rule prevents a second effect.
"""

from __future__ import annotations

import json
import re
import urllib.request
import uuid
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any
from urllib.parse import urlencode

from cost_reconciler.secretref import Secret
from cost_reconciler.transport import TransportError, send

API_PREFIX = "/api/v1"
ESTIMATES_PATH = f"{API_PREFIX}/costs/estimates"
RECONCILIATIONS_PATH = f"{API_PREFIX}/costs/reconciliations"
PAGE_LIMIT = 100
_INT64_MAX = 2**63 - 1
_CURRENCY = re.compile(r"[A-Z]{3}")


class ServerError(Exception):
    """A request to the server failed.

    The message names the stage, the kind and the status; problem members are attributes for
    the log and never part of the message.

    Attributes:
        stage: `estimates` or `submit`.
        kind: `status` (an answer outside `2xx`, also a redirect), `network` or `malformed`.
        status: The HTTP status, if any.
        code: The problem `code`, if the answer was a problem document.
        reason: The problem `reason`, if any.
        errors: (pointer, code) of the problem's `errors[]`.
    """

    def __init__(
        self,
        stage: str,
        kind: str,
        status: int | None = None,
        code: str | None = None,
        reason: str | None = None,
        errors: tuple[tuple[str, str], ...] = (),
    ) -> None:
        """Create the error.

        Args:
            stage: The stage.
            kind: The kind.
            status: The HTTP status.
            code: The problem code.
            reason: The problem reason.
            errors: The field errors.
        """
        super().__init__(f"server {stage}: {kind}" + (f" {status}" if status is not None else ""))
        self.stage = stage
        self.kind = kind
        self.status = status
        self.code = code
        self.reason = reason
        self.errors = errors


@dataclass(frozen=True)
class Estimate:
    """One item of `listCostEstimates` (schema `CostEstimate`)."""

    run_id: str
    agent_id: str
    provider: str
    currency: str
    estimate_micros: int
    reconciled: bool


@dataclass(frozen=True)
class Entry:
    """One entry of a submission (schema `ReconcileEntry`)."""

    run_id: str
    amount_micros: int
    currency: str


@dataclass(frozen=True)
class SubmitResult:
    """The answer of a submission (schema `CostReconcileResult`)."""

    reconciled: int
    unmatched_run_ids: tuple[str, ...]


def format_time(value: datetime) -> str:
    """Format a time as the API's timestamp: RFC 3339 UTC with milliseconds and `Z`."""
    value = value.astimezone(UTC)
    return value.strftime("%Y-%m-%dT%H:%M:%S.") + f"{value.microsecond // 1000:03d}Z"


def _str(value: object) -> str | None:
    return value if isinstance(value, str) else None


def _problem(stage: str, status: int, body: bytes) -> ServerError:
    try:
        doc: Any = json.loads(body)
    except (ValueError, UnicodeDecodeError, RecursionError):
        doc = None
    if not isinstance(doc, dict):
        return ServerError(stage, "status", status)
    errors: list[tuple[str, str]] = []
    raw = doc.get("errors")
    for item in raw if isinstance(raw, list) else []:
        if isinstance(item, dict):
            pointer, code = _str(item.get("pointer")), _str(item.get("code"))
            if pointer is not None and code is not None:
                errors.append((pointer, code))
    return ServerError(
        stage, "status", status, _str(doc.get("code")), _str(doc.get("reason")), tuple(errors)
    )


def _int(value: object) -> int | None:
    # bool is a subclass of int in Python, but true is no amount.
    return value if isinstance(value, int) and not isinstance(value, bool) else None


def _estimate(value: object, agents: set[str]) -> Estimate:
    if not isinstance(value, dict):
        raise ServerError("estimates", "malformed")
    run_id, agent_id = value.get("run_id"), value.get("agent_id")
    provider, currency = value.get("provider"), value.get("currency")
    micros, reconciled = _int(value.get("estimate_micros")), value.get("reconciled")
    if (
        not isinstance(run_id, str)
        or not isinstance(agent_id, str)
        or agent_id not in agents
        or not isinstance(provider, str)
        or not isinstance(currency, str)
        or not _CURRENCY.fullmatch(currency)
        or micros is None
        or not 0 <= micros <= _INT64_MAX
        or not isinstance(reconciled, bool)
    ):
        raise ServerError("estimates", "malformed")
    return Estimate(run_id, agent_id, provider, currency, micros, reconciled)


class ServerClient:
    """Calls `listCostEstimates` and `submitCostReconciliation`."""

    def __init__(
        self,
        base_url: str,
        timeout_s: float,
        opener: urllib.request.OpenerDirector,
        new_key: Callable[[], uuid.UUID],
    ) -> None:
        """Create the client.

        Args:
            base_url: `reconciler.server_url` without a trailing slash.
            timeout_s: `reconciler.http_timeout` in seconds.
            opener: The opener of `transport.build_opener`.
            new_key: Source of `Idempotency-Key` values (UUIDv7).
        """
        self._base_url = base_url
        self._timeout_s = timeout_s
        self._opener = opener
        self._new_key = new_key

    def _send(self, stage: str, request: urllib.request.Request) -> bytes:
        try:
            answer = send(self._opener, request, self._timeout_s, read_error_body=True)
        except TransportError:
            raise ServerError(stage, "network") from None
        if not 200 <= answer.status < 300:
            raise _problem(stage, answer.status, answer.body)
        return answer.body

    def list_estimates(
        self, token: Secret, start: datetime, end: datetime, agent_ids: Sequence[str]
    ) -> list[Estimate]:
        """Read the estimates of the runs of agents in a range, all pages.

        Args:
            token: The app token.
            start: Start of the range (inclusive).
            end: End of the range (exclusive).
            agent_ids: The agents; sent sorted.

        Returns:
            The estimates in the server's order (by run ID).

        Raises:
            ServerError: A request failed, or an answer is malformed (also an item of an agent
                that was not asked for, or a run ID twice).
        """
        agents = sorted(set(agent_ids))
        base = [("from", format_time(start)), ("to", format_time(end))]
        base += [("agent_id", a) for a in agents]
        base.append(("limit", str(PAGE_LIMIT)))
        result: list[Estimate] = []
        seen: set[str] = set()
        cursor: str | None = None
        while True:
            query = base if cursor is None else [*base, ("cursor", cursor)]
            request = urllib.request.Request(  # noqa: S310 - the scheme is checked in config
                f"{self._base_url}{ESTIMATES_PATH}?{urlencode(query)}",
                method="GET",
                headers={"Authorization": f"Bearer {token.reveal()}", "Accept": "application/json"},
            )
            try:
                page: Any = json.loads(self._send("estimates", request))
            except (ValueError, UnicodeDecodeError, RecursionError):
                raise ServerError("estimates", "malformed") from None
            if not isinstance(page, dict) or not isinstance(page.get("items"), list):
                raise ServerError("estimates", "malformed")
            for item in page["items"]:
                estimate = _estimate(item, set(agents))
                if estimate.run_id in seen:
                    raise ServerError("estimates", "malformed")
                seen.add(estimate.run_id)
                result.append(estimate)
            if "next_cursor" not in page:
                return result
            cursor = page["next_cursor"]
            if not isinstance(cursor, str) or not cursor:
                raise ServerError("estimates", "malformed")

    def submit(
        self,
        token: Secret,
        provider: str,
        start: datetime,
        end: datetime,
        entries: Sequence[Entry],
    ) -> SubmitResult:
        """Submit the reconciled amounts of one bucket.

        Args:
            token: The app token.
            provider: The provider name the runtimes report in `cost_entries.provider`.
            start: `period_start`, the bucket start.
            end: `period_end`, the bucket end.
            entries: The entries.

        Returns:
            The number of reconciled entries and the unmatched run IDs.

        Raises:
            ServerError: The request failed or the answer is malformed.
        """
        body = {
            "provider": provider,
            "period_start": format_time(start),
            "period_end": format_time(end),
            "entries": [
                {"run_id": e.run_id, "amount_micros": e.amount_micros, "currency": e.currency}
                for e in entries
            ],
        }
        request = urllib.request.Request(  # noqa: S310 - the scheme is checked in config
            f"{self._base_url}{RECONCILIATIONS_PATH}",
            data=json.dumps(body).encode("utf-8"),
            method="POST",
            headers={
                "Authorization": f"Bearer {token.reveal()}",
                "Idempotency-Key": str(self._new_key()),
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
        )
        try:
            answer: Any = json.loads(self._send("submit", request))
        except (ValueError, UnicodeDecodeError, RecursionError):
            raise ServerError("submit", "malformed") from None
        if not isinstance(answer, dict):
            raise ServerError("submit", "malformed")
        reconciled, unmatched = _int(answer.get("reconciled")), answer.get("unmatched_run_ids")
        if (
            reconciled is None
            or reconciled < 0
            or not isinstance(unmatched, list)
            or not all(isinstance(u, str) for u in unmatched)
        ):
            raise ServerError("submit", "malformed")
        return SubmitResult(reconciled, tuple(unmatched))
