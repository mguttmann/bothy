# SPDX-License-Identifier: Apache-2.0
"""Exact sums of cent strings and their one conversion to micro units.

The cost API reports amounts as decimal strings in cents, the lowest unit of the currency
(10 12.4 point 2). Per bucket, workspace and currency the strings are summed exactly with
`decimal.Decimal`, then converted once: one cent is ten thousand micro units, rounded half to
even to an integer. Summing before rounding keeps the total exact.

Example:
    >>> cents_to_micros(parse_cents("600.005") + parse_cents("400.005"))
    10000100
"""

from __future__ import annotations

import re
from collections.abc import Iterable
from decimal import ROUND_HALF_EVEN, Context, Decimal, InvalidOperation
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from cost_reconciler.costapi import CostLine

MICROS_PER_CENT = 10_000
_INT64_MAX = 2**63 - 1
# Digits, optionally a dot and more digits: no sign, no exponent, no NaN or infinity, no
# separators. A negative cost (a credit) is outside the documented shape and fails the pass.
_DECIMAL = re.compile(r"[0-9]+(\.[0-9]+)?")
# Enough precision that no sum of report strings is ever rounded before the one conversion.
_EXACT = Context(prec=200, traps=[InvalidOperation])


class AmountError(ValueError):
    """An amount that is no decimal cent string or exceeds the range of int64 micro units."""


def parse_cents(text: object) -> Decimal:
    """Parse a decimal cent string exactly.

    Args:
        text: The `amount` member of a cost line.

    Returns:
        The value in cents.

    Raises:
        AmountError: The value is no string of digits with an optional fraction, or too long
            to stay exact.
    """
    if not isinstance(text, str) or not _DECIMAL.fullmatch(text):
        raise AmountError("the amount is no decimal string")
    if len(text) > _EXACT.prec // 2:
        raise AmountError("the amount has too many digits")
    return Decimal(text)


def cents_to_micros(cents: Decimal) -> int:
    """Convert cents to micro units, rounding half to even once.

    Args:
        cents: A non-negative amount in cents.

    Returns:
        The amount in micro units.

    Raises:
        AmountError: The result exceeds the range of int64 (the server's `amount_micros`).
    """
    micros = _EXACT.multiply(cents, Decimal(MICROS_PER_CENT)).quantize(
        Decimal(1), rounding=ROUND_HALF_EVEN, context=_EXACT
    )
    if micros > _INT64_MAX:
        raise AmountError("the amount exceeds the range of micro units")
    return int(micros)


def workspace_totals(lines: Iterable[CostLine]) -> dict[tuple[str | None, str], int]:
    """Sum the lines of one bucket per workspace and currency, in micro units.

    Args:
        lines: The cost lines of one bucket.

    Returns:
        (workspace ID or None, currency) to the total in micro units, in the order the pairs
        first appear.

    Raises:
        AmountError: An amount is invalid or a total exceeds the range of int64.
    """
    sums: dict[tuple[str | None, str], Decimal] = {}
    for line in lines:
        key = (line.workspace_id, line.currency)
        sums[key] = _EXACT.add(sums.get(key, Decimal(0)), parse_cents(line.amount))
    return {key: cents_to_micros(total) for key, total in sums.items()}
