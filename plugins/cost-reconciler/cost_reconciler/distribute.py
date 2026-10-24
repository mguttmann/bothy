# SPDX-License-Identifier: Apache-2.0
"""Distribution of a workspace's daily amount over the estimates of its runs (BR-10).

Rule (10 12.4 point 3, ADR 0055 point 2): runs in another currency are left out and reported as
a currency mismatch; runs with estimate `0` are left out and stay estimates. Each remaining run
gets floor(T * e_i / sum(e)); the remainder goes to the run with the largest estimate, ties to the
smallest run ID, so the shares sum to T exactly. Without a remaining run the whole amount is not
assigned: `no_runs`, or `zero_estimates` when the only runs in the currency have estimate `0`
(the proportion is undefined). Integer arithmetic only.

Example:
    >>> distribute(10, "USD", [RunEstimate("a", "USD", 1), RunEstimate("b", "USD", 2)]).shares
    (('a', 3), ('b', 7))
"""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass

NO_MAPPING = "no_mapping"
"""Reason: the workspace has no entry in `reconciler.workspace_map`."""
NO_RUNS = "no_runs"
"""Reason: no run of the mapped agents in the currency of the amount."""
ZERO_ESTIMATES = "zero_estimates"
"""Reason: the runs in the currency of the amount all have estimate 0."""


@dataclass(frozen=True)
class RunEstimate:
    """The estimate of one run.

    Attributes:
        run_id: The run.
        currency: ISO 4217 code of the estimate.
        estimate_micros: The estimate in micro units, never negative.
    """

    run_id: str
    currency: str
    estimate_micros: int


@dataclass(frozen=True)
class Distribution:
    """The result of one distribution.

    Attributes:
        shares: (run ID, amount in micro units), sorted by run ID; empty when nothing is
            assigned.
        unassigned_reason: None, `no_runs` or `zero_estimates`.
        currency_mismatch: The run IDs in another currency, sorted.
    """

    shares: tuple[tuple[str, int], ...]
    unassigned_reason: str | None
    currency_mismatch: tuple[str, ...]


def distribute(total: int, currency: str, runs: Sequence[RunEstimate]) -> Distribution:
    """Distribute a total over the estimates of runs.

    Args:
        total: The workspace's amount of one day and currency in micro units.
        currency: Its currency.
        runs: The estimates of the runs of the workspace's agents in that day.

    Returns:
        The shares, the reason when nothing is assigned and the runs in another currency.

    Raises:
        ValueError: The total or an estimate is negative.
    """
    if total < 0:
        raise ValueError("the total is negative")
    if any(r.estimate_micros < 0 for r in runs):
        raise ValueError("an estimate is negative")
    mismatch = tuple(sorted(r.run_id for r in runs if r.currency != currency))
    same = [r for r in runs if r.currency == currency]
    eligible = [r for r in same if r.estimate_micros > 0]
    if not eligible:
        reason = ZERO_ESTIMATES if same else NO_RUNS
        return Distribution(shares=(), unassigned_reason=reason, currency_mismatch=mismatch)
    weight = sum(r.estimate_micros for r in eligible)
    shares = {r.run_id: total * r.estimate_micros // weight for r in eligible}
    largest = min(eligible, key=lambda r: (-r.estimate_micros, r.run_id))
    shares[largest.run_id] += total - sum(shares.values())
    return Distribution(
        shares=tuple(sorted(shares.items())), unassigned_reason=None, currency_mismatch=mismatch
    )
