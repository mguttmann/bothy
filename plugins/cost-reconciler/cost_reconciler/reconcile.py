# SPDX-License-Identifier: Apache-2.0
"""One reconciliation pass (docs/spec/10-backend-policy.md 12.4 points 1 to 5).

A pass reads both secrets again, fetches the whole cost report of the window (all pages) and
checks it, sums every bucket per workspace and currency, and only then talks to the server: per
bucket in ascending order it reads the estimates of the mapped agents once, distributes each
workspace's amount (BR-10) and submits all entries of the bucket in one request. Amounts without
a matching run are logged as `not_assigned`. The first failure ends the pass; the next pass
starts again, and the server's no-op rule keeps a repeated submission without a second effect.
"""

from __future__ import annotations

import enum
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from datetime import datetime

from cost_reconciler.amounts import AmountError, workspace_totals
from cost_reconciler.config import Config
from cost_reconciler.costapi import Bucket, CostApiClient, CostApiError, report_window
from cost_reconciler.distribute import NO_MAPPING, RunEstimate, distribute
from cost_reconciler.logs import EventLog
from cost_reconciler.secretref import Secret, SecretRefError, read_secret
from cost_reconciler.server import Entry, Estimate, ServerClient, ServerError, format_time
from cost_reconciler.transport import StopRequestedError

PROVIDER = "anthropic"
"""The provider name the runtimes report in `cost_entries.provider` for this provider."""


class PassResult(enum.Enum):
    """How a pass ended."""

    OK = "ok"
    FAILED = "failed"
    CONFIG_ERROR = "config_error"


@dataclass(frozen=True)
class PassContext:
    """What a pass needs.

    Attributes:
        config: The checked configuration.
        cost_api: Client of the cost report; its pacer keeps the request interval.
        server: Client of the server's cost operations.
        log: The event log; the pass sets the current secrets on its filter.
        now: The wall clock with a time zone.
        stopped: Reports whether a stop was requested.
        environ: The environment for `env:` references.
    """

    config: Config
    cost_api: CostApiClient
    server: ServerClient
    log: EventLog
    now: Callable[[], datetime]
    stopped: Callable[[], bool]
    environ: Mapping[str, str]


class _SecretReadError(Exception):
    def __init__(self, key: str, rule: str) -> None:
        super().__init__(key)
        self.key = key
        self.rule = rule


def _read_secrets(ctx: PassContext) -> tuple[Secret, Secret]:
    values = []
    for key, ref in (
        ("reconciler.token_ref", ctx.config.token_ref),
        ("reconciler.admin_key_ref", ctx.config.admin_key_ref),
    ):
        try:
            values.append(read_secret(ref, ctx.environ))
        except SecretRefError as err:
            raise _SecretReadError(key, err.code) from None
    return values[0], values[1]


def _check_stop(ctx: PassContext) -> None:
    if ctx.stopped():
        raise StopRequestedError


def _bucket_entries(
    ctx: PassContext,
    token: Secret,
    bucket: Bucket,
    totals: Mapping[tuple[str | None, str], int],
) -> list[Entry]:
    """Distribute one bucket and return its entries, logging what stays unassigned."""
    log, config = ctx.log, ctx.config
    start = format_time(bucket.starting_at)
    estimates: list[Estimate] = []
    if any(ws is not None and ws in config.workspace_map for ws, _ in totals):
        _check_stop(ctx)
        estimates = ctx.server.list_estimates(
            token, bucket.starting_at, bucket.ending_at, config.agent_ids
        )
    by_workspace: dict[str, list[RunEstimate]] = {}
    for estimate in estimates:
        workspace = config.workspace_of(estimate.agent_id)
        # A run of another provider cannot carry this provider's invoice amount; the server
        # would report it unmatched, so it never gets a share.
        if workspace is not None and estimate.provider == PROVIDER:
            by_workspace.setdefault(workspace, []).append(
                RunEstimate(estimate.run_id, estimate.currency, estimate.estimate_micros)
            )
    entries: list[Entry] = []
    for (workspace, currency), total in sorted(
        totals.items(), key=lambda t: (t[0][0] or "", t[0][1])
    ):
        fields: dict[str, object] = {
            "bucket_start": start,
            "workspace_id": workspace,
            "currency": currency,
        }
        if workspace is None or workspace not in config.workspace_map:
            log.emit("not_assigned", level="info", amount_micros=total, reason=NO_MAPPING, **fields)
            continue
        result = distribute(total, currency, by_workspace.get(workspace, []))
        if result.currency_mismatch:
            log.emit(
                "currency_mismatch",
                level="warning",
                run_ids=list(result.currency_mismatch),
                **fields,
            )
        if result.unassigned_reason is not None:
            log.emit(
                "not_assigned",
                level="info",
                amount_micros=total,
                reason=result.unassigned_reason,
                **fields,
            )
            continue
        entries.extend(Entry(run_id, share, currency) for run_id, share in result.shares)
    entries.sort(key=lambda e: e.run_id)
    return entries


def _fail(ctx: PassContext, stage: str, error: str, **fields: object) -> PassResult:
    ctx.log.emit("pass_failed", level="error", stage=stage, error=error, **fields)
    return PassResult.FAILED


def run_pass(ctx: PassContext) -> PassResult:
    """Run one pass.

    Args:
        ctx: The context.

    Returns:
        OK when every submission succeeded, FAILED when anything failed or a stop interrupted
        the pass, CONFIG_ERROR when a secret cannot be read (also a file with an open mode).
    """
    window_start, window_end = report_window(ctx.now(), ctx.config.lookback_days)
    ctx.log.emit(
        "pass_started", window_start=format_time(window_start), window_end=format_time(window_end)
    )
    try:
        token, admin_key = _read_secrets(ctx)
    except _SecretReadError as err:
        ctx.log.emit("config_error", level="error", key=err.key, rule=err.rule)
        return PassResult.CONFIG_ERROR
    ctx.log.set_secrets([token.reveal(), admin_key.reveal()])
    stage = "cost_api"
    bucket_start: str | None = None
    try:
        buckets = ctx.cost_api.fetch_report(admin_key, window_start, window_end)
        try:
            # Every bucket is summed before the first submission: never submit from a report
            # that is malformed in a later bucket.
            totals = [workspace_totals(b.lines) for b in buckets]
        except AmountError:
            return _fail(ctx, stage, "malformed")
        submissions = 0
        for bucket, bucket_totals in zip(buckets, totals, strict=True):
            bucket_start = format_time(bucket.starting_at)
            stage = "estimates"
            entries = _bucket_entries(ctx, token, bucket, bucket_totals)
            if not entries:
                continue
            stage = "submit"
            _check_stop(ctx)
            result = ctx.server.submit(
                token, PROVIDER, bucket.starting_at, bucket.ending_at, entries
            )
            submissions += 1
            ctx.log.emit(
                "submitted",
                bucket_start=bucket_start,
                entry_count=len(entries),
                reconciled=result.reconciled,
            )
            if result.unmatched_run_ids:
                ctx.log.emit(
                    "unmatched_runs",
                    level="warning",
                    bucket_start=bucket_start,
                    run_ids=list(result.unmatched_run_ids),
                )
    except StopRequestedError:
        return _fail(ctx, stage, "interrupted")
    except CostApiError as err:
        return _fail(ctx, stage, err.kind, **({"status": err.status} if err.status else {}))
    except ServerError as err:
        fields: dict[str, object] = {"bucket_start": bucket_start}
        if err.status is not None:
            fields["status"] = err.status
        if err.code is not None:
            fields["code"] = err.code
        if err.reason is not None:
            fields["reason"] = err.reason
        if err.errors:
            fields["errors"] = [{"pointer": p, "code": c} for p, c in err.errors]
        return _fail(ctx, err.stage, err.kind, **fields)
    ctx.log.emit("pass_finished", bucket_count=len(buckets), submission_count=submissions)
    return PassResult.OK
