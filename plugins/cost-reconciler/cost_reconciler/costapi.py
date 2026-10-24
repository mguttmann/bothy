# SPDX-License-Identifier: Apache-2.0
"""Client of the provider's cost report (docs/spec/10-backend-policy.md 12.4 point 2).

`GET <reconciler.cost_api_url>/v1/organizations/cost_report` with daily buckets grouped by
`workspace_id` and `description`, authenticated with the admin key in the header `x-api-key`
and the header `anthropic-version`, as the provider documentation cited in 10 12.4 prescribes.
The client follows `has_more` and `next_page` to the last page and checks every page before it
returns anything, so a pass never submits from a partial report. An answer outside `2xx` is an
error that carries only the status; the body is never read.
"""

from __future__ import annotations

import json
import re
import urllib.request
from dataclasses import dataclass
from datetime import UTC, datetime, time, timedelta
from typing import Any
from urllib.parse import urlencode

from cost_reconciler.amounts import AmountError, parse_cents
from cost_reconciler.secretref import Secret
from cost_reconciler.transport import Pacer, TransportError, send

REPORT_PATH = "/v1/organizations/cost_report"
API_VERSION = "2023-06-01"
_CURRENCY = re.compile(r"[A-Z]{3}")


class CostApiError(Exception):
    """The cost report could not be fetched or is malformed.

    Attributes:
        kind: `status` (an answer outside `2xx`, also a redirect), `network` (no answer) or
            `malformed` (a page without the documented fields).
        status: The HTTP status for `status`, else None.
    """

    def __init__(self, kind: str, status: int | None = None, rule: str = "") -> None:
        """Create the error.

        Args:
            kind: The kind.
            status: The HTTP status, if any.
            rule: A fixed text naming the missing or invalid field.
        """
        text = f"cost API: {kind}"
        if status is not None:
            text += f" {status}"
        if rule:
            text += f" ({rule})"
        super().__init__(text)
        self.kind = kind
        self.status = status


@dataclass(frozen=True)
class CostLine:
    """One cost line of a bucket.

    Attributes:
        workspace_id: The workspace; None for the default workspace.
        currency: ISO 4217 code.
        amount: Decimal string in cents, checked by `amounts.parse_cents`.
    """

    workspace_id: str | None
    currency: str
    amount: str


@dataclass(frozen=True)
class Bucket:
    """One daily bucket.

    Attributes:
        starting_at: Start (inclusive), UTC.
        ending_at: End (exclusive), UTC.
        lines: The cost lines in the order of the report.
    """

    starting_at: datetime
    ending_at: datetime
    lines: tuple[CostLine, ...]


def report_window(now: datetime, lookback_days: int) -> tuple[datetime, datetime]:
    """Return the window of complete UTC days before today.

    Args:
        now: The current time with a time zone.
        lookback_days: `reconciler.lookback` in days.

    Returns:
        Start of today (UTC) minus the lookback, and start of today (UTC).

    Raises:
        ValueError: `now` has no time zone.

    Example:
        >>> report_window(datetime(2026, 10, 21, 5, tzinfo=UTC), 3)[0].isoformat()
        '2026-10-18T00:00:00+00:00'
    """
    if now.tzinfo is None:
        raise ValueError("now needs a time zone")
    today = datetime.combine(now.astimezone(UTC).date(), time(), tzinfo=UTC)
    return today - timedelta(days=lookback_days), today


def format_time(value: datetime) -> str:
    """Format a UTC time in the RFC 3339 form of the cost API, for example `...T00:00:00Z`."""
    return value.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")


def _parse_time(value: object, rule: str) -> datetime:
    if not isinstance(value, str):
        raise CostApiError("malformed", rule=rule)
    try:
        parsed = datetime.fromisoformat(value)
        if parsed.tzinfo is None:
            raise CostApiError("malformed", rule=rule)
        return parsed.astimezone(UTC)
    except (ValueError, OverflowError):
        raise CostApiError("malformed", rule=rule) from None


def _line(value: object) -> CostLine:
    if not isinstance(value, dict):
        raise CostApiError("malformed", rule="a result is no object")
    if "amount" not in value or "currency" not in value or "workspace_id" not in value:
        raise CostApiError("malformed", rule="a result lacks amount, currency or workspace_id")
    amount, currency, workspace = value["amount"], value["currency"], value["workspace_id"]
    if not isinstance(currency, str) or not _CURRENCY.fullmatch(currency):
        raise CostApiError("malformed", rule="currency is no ISO 4217 code")
    if workspace is not None and (not isinstance(workspace, str) or not workspace):
        raise CostApiError("malformed", rule="workspace_id is no string")
    if not isinstance(amount, str):
        raise CostApiError("malformed", rule="amount is no string")
    try:
        parse_cents(amount)
    except AmountError:
        raise CostApiError("malformed", rule="amount is no decimal string") from None
    return CostLine(workspace_id=workspace, currency=currency, amount=amount)


def _bucket(value: object) -> Bucket:
    if not isinstance(value, dict):
        raise CostApiError("malformed", rule="a bucket is no object")
    start = _parse_time(value.get("starting_at"), "starting_at is no RFC 3339 time")
    end = _parse_time(value.get("ending_at"), "ending_at is no RFC 3339 time")
    if end <= start:
        raise CostApiError("malformed", rule="ending_at is not after starting_at")
    results = value.get("results")
    if not isinstance(results, list):
        raise CostApiError("malformed", rule="results is no list")
    return Bucket(start, end, tuple(_line(r) for r in results))


def _page(body: bytes) -> tuple[list[Bucket], str | None]:
    try:
        page: Any = json.loads(body)
    except (ValueError, UnicodeDecodeError, RecursionError):
        raise CostApiError("malformed", rule="the page is no JSON") from None
    if not isinstance(page, dict):
        raise CostApiError("malformed", rule="the page is no object")
    data, has_more = page.get("data"), page.get("has_more")
    if not isinstance(data, list):
        raise CostApiError("malformed", rule="data is no list")
    if not isinstance(has_more, bool):
        raise CostApiError("malformed", rule="has_more is no boolean")
    next_page = page.get("next_page")
    if has_more and (not isinstance(next_page, str) or not next_page):
        raise CostApiError("malformed", rule="has_more without next_page")
    return [_bucket(b) for b in data], next_page if has_more else None


class CostApiClient:
    """Fetches the cost report of a window, all pages, through one pacer."""

    def __init__(
        self,
        base_url: str,
        timeout_s: float,
        pacer: Pacer,
        opener: urllib.request.OpenerDirector,
    ) -> None:
        """Create the client.

        Args:
            base_url: `reconciler.cost_api_url` without a trailing slash.
            timeout_s: `reconciler.http_timeout` in seconds.
            pacer: Keeps `reconciler.min_request_interval` between requests.
            opener: The opener of `transport.build_opener`.
        """
        self._base_url = base_url
        self._timeout_s = timeout_s
        self._pacer = pacer
        self._opener = opener

    def _request(self, admin_key: Secret, query: list[tuple[str, str]]) -> bytes:
        url = f"{self._base_url}{REPORT_PATH}?{urlencode(query)}"
        request = urllib.request.Request(  # noqa: S310 - the scheme is checked in config
            url,
            method="GET",
            headers={
                "x-api-key": admin_key.reveal(),
                "anthropic-version": API_VERSION,
                "Accept": "application/json",
            },
        )
        self._pacer.wait()
        try:
            answer = send(self._opener, request, self._timeout_s, read_error_body=False)
        except TransportError:
            raise CostApiError("network") from None
        if not 200 <= answer.status < 300:
            raise CostApiError("status", status=answer.status)
        return answer.body

    def fetch_report(self, admin_key: Secret, start: datetime, end: datetime) -> list[Bucket]:
        """Fetch every page of the report of a window.

        Buckets with the same bounds on several pages are joined; the result is sorted by
        `starting_at` ascending.

        Args:
            admin_key: The provider's admin key.
            start: Start of the window (inclusive).
            end: End of the window (exclusive).

        Returns:
            The buckets.

        Raises:
            CostApiError: A request failed or a page is malformed.
            StopRequestedError: A stop was requested before a request.
        """
        base = [
            ("starting_at", format_time(start)),
            ("ending_at", format_time(end)),
            ("bucket_width", "1d"),
            ("group_by[]", "workspace_id"),
            ("group_by[]", "description"),
        ]
        joined: dict[tuple[datetime, datetime], list[CostLine]] = {}
        page_token: str | None = None
        while True:
            query = base if page_token is None else [*base, ("page", page_token)]
            buckets, page_token = _page(self._request(admin_key, query))
            for bucket in buckets:
                joined.setdefault((bucket.starting_at, bucket.ending_at), []).extend(bucket.lines)
            if page_token is None:
                break
        return [Bucket(s, e, tuple(lines)) for (s, e), lines in sorted(joined.items())]
