# SPDX-License-Identifier: Apache-2.0
"""Cost of a run as the difference of the cumulative session values (09 18.7, 4.10, note 14).

``result.modelUsage`` counts per model for the whole session, also across ``--resume``; the
runner stores the last values per session in ``sessions/<ref>/usage.json`` and reports the
difference. A negative difference of any value means the runtime started counting again, so
the current values count as this run's. Before ``finishRun`` the new values and the reports
are stored together as a pending record, so a retried ``finishRun`` of the same run sends the
same reports and a delta is reported at most once. Cache writes count as 5 minutes (10 12.3).

A session bound with a cost baseline (an import or a rebind, ``AgentSession.cost_baseline_micros``,
22 9 step 7, RK-75) whose values the runner has never stored keeps a remaining baseline: the
amounts of the next runs are reduced by it until it is used up, and a reset of the runtime's
counters clears it. Tokens have no baseline (09 18.7).

A resumed session restores its totals from the last ``cost-state`` line of its transcript
(``<config dir>/projects/<folder>/<ref>.jsonl``, pinned CLI 2.1.288). Before the first process
of a run that resumes, the runner reads the same line (`transcript_totals`) and makes it the stored
values (`UsageStore.rebase`), so the run books only what its own turns added, in amounts and in
tokens, also for a session started outside the runner or continued outside it since its last run
(RUNNER-RESUME-COST).

Examples:
    >>> to_micros(Decimal("0.0000025"))
    2
"""

from __future__ import annotations

import json
from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from decimal import ROUND_HALF_EVEN, Decimal, InvalidOperation
from pathlib import Path
from typing import Any

from bothy_sdk.generated.models import CostReport

from claude_code_runner.statedir import ensure_private_dir, write_private

__all__ = [
    "ModelUsage",
    "SessionUsage",
    "UsageStore",
    "claude_config_dir",
    "context_windows",
    "cost_reports",
    "counters_reset",
    "difference",
    "find_transcript",
    "metrics_totals",
    "parse_model_usage",
    "subtract_baseline",
    "to_micros",
    "transcript_totals",
]

_FIELDS = ("inputTokens", "outputTokens", "cacheReadInputTokens", "cacheCreationInputTokens")
_MICROS = Decimal(1_000_000)
_COST_STATE = b'"cost-state"'


@dataclass(frozen=True)
class ModelUsage:
    """Cumulative or difference values of one model.

    Attributes:
        input_tokens: ``inputTokens``.
        output_tokens: ``outputTokens``.
        cache_read: ``cacheReadInputTokens``.
        cache_creation: ``cacheCreationInputTokens``.
        cost_usd: ``costUSD`` as a decimal of its text.
    """

    input_tokens: int
    output_tokens: int
    cache_read: int
    cache_creation: int
    cost_usd: Decimal

    def to_json(self) -> dict[str, Any]:
        """Return the stored form (the cost as decimal text)."""
        return {
            "inputTokens": self.input_tokens,
            "outputTokens": self.output_tokens,
            "cacheReadInputTokens": self.cache_read,
            "cacheCreationInputTokens": self.cache_creation,
            "costUSD": str(self.cost_usd),
        }


def _int(value: Any) -> int:
    return value if isinstance(value, int) and not isinstance(value, bool) and value >= 0 else 0


def _decimal(value: Any) -> Decimal:
    if isinstance(value, bool) or not isinstance(value, int | float | str):
        return Decimal(0)
    try:
        # repr of a float is its shortest decimal text, never the binary fraction.
        number = Decimal(repr(value)) if isinstance(value, float) else Decimal(value)
    except InvalidOperation:
        return Decimal(0)
    return number if number.is_finite() and number >= 0 else Decimal(0)


def _from_json(raw: Mapping[str, Any]) -> ModelUsage:
    ints = [_int(raw.get(f)) for f in _FIELDS]
    return ModelUsage(ints[0], ints[1], ints[2], ints[3], _decimal(raw.get("costUSD")))


def parse_model_usage(result: Mapping[str, Any]) -> dict[str, ModelUsage]:
    """Read ``modelUsage`` of a ``result`` message.

    Args:
        result: The ``result`` message.

    Returns:
        The values per model (empty without ``modelUsage``).
    """
    usage = result.get("modelUsage")
    if not isinstance(usage, Mapping):
        return {}
    return {str(m): _from_json(v) for m, v in usage.items() if isinstance(v, Mapping)}


def claude_config_dir(env: Mapping[str, str]) -> Path | None:
    """Return the CLI's configuration folder for a turn environment.

    Args:
        env: The environment of the turn (``CLAUDE_CONFIG_DIR`` of ``organization``, else
            ``HOME``).

    Returns:
        ``CLAUDE_CONFIG_DIR``, else ``$HOME/.claude``; ``None`` without either.
    """
    if env.get("CLAUDE_CONFIG_DIR"):
        return Path(env["CLAUDE_CONFIG_DIR"])
    if env.get("HOME"):
        return Path(env["HOME"]) / ".claude"
    return None


def find_transcript(config_dir: Path, ref: str) -> Path | None:
    """Return the transcript of session ``ref`` under ``<config_dir>/projects``.

    The CLI names the folder after the working directory; any folder counts, so a session moved
    or started in another folder is found too. With several, the newest file wins.

    Args:
        config_dir: The CLI's configuration folder.
        ref: The session reference.

    Returns:
        The path, or ``None`` when there is none (or ``ref`` is no plain file name).
    """
    if not ref or ref != Path(ref).name or ref.startswith("."):
        return None
    try:
        folders = list((config_dir / "projects").iterdir())
        found = [f / f"{ref}.jsonl" for f in folders if (f / f"{ref}.jsonl").is_file()]
    except OSError:
        return None
    newest: tuple[int, Path] | None = None
    for path in found:
        try:
            mtime = path.stat().st_mtime_ns
        except OSError:  # removed after the listing
            continue
        if newest is None or mtime > newest[0]:
            newest = (mtime, path)
    return newest[1] if newest is not None else None


def transcript_totals(path: Path) -> dict[str, ModelUsage] | None:
    """Read the session totals a resume restores: the last valid ``cost-state`` line.

    Args:
        path: The session transcript (JSON lines).

    Returns:
        Its ``modelUsage`` per model, or ``None`` without a readable ``cost-state`` line.
    """
    totals: dict[str, ModelUsage] | None = None
    try:
        with path.open("rb") as fh:
            for raw in fh:
                if _COST_STATE not in raw:
                    continue
                try:
                    line = json.loads(raw)
                except ValueError:
                    continue
                if not isinstance(line, dict) or line.get("type") != "cost-state":
                    continue
                if isinstance(line.get("modelUsage"), Mapping):
                    totals = parse_model_usage(line)
    except OSError:
        return None
    return totals


def context_windows(result: Mapping[str, Any]) -> dict[str, int]:
    """Read the ``contextWindow`` per model of a ``result`` message.

    Args:
        result: The ``result`` message.

    Returns:
        The windows of the models that state a positive one.
    """
    usage = result.get("modelUsage")
    if not isinstance(usage, Mapping):
        return {}
    out: dict[str, int] = {}
    for model, values in usage.items():
        window = values.get("contextWindow") if isinstance(values, Mapping) else None
        if isinstance(window, int) and not isinstance(window, bool) and window > 0:
            out[str(model)] = window
    return out


def difference(
    previous: Mapping[str, ModelUsage], current: Mapping[str, ModelUsage]
) -> dict[str, ModelUsage]:
    """Return this run's values per model.

    Args:
        previous: The stored values of the session (empty for a new session).
        current: The cumulative values of this run's ``result``.

    Returns:
        The differences; for a model with any negative difference the current values.
    """
    out: dict[str, ModelUsage] = {}
    for model, now in current.items():
        before = previous.get(model)
        if before is None:
            out[model] = now
            continue
        diff = ModelUsage(
            now.input_tokens - before.input_tokens,
            now.output_tokens - before.output_tokens,
            now.cache_read - before.cache_read,
            now.cache_creation - before.cache_creation,
            now.cost_usd - before.cost_usd,
        )
        values = (diff.input_tokens, diff.output_tokens, diff.cache_read, diff.cache_creation)
        out[model] = now if min(values) < 0 or diff.cost_usd < 0 else diff
    return out


def counters_reset(previous: Mapping[str, ModelUsage], current: Mapping[str, ModelUsage]) -> bool:
    """Report whether the runtime started counting again (the negative-difference rule).

    Args:
        previous: The stored values of the session.
        current: The cumulative values of this run's ``result``.

    Returns:
        ``True`` when any value of a model known before went down.
    """
    for model, now in current.items():
        before = previous.get(model)
        if before is None:
            continue
        pairs = (
            (now.input_tokens, before.input_tokens),
            (now.output_tokens, before.output_tokens),
            (now.cache_read, before.cache_read),
            (now.cache_creation, before.cache_creation),
        )
        if any(n < b for n, b in pairs) or now.cost_usd < before.cost_usd:
            return True
    return False


def subtract_baseline(
    reports: Sequence[CostReport], remaining: int
) -> tuple[list[CostReport], int]:
    """Subtract the remaining baseline from the amounts of one run (09 18.7, RK-75).

    The part ``min(remaining, sum of the amounts)`` is shared across the models in proportion to
    their amounts: each model first gives the floor of its share, then the leftover micros come
    one by one from the models in model order that still have an amount left, so no amount turns
    negative and the result never depends on the order of the input. Tokens stay as they are.

    Args:
        reports: The reports of the run (one per model).
        remaining: The remaining baseline in micros; 0 or less changes nothing.

    Returns:
        The reports with reduced amounts and the baseline that remains after them.

    Examples:
        >>> r = [CostReport(provider="anthropic", model="m", currency="USD", amount_micros=30)]
        >>> out, rest = subtract_baseline(r, 50)
        >>> (out[0].amount_micros, rest)
        (0, 20)
    """
    amounts = [_n(r.amount_micros) for r in reports]
    total = sum(amounts)
    take = min(max(remaining, 0), total)
    if take == 0:
        return list(reports), max(remaining, 0)
    shares = [take * a // total for a in amounts]
    left = take - sum(shares)
    for i in sorted(range(len(reports)), key=lambda i: reports[i].model):
        if left == 0:
            break
        if amounts[i] - shares[i] > 0:
            shares[i] += 1
            left -= 1
    out = []
    for report, amount, share in zip(reports, amounts, shares, strict=True):
        reduced = CostReport.from_dict(report.to_dict())
        reduced.amount_micros = amount - share
        out.append(reduced)
    return out, remaining - take


def _n(value: Any) -> int:
    return value if isinstance(value, int) and not isinstance(value, bool) else 0


def to_micros(cost_usd: Decimal) -> int:
    """Convert US dollars into millionths with half-even rounding.

    Args:
        cost_usd: The amount.

    Returns:
        The amount in micros.
    """
    return int((cost_usd * _MICROS).quantize(Decimal(1), rounding=ROUND_HALF_EVEN))


def cost_reports(diff: Mapping[str, ModelUsage]) -> list[CostReport]:
    """Return ``RunFinish.costs`` for this run's values (the server labels them estimates).

    Args:
        diff: This run's values per model.

    Returns:
        One report per model, in model order.
    """
    return [
        CostReport(
            provider="anthropic",
            model=model,
            currency="USD",
            tokens_in=u.input_tokens,
            tokens_out=u.output_tokens,
            tokens_cache_read=u.cache_read,
            tokens_cache_write=u.cache_creation,
            amount_micros=to_micros(u.cost_usd),
        )
        for model, u in sorted(diff.items())
    ]


def metrics_totals(diff: Mapping[str, ModelUsage]) -> dict[str, int]:
    """Return the token sums of this run for ``RunFinish.metrics``.

    Args:
        diff: This run's values per model.

    Returns:
        ``tokens_in``, ``tokens_out``, ``tokens_cache_read`` and ``tokens_cache_write``.
    """
    return {
        "tokens_in": sum(u.input_tokens for u in diff.values()),
        "tokens_out": sum(u.output_tokens for u in diff.values()),
        "tokens_cache_read": sum(u.cache_read for u in diff.values()),
        "tokens_cache_write": sum(u.cache_creation for u in diff.values()),
    }


def _mapping(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


@dataclass(frozen=True)
class SessionUsage:
    """The stored state of a session.

    Attributes:
        values: The last cumulative values per model.
        context_windows: The last known ``contextWindow`` per model.
        pending_run: The run whose reports are pending, if any.
        pending: Those reports in their ``to_dict`` form.
        baseline_remaining: The part of the session's cost baseline not yet subtracted, in
            micros; ``None`` when the runner never took a baseline for the session.
    """

    values: dict[str, ModelUsage] = field(default_factory=dict)
    context_windows: dict[str, int] = field(default_factory=dict)
    pending_run: str | None = None
    pending: list[dict[str, Any]] = field(default_factory=list)
    baseline_remaining: int | None = None


class UsageStore:
    """``usage.json`` of one session directory (mode ``0600``, written atomically).

    Args:
        session_dir: ``<runner.state_dir>/sessions/<ref>``.
    """

    def __init__(self, session_dir: Path) -> None:
        """Create the store; nothing is read before `load`."""
        self.path = session_dir / "usage.json"

    def load(self) -> SessionUsage:
        """Read the stored state; a missing or unreadable file is an empty state.

        Returns:
            The state.
        """
        try:
            raw = json.loads(self.path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            return SessionUsage()
        if not isinstance(raw, dict):
            return SessionUsage()
        values = _mapping(raw.get("values"))
        windows = _mapping(raw.get("context_windows"))
        pending = _mapping(raw.get("pending"))
        baseline = raw.get("baseline_remaining_micros")
        return SessionUsage(
            values={m: _from_json(v) for m, v in values.items() if isinstance(v, dict)},
            context_windows={m: w for m, w in windows.items() if isinstance(w, int)},
            pending_run=pending.get("run_id") if isinstance(pending.get("run_id"), str) else None,
            pending=[r for r in pending.get("reports", []) if isinstance(r, dict)],
            baseline_remaining=baseline
            if isinstance(baseline, int) and not isinstance(baseline, bool) and baseline >= 0
            else None,
        )

    def record(
        self,
        run_id: str,
        current: Mapping[str, ModelUsage],
        reports: Sequence[CostReport],
        windows: Mapping[str, int],
        baseline_remaining: int | None = None,
    ) -> None:
        """Store the new cumulative values with the pending reports of ``run_id``.

        The remaining baseline is written in the same atomic record, so a retried ``finishRun``
        and the next run see the same state.

        Args:
            run_id: The run whose ``finishRun`` follows.
            current: The cumulative values of its ``result``.
            reports: The reports it sends.
            windows: The ``contextWindow`` per model.
            baseline_remaining: The baseline left after this run; ``None`` for a session without
                one.
        """
        state = self.load()
        merged = dict(state.values)
        merged.update(current)
        known = dict(state.context_windows)
        known.update(windows)
        doc = {
            "values": {m: u.to_json() for m, u in sorted(merged.items())},
            "context_windows": dict(sorted(known.items())),
            "pending": {"run_id": run_id, "reports": [r.to_dict() for r in reports]},
        }
        if baseline_remaining is not None:
            doc["baseline_remaining_micros"] = baseline_remaining
        ensure_private_dir(self.path.parent)
        write_private(self.path, json.dumps(doc, sort_keys=True), atomic=True)

    def rebase(self, totals: Mapping[str, ModelUsage]) -> None:
        """Make ``totals`` the stored values: the session's sums before a resumed turn.

        The remaining baseline ends (the old amounts are in ``totals``); the context windows and
        a pending record stay.

        Args:
            totals: The values per model the runtime restores (`transcript_totals`).
        """
        state = self.load()
        doc: dict[str, Any] = {
            "values": {m: u.to_json() for m, u in sorted(totals.items())},
            "context_windows": dict(sorted(state.context_windows.items())),
            "baseline_remaining_micros": 0,
        }
        if state.pending_run is not None:
            doc["pending"] = {"run_id": state.pending_run, "reports": state.pending}
        ensure_private_dir(self.path.parent)
        write_private(self.path, json.dumps(doc, sort_keys=True), atomic=True)

    def start_without_baseline(self) -> None:
        """Mark a session the runner created itself: no cost baseline ever applies to it.

        The server keeps a bound row's ``cost_baseline_micros`` when a new reference replaces
        the old one, so the mark is written before the new reference can be reported.
        """
        if self.path.exists():
            return
        doc = {"values": {}, "context_windows": {}, "baseline_remaining_micros": 0}
        ensure_private_dir(self.path.parent)
        write_private(self.path, json.dumps(doc, sort_keys=True), atomic=True)

    def pending_for(self, run_id: str) -> list[CostReport] | None:
        """Return the stored reports of ``run_id``, if they are the pending ones.

        Args:
            run_id: The run.

        Returns:
            The reports, or ``None`` when another run (or none) is pending.
        """
        state = self.load()
        if state.pending_run != run_id:
            return None
        return [CostReport.from_dict(r) for r in state.pending]
