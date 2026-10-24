# SPDX-License-Identifier: Apache-2.0
"""BR-10: distribution of a workspace's daily amount over the estimates of its runs."""

from __future__ import annotations

import random

import pytest

from cost_reconciler.distribute import (
    NO_RUNS,
    ZERO_ESTIMATES,
    Distribution,
    RunEstimate,
    distribute,
)


def run(run_id: str, estimate: int, currency: str = "USD") -> RunEstimate:
    return RunEstimate(run_id=run_id, currency=currency, estimate_micros=estimate)


def test_single_run_takes_everything() -> None:
    assert distribute(500, "USD", [run("r1", 7)]) == Distribution(
        shares=(("r1", 500),), unassigned_reason=None, currency_mismatch=()
    )


def test_proportional_shares_with_remainder_on_largest() -> None:
    result = distribute(10000100, "USD", [run("r1", 2000000), run("r3", 1000000)])
    assert result.shares == (("r1", 6666734), ("r3", 3333366))
    assert sum(s for _, s in result.shares) == 10000100


def test_remainder_tie_goes_to_smallest_run_id() -> None:
    result = distribute(10, "USD", [run("rb", 1), run("ra", 1), run("rc", 1)])
    assert dict(result.shares) == {"ra": 4, "rb": 3, "rc": 3}


def test_shares_are_sorted_by_run_id() -> None:
    result = distribute(9, "USD", [run("rz", 1), run("ra", 2)])
    assert [r for r, _ in result.shares] == ["ra", "rz"]


def test_zero_estimates_and_other_currencies_are_left_out() -> None:
    result = distribute(
        100, "USD", [run("r1", 3), run("r2", 0), run("r3", 1), run("r4", 5, currency="EUR")]
    )
    assert dict(result.shares) == {"r1": 75, "r3": 25}
    assert result.currency_mismatch == ("r4",)
    assert result.unassigned_reason is None


def test_no_run_gives_no_runs() -> None:
    result = distribute(100, "USD", [])
    assert result == Distribution(shares=(), unassigned_reason=NO_RUNS, currency_mismatch=())


def test_only_other_currencies_gives_no_runs() -> None:
    result = distribute(100, "USD", [run("r2", 5, "EUR"), run("r1", 5, "EUR")])
    assert result.unassigned_reason == NO_RUNS
    assert result.currency_mismatch == ("r1", "r2")
    assert result.shares == ()


def test_only_zero_estimates_gives_zero_estimates() -> None:
    result = distribute(100, "USD", [run("r1", 0), run("r2", 0)])
    assert result.unassigned_reason == ZERO_ESTIMATES
    assert result.shares == ()


def test_zero_total_gives_zero_shares() -> None:
    result = distribute(0, "USD", [run("r1", 3), run("r2", 1)])
    assert result.shares == (("r1", 0), ("r2", 0))
    assert result.unassigned_reason is None


def test_large_values_stay_exact() -> None:
    big = 2**63 - 1
    result = distribute(big, "USD", [run("r1", big), run("r2", big - 1)])
    assert sum(s for _, s in result.shares) == big


@pytest.mark.parametrize("total", [-1])
def test_negative_total_is_rejected(total: int) -> None:
    with pytest.raises(ValueError, match="total"):
        distribute(total, "USD", [run("r1", 1)])


def test_negative_estimate_is_rejected() -> None:
    with pytest.raises(ValueError, match="estimate"):
        distribute(1, "USD", [run("r1", -1)])


def test_random_cases_hold_the_rules() -> None:
    rng = random.Random(20261024)  # noqa: S311 - a seeded test generator, no secret
    for _ in range(2000):
        total = rng.choice([0, 1, rng.randrange(1, 10**6), rng.randrange(1, 10**15)])
        runs = [
            run(
                f"r{i:03d}",
                rng.choice([0, rng.randrange(1, 1000), rng.randrange(1, 10**12)]),
                rng.choice(["USD", "USD", "USD", "EUR"]),
            )
            for i in rng.sample(range(1000), rng.randrange(0, 8))
        ]
        result = distribute(total, "USD", runs)
        eligible = [r for r in runs if r.currency == "USD" and r.estimate_micros > 0]
        assert result.currency_mismatch == tuple(
            sorted(r.run_id for r in runs if r.currency != "USD")
        )
        if not eligible:
            assert result.shares == ()
            assert result.unassigned_reason in {NO_RUNS, ZERO_ESTIMATES}
            continue
        shares = dict(result.shares)
        assert set(shares) == {r.run_id for r in eligible}
        assert sum(shares.values()) == total
        weight = sum(r.estimate_micros for r in eligible)
        largest = min(eligible, key=lambda r: (-r.estimate_micros, r.run_id))
        for r in eligible:
            floor_share = total * r.estimate_micros // weight
            if r.run_id == largest.run_id:
                assert shares[r.run_id] == total - sum(
                    total * o.estimate_micros // weight for o in eligible if o is not r
                )
            else:
                assert shares[r.run_id] == floor_share
