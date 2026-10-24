# SPDX-License-Identifier: Apache-2.0
"""Exact cent sums and their one conversion to micro units."""

from __future__ import annotations

from decimal import Decimal

import pytest

from cost_reconciler.amounts import AmountError, cents_to_micros, parse_cents, workspace_totals
from cost_reconciler.costapi import CostLine


def test_parse_cents_is_exact() -> None:
    assert parse_cents("600.005") == Decimal("600.005")
    assert parse_cents("0") == Decimal(0)
    assert parse_cents("0.1") + parse_cents("0.2") == Decimal("0.3")


@pytest.mark.parametrize(
    "text",
    ["", " 1", "1 ", "-1", "+1", "1e3", "1E3", "NaN", "Infinity", "1.", ".5", "1,5", "0x10", "1_0"],
)
def test_parse_cents_rejects_invalid_strings(text: str) -> None:
    with pytest.raises(AmountError):
        parse_cents(text)


@pytest.mark.parametrize("value", [None, 1, 1.5, ["1"]])
def test_parse_cents_rejects_non_strings(value: object) -> None:
    with pytest.raises(AmountError):
        parse_cents(value)


@pytest.mark.parametrize(
    ("cents", "micros"),
    [
        ("1000.010", 10000100),
        ("250.5", 2505000),
        ("50", 500000),
        ("0.00005", 0),  # 0.5 micro units: half to even rounds down to 0
        ("0.00015", 2),  # 1.5 micro units: half to even rounds up to 2
        ("0.00025", 2),  # 2.5 micro units: half to even rounds down to 2
        ("0.000251", 3),
        ("0.00004999", 0),
    ],
)
def test_cents_to_micros_rounds_half_to_even(cents: str, micros: int) -> None:
    assert cents_to_micros(Decimal(cents)) == micros


def test_cents_to_micros_rejects_values_beyond_int64() -> None:
    with pytest.raises(AmountError):
        cents_to_micros(Decimal(2**63) / 10000 + 1)


def test_long_digit_strings_stay_exact() -> None:
    text = "123456789012.000000000000000000000000000000005"
    assert parse_cents(text) == Decimal(text)
    assert cents_to_micros(parse_cents(text) * 2) == 2469135780240000


def test_workspace_totals_sum_before_rounding() -> None:
    lines = [
        CostLine(workspace_id="w1", currency="USD", amount="0.00005"),
        CostLine(workspace_id="w1", currency="USD", amount="0.00005"),
        CostLine(workspace_id="w2", currency="USD", amount="0.00005"),
        CostLine(workspace_id=None, currency="USD", amount="1"),
        CostLine(workspace_id="w1", currency="EUR", amount="2"),
    ]
    assert workspace_totals(lines) == {
        ("w1", "USD"): 1,
        ("w2", "USD"): 0,
        (None, "USD"): 10000,
        ("w1", "EUR"): 20000,
    }


def test_workspace_totals_reject_an_invalid_amount() -> None:
    with pytest.raises(AmountError):
        workspace_totals([CostLine(workspace_id="w1", currency="USD", amount="abc")])
